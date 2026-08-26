import { DsnErr, type Access, type Kind } from "./types";

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

const SELECT_TAILS: string[][] = [
  ["INTO", "OUTFILE"],
  ["INTO", "DUMPFILE"],
  ["FOR", "UPDATE"],
  ["FOR", "SHARE"],
  ["LOCK", "IN", "SHARE", "MODE"],
];

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
  if (kind === "mysql" || kind === "doris") {
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
  gateEs(stmt);
}

function reject(message: string): never {
  throw new DsnErr(message, 2);
}

function gateSql(kind: Kind, stmt: string): void {
  const words = sqlWords(stmt);
  const head = words[0];
  if (head === undefined) {
    reject("empty statement");
  }
  const allow = kind === "doris" ? DORIS_READ : SQL_READ;
  if (!allow.has(head)) {
    reject(`blocked head: ${head}`);
  }
  if (sqlHasTail(stmt)) {
    reject("multi-statement is blocked");
  }
  if (head === "SELECT" || head === "WITH") {
    const tail = matchSelectTail(words);
    if (tail !== undefined) {
      reject(`blocked ${tail}`);
    }
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

type SqlTok =
  | { kind: "word"; value: string }
  | { kind: "string" }
  | { kind: "semi" };

/** 为什么: 只要语句头和分号, 不做 vendor parser, 字符串里的 ; 不能当第二句. */
export function sqlWords(stmt: string): string[] {
  return tokenizeSql(stmt)
    .filter((tok) => tok.kind === "word")
    .map((tok) => tok.value);
}

function sqlHasTail(stmt: string): boolean {
  const toks = tokenizeSql(stmt);
  const semi = toks.findIndex((tok) => tok.kind === "semi");
  if (semi < 0) {
    return false;
  }
  return toks.slice(semi + 1).some((tok) => tok.kind !== "semi");
}

function matchSelectTail(words: string[]): string | undefined {
  for (const seq of SELECT_TAILS) {
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

function tokenizeSql(input: string): SqlTok[] {
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
    if (ch === "#") {
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
    if (ch === "'" || ch === '"' || ch === "`") {
      index = skipSqlString(input, index, ch);
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

function skipSqlString(input: string, start: number, quote: string): number {
  let index = start + 1;
  while (index < input.length) {
    const ch = input[index] ?? "";
    if (ch === "\\" && quote !== "`") {
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

/** 为什么: Redis 命令行和官方客户端一样按空白+引号切开, 头永远是第一个 token. */
export function splitArgs(stmt: string): string[] {
  const args: string[] = [];
  let index = 0;
  while (index < stmt.length) {
    while (index < stmt.length && /\s/.test(stmt[index] ?? "")) {
      index += 1;
    }
    if (index >= stmt.length) {
      break;
    }
    const ch = stmt[index] ?? "";
    if (ch === "'" || ch === '"') {
      const start = index + 1;
      index += 1;
      while (index < stmt.length && stmt[index] !== ch) {
        if (stmt[index] === "\\") {
          index += 2;
          continue;
        }
        index += 1;
      }
      args.push(stmt.slice(start, index));
      index += 1;
      continue;
    }
    const start = index;
    while (index < stmt.length && !/\s/.test(stmt[index] ?? "")) {
      index += 1;
    }
    args.push(stmt.slice(start, index));
  }
  return args;
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
