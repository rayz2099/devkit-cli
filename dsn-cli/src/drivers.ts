import { Client as EsClient, HttpConnection } from "@elastic/elasticsearch";
import { Redis } from "ioredis";
import { MongoClient } from "mongodb";
import mysql from "mysql2/promise";
import { parseRestQuery, splitArgs } from "./gate";
import { queryKafka } from "./kafka";
import { DsnErr, type Kind, type QueryOut, type Timeouts } from "./types";

/** 为什么: doctor 只探连通, 必须用各 Kind 最小只读语句, 不能让用户语句或 Gate 介入. */
export function probeStmt(kind: Kind): string {
  switch (kind) {
    case "mysql":
    case "doris":
      return "SELECT 1";
    case "redis":
      return "PING";
    case "mongodb":
      return '{"ping":1}';
    case "elasticsearch":
      return "GET /";
    case "kafka":
      return "ping";
    default: {
      const _never: never = kind;
      return _never;
    }
  }
}

/** 为什么: Query 走 TS driver, 不 spawn 官方客户端, 才能在出站前过 Gate. */
export async function runDriver(
  kind: Kind,
  url: string,
  stmt: string,
  timeouts: Timeouts,
): Promise<QueryOut> {
  try {
    if (kind === "mysql" || kind === "doris") {
      return await queryMysql(url, stmt, timeouts);
    }
    if (kind === "redis") {
      return await queryRedis(url, stmt, timeouts);
    }
    if (kind === "mongodb") {
      return await queryMongo(url, stmt, timeouts);
    }
    if (kind === "elasticsearch") {
      return await queryEs(url, stmt, timeouts);
    }
    return await queryKafka(url, stmt, timeouts);
  } catch (error) {
    if (error instanceof DsnErr) {
      throw error;
    }
    const message = error instanceof Error ? error.message : String(error);
    throw new DsnErr(message, 3);
  }
}

async function queryMysql(url: string, stmt: string, timeouts: Timeouts): Promise<QueryOut> {
  const conn = await mysql.createConnection({
    uri: url,
    connectTimeout: timeouts.connectMs,
  });
  try {
    const [res, fields] = await conn.query({ sql: stmt, timeout: timeouts.execMs });
    return mysqlOut(res, fields);
  } finally {
    await conn.end();
  }
}

function mysqlOut(res: unknown, fields: unknown): QueryOut {
  if (!Array.isArray(res)) {
    const header = res as { affectedRows?: number; insertId?: number };
    return {
      columns: ["affectedRows", "insertId"],
      rows: [
        {
          affectedRows: header.affectedRows ?? 0,
          insertId: header.insertId ?? 0,
        },
      ],
    };
  }
  if (res.length > 0 && Array.isArray(res[0])) {
    throw new DsnErr("multiple result sets", 3);
  }
  const cols = mysqlCols(fields, res as object[]);
  const rows = (res as object[]).map((row) => asRow(row));
  return { columns: cols, rows };
}

function mysqlCols(fields: unknown, rows: object[]): string[] {
  if (Array.isArray(fields) && fields.length > 0) {
    return fields.map((field) => {
      const rec = field as { name?: string };
      return rec.name ?? "";
    });
  }
  const first = rows[0];
  return first === undefined ? [] : Object.keys(first);
}

async function queryRedis(url: string, stmt: string, timeouts: Timeouts): Promise<QueryOut> {
  const args = splitArgs(stmt);
  const cmd = args[0];
  if (cmd === undefined) {
    throw new DsnErr("empty statement", 2);
  }
  const redis = new Redis(url, {
    lazyConnect: true,
    maxRetriesPerRequest: 0,
    enableOfflineQueue: false,
    connectTimeout: timeouts.connectMs,
    commandTimeout: timeouts.execMs,
    retryStrategy() {
      // 为什么: 探测失败必须停, 不能重连把超时拖过 deadline.
      return null;
    },
  });
  // 为什么: ioredis 在 socket 失败时 emit error, 无 listener 会打 Unhandled error event 污染 doctor.
  redis.on("error", () => {});
  try {
    await redis.connect();
    const rest = args.slice(1);
    const value = await redis.call(cmd, ...rest);
    return valueOut(value);
  } finally {
    redis.disconnect();
  }
}

async function queryMongo(url: string, stmt: string, timeouts: Timeouts): Promise<QueryOut> {
  const doc = JSON.parse(stmt) as Record<string, unknown>;
  const client = new MongoClient(url, {
    serverSelectionTimeoutMS: timeouts.connectMs,
    connectTimeoutMS: timeouts.connectMs,
    socketTimeoutMS: timeouts.execMs,
  });
  try {
    await client.connect();
    const dbName = mongoDb(url);
    const db = dbName === "" ? client.db() : client.db(dbName);
    const out = await db.command(doc, { timeoutMS: timeouts.execMs });
    return mongoOut(out as Record<string, unknown>);
  } finally {
    await client.close();
  }
}

function mongoDb(url: string): string {
  const parsed = new URL(url);
  return decodeURIComponent(parsed.pathname.replace(/^\//, ""));
}

function mongoOut(out: Record<string, unknown>): QueryOut {
  const cursor = out.cursor;
  if (cursor && typeof cursor === "object" && !Array.isArray(cursor)) {
    const batch = (cursor as { firstBatch?: unknown }).firstBatch;
    if (Array.isArray(batch)) {
      return docsOut(batch);
    }
  }
  return docsOut([out]);
}

/** 为什么: Bun 的 undici.Pool 没有 close, 必须用 HttpConnection, 否则成功请求会被 close 打成失败. */
async function queryEs(url: string, stmt: string, timeouts: Timeouts): Promise<QueryOut> {
  const parsed = parseRestQuery(stmt);
  const client = new EsClient({
    node: url,
    Connection: HttpConnection,
    pingTimeout: timeouts.connectMs,
    requestTimeout: timeouts.execMs,
    maxRetries: 0,
  });
  try {
    const res = await client.transport.request({
      method: parsed.method,
      path: parsed.path,
      body: parsed.body as Record<string, unknown> | undefined,
    });
    return esOut(res);
  } finally {
    await client.close();
  }
}

function esOut(res: unknown): QueryOut {
  if (res && typeof res === "object") {
    const hits = (res as { hits?: { hits?: unknown } }).hits?.hits;
    if (Array.isArray(hits)) {
      return docsOut(hits);
    }
  }
  return docsOut([res]);
}

function docsOut(docs: unknown[]): QueryOut {
  const rows = docs.map((doc) => asRow(doc));
  const cols = rows[0] === undefined ? [] : Object.keys(rows[0]);
  return { columns: cols, rows };
}

function valueOut(value: unknown): QueryOut {
  if (Array.isArray(value)) {
    const rows = value.map((item) => ({ value: item }));
    return { columns: ["value"], rows };
  }
  return { columns: ["value"], rows: [{ value }] };
}

function asRow(value: unknown): Record<string, unknown> {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return { ...(value as Record<string, unknown>) };
  }
  return { value };
}
