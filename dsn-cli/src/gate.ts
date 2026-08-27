import { parseKafkaStmt } from "./kafka-stmt";
import { parsePgCatalog, pgCatalogHead } from "./pg-stmt";
import { splitArgs } from "./split";
import { DsnErr, type Access, type Kind } from "./types";

export { splitArgs };

const SQL_READ = new Set([
  "SELECT",
  "WITH",
  "SHOW",
  "EXPLAIN",
  "DESC",
  "DESCRIBE",
  "USE",
]);
const DORIS_READ = new Set([...SQL_READ, "SWITCH"]);
const PG_READ = new Set([
  "SELECT",
  "WITH",
  "TABLE",
  "VALUES",
  "TABLES",
  "COLUMNS",
  "DDL",
  "DESC",
  "DESCRIBE",
  "SHOW",
  "EXPLAIN",
]);

const SELECT_TAILS: string[][] = [
  ["INTO", "OUTFILE"],
  ["INTO", "DUMPFILE"],
  ["FOR", "UPDATE"],
  ["FOR", "SHARE"],
  ["LOCK", "IN", "SHARE", "MODE"],
];

const PG_LOCK_TAILS: string[][] = [
  ["FOR", "UPDATE"],
  ["FOR", "SHARE"],
  ["FOR", "NO", "KEY", "UPDATE"],
  ["FOR", "KEY", "SHARE"],
];

const PG_INNER_HEADS = new Set([
  "SELECT",
  "WITH",
  "TABLE",
  "VALUES",
  "INSERT",
  "UPDATE",
  "DELETE",
  "MERGE",
  "CREATE",
  "DROP",
  "ALTER",
  "TRUNCATE",
  "REFRESH",
  "CALL",
  "DO",
  "COPY",
  "DECLARE",
  "PREPARE",
  "EXECUTE",
  "VACUUM",
]);

type SqlDialect = "mysql" | "postgres";

const REDIS_READ = new Set([
  "GET",
  "MGET",
  "HGET",
  "HGETALL",
  "LRANGE",
  "SMEMBERS",
  "ZRANGE",
  "KEYS",
  "SCAN",
  "TYPE",
  "TTL",
  "EXISTS",
  "INFO",
  "PING",
  "LLEN",
  "SCARD",
  "ZCARD",
  "HLEN",
  "STRLEN",
  "DBSIZE",
]);

const MONGO_READ = new Set([
  "find",
  "aggregate",
  "count",
  "distinct",
  "listCollections",
  "listIndexes",
  "listDatabases",
  "collStats",
  "dbStats",
  "explain",
  "ping",
  "hello",
  "getMore",
  "countDocuments",
  "estimatedDocumentCount",
]);

/** 为什么: write 原样转发; read 必须在 Driver 出站前按 Kind 闭集拦截. */
export function gateQuery(kind: Kind, access: Access, stmt: string): void {
  if (access === "write") {
    return;
  }
  if (kind === "mysql" || kind === "doris" || kind === "postgres") {
    gateSql(kind, stmt);
    return;
  }
  if (kind === "redis") {
    gateRedis(stmt);
    return;
  }
  if (kind === "mongodb") {
    gateMongo(stmt);
    return;
  }
  if (kind === "elasticsearch") {
    gateEs(stmt);
    return;
  }
  if (kind === "kafka") {
    gateKafka(stmt);
    return;
  }
  const _never: never = kind;
  void _never;
}

function reject(message: string): never {
  throw new DsnErr(message, 2);
}

function sqlDialect(kind: "mysql" | "doris" | "postgres"): SqlDialect {
  return kind === "postgres" ? "postgres" : "mysql";
}

function sqlAllow(kind: "mysql" | "doris" | "postgres"): Set<string> {
  if (kind === "doris") {
    return DORIS_READ;
  }
  if (kind === "postgres") {
    return PG_READ;
  }
  return SQL_READ;
}

function gateSql(kind: "mysql" | "doris" | "postgres", stmt: string): void {
  const dialect = sqlDialect(kind);
  const words = sqlWords(stmt, dialect);
  const head = words[0];
  if (head === undefined) {
    reject("empty statement");
  }
  const allow = sqlAllow(kind);
  if (!allow.has(head)) {
    reject(`blocked head: ${head}`);
  }
  if (sqlHasTail(stmt, dialect)) {
    reject("multi-statement is blocked");
  }
  if (kind === "postgres") {
    if (pgCatalogHead(stmt) !== undefined) {
      parsePgCatalog(stmt);
      return;
    }
    gatePgTails(head, words);
    return;
  }
  if (head === "SELECT" || head === "WITH") {
    const tail = matchSelectTail(words, SELECT_TAILS);
    if (tail !== undefined) {
      reject(`blocked ${tail}`);
    }
  }
}

/** 为什么: postgres 的写形尾巴和 mysql 不同, 不能共用 SELECT_TAILS. */
function gatePgTails(head: string, words: string[]): void {
  if (head === "SHOW" && words[1] === "TABLES") {
    reject("blocked SHOW TABLES; use TABLES or TABLES LIKE <pattern>");
  }
  if (head === "SHOW" && words[1] === "CREATE") {
    reject("blocked SHOW CREATE; use DDL <table>");
  }
  if (head === "SHOW" && words[1] === "COLUMNS") {
    reject("blocked SHOW COLUMNS; use COLUMNS <table>");
  }
  if (head === "SELECT" || head === "WITH" || head === "TABLE" || head === "VALUES") {
    const lock = matchSelectTail(words, PG_LOCK_TAILS);
    if (lock !== undefined) {
      reject(`blocked ${lock}`);
    }
  }
  if ((head === "SELECT" || head === "WITH") && containsSeq(words, ["INTO"])) {
    reject("blocked INTO");
  }
  if (head === "EXPLAIN") {
    gateExplainAnalyze(words);
  }
}

/** 为什么: EXPLAIN ANALYZE 会执行内层语句, 只拦选项位的 ANALYZE, 不拦 FROM analyze. */
function gateExplainAnalyze(words: string[]): void {
  const analyzeAt = words.indexOf("ANALYZE");
  if (analyzeAt < 0) {
    return;
  }
  const innerAt = words.findIndex((word, index) => index > 0 && PG_INNER_HEADS.has(word));
  if (innerAt < 0 || analyzeAt < innerAt) {
    reject("blocked ANALYZE");
  }
}

function gateRedis(stmt: string): void {
  const args = splitArgs(stmt);
  const head = args[0]?.toUpperCase();
  if (head === undefined) {
    reject("empty statement");
  }
  if (!REDIS_READ.has(head)) {
    reject(`blocked head: ${head}`);
  }
}

function gateMongo(stmt: string): void {
  let doc: unknown;
  try {
    doc = JSON.parse(stmt);
  } catch {
    reject("invalid mongo command JSON");
  }
  if (!doc || typeof doc !== "object" || Array.isArray(doc)) {
    reject("mongo command must be a JSON object");
  }
  const key = Object.keys(doc as object)[0];
  if (key === undefined) {
    reject("empty mongo command");
  }
  if (!MONGO_READ.has(key)) {
    reject(`blocked head: ${key}`);
  }
}

function gateEs(stmt: string): void {
  const parsed = parseRestQuery(stmt);
  if (parsed.method !== "GET" && parsed.method !== "HEAD") {
    reject(`blocked head: ${parsed.method}`);
  }
}

function gateKafka(stmt: string): void {
  parseKafkaStmt(stmt);
}

type SqlTok =
  | { kind: "word"; value: string }
  | { kind: "string" }
  | { kind: "semi" };

/** 为什么: 只要语句头和分号, 不做 vendor parser, 字符串里的 ; 不能当第二句. */
function sqlWords(stmt: string, dialect: SqlDialect): string[] {
  return tokenizeSql(stmt, dialect)
    .filter((tok) => tok.kind === "word")
    .map((tok) => tok.value);
}

function sqlHasTail(stmt: string, dialect: SqlDialect): boolean {
  const toks = tokenizeSql(stmt, dialect);
  const semi = toks.findIndex((tok) => tok.kind === "semi");
  if (semi < 0) {
    return false;
  }
  return toks.slice(semi + 1).some((tok) => tok.kind !== "semi");
}

function matchSelectTail(words: string[], tails: string[][]): string | undefined {
  for (const seq of tails) {
    if (containsSeq(words, seq)) {
      return seq.join(" ");
    }
  }
  return undefined;
}

function containsSeq(words: string[], seq: string[]): boolean {
  for (let index = 0; index <= words.length - seq.length; index += 1) {
    let ok = true;
    for (let offset = 0; offset < seq.length; offset += 1) {
      if (words[index + offset] !== seq[offset]) {
        ok = false;
        break;
      }
    }
    if (ok) {
      return true;
    }
  }
  return false;
}

function tokenizeSql(input: string, dialect: SqlDialect): SqlTok[] {
  const toks: SqlTok[] = [];
  let index = 0;
  while (index < input.length) {
    const ch = input[index] ?? "";
    const next = input[index + 1] ?? "";
    if (/\s/.test(ch)) {
      index += 1;
      continue;
    }
    if (ch === "-" && next === "-") {
      index += 2;
      while (index < input.length && input[index] !== "\n") {
        index += 1;
      }
      continue;
    }
    if (dialect === "mysql" && ch === "#") {
      index += 1;
      while (index < input.length && input[index] !== "\n") {
        index += 1;
      }
      continue;
    }
    if (ch === "/" && next === "*") {
      index += 2;
      while (index < input.length && !(input[index] === "*" && input[index + 1] === "/")) {
        index += 1;
      }
      index += 2;
      continue;
    }
    if (dialect === "postgres" && ch === "$") {
      const end = skipDollarString(input, index);
      if (end !== undefined) {
        toks.push({ kind: "string" });
        index = end;
        continue;
      }
    }
    if (ch === "'" || ch === '"' || ch === "`") {
      index = skipSqlString(input, index, ch, dialect);
      toks.push({ kind: "string" });
      continue;
    }
    if (ch === ";") {
      toks.push({ kind: "semi" });
      index += 1;
      continue;
    }
    if (/[A-Za-z_]/.test(ch)) {
      const start = index;
      index += 1;
      while (index < input.length && /[A-Za-z0-9_]/.test(input[index] ?? "")) {
        index += 1;
      }
      toks.push({ kind: "word", value: input.slice(start, index).toUpperCase() });
      continue;
    }
    index += 1;
  }
  return toks;
}

/** 为什么: $tag$ 里的分号不是第二句; $1 占位符不能当 dollar-quote. */
function skipDollarString(input: string, start: number): number | undefined {
  let index = start + 1;
  if (index < input.length && /[A-Za-z_]/.test(input[index] ?? "")) {
    index += 1;
    while (index < input.length && /[A-Za-z0-9_]/.test(input[index] ?? "")) {
      index += 1;
    }
  }
  if (input[index] !== "$") {
    return undefined;
  }
  const open = input.slice(start, index + 1);
  const from = index + 1;
  const closeAt = input.indexOf(open, from);
  if (closeAt < 0) {
    return input.length;
  }
  return closeAt + open.length;
}

function skipSqlString(input: string, start: number, quote: string, dialect: SqlDialect): number {
  let index = start + 1;
  while (index < input.length) {
    const ch = input[index] ?? "";
    if (dialect === "mysql" && ch === "\\" && quote !== "`") {
      index += 2;
      continue;
    }
    if (ch === quote) {
      if (input[index + 1] === quote) {
        index += 2;
        continue;
      }
      return index + 1;
    }
    index += 1;
  }
  return input.length;
}

export function parseRestQuery(stmt: string): { method: string; path: string; body?: unknown } {
  const trimmed = stmt.trim();
  const match = /^([A-Za-z]+)\s+(\S+)(?:\s+([\s\S]+))?$/.exec(trimmed);
  if (match === null) {
    throw new DsnErr("elasticsearch query must be <METHOD> /path", 2);
  }
  const method = (match[1] ?? "GET").toUpperCase();
  let path = match[2] ?? "/";
  if (!path.startsWith("/")) {
    path = `/${path}`;
  }
  const raw = match[3]?.trim();
  if (raw === undefined || raw === "") {
    return { method, path };
  }
  try {
    return { method, path, body: JSON.parse(raw) };
  } catch {
    throw new DsnErr("invalid elasticsearch JSON body", 2);
  }
}
