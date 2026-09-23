import { chmodSync, mkdirSync, statSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { RcloneErr } from "./types";
import type { Crypt, FileCfg, Source, Target, Task } from "./types";

const TOP_KEYS = ["rcloneBin", "sources", "targets", "tasks"];
const SOURCE_KEYS = ["name", "path"];
const TARGET_KEYS = [
  "name",
  "url",
  "vendor",
  "user",
  "pass",
  "root",
  "skipLinks",
  "skipHidden",
  "sizeOnly",
  "transfers",
  "checkers",
  "retries",
  "retriesSleep",
  "lowLevelRetries",
  "timeout",
  "contimeout",
  "tpslimit",
  "pacerMinSleep",
  "encryptFilenames",
  "crypt",
];
const TASK_KEYS = [
  "name",
  "source",
  "target",
  "excludes",
  "transfers",
  "tpslimit",
  "timeout",
  "encryptFilenames",
  "skipHidden",
];
const CRYPT_PAIR_KEYS = ["name", "key", "salt", "algorithm", "default"];

/** 为什么: 配置和空白 rclone 配置都放在 XDG, 不能写进仓库, 也不能去碰用户的 rclone.conf. */
export function cfgFile(home: string): string {
  return join(needHome(home), ".config", "rclone-cli", "config.json");
}

/**
 * 为什么: rclone 只要 --config 指向的文件里有 remote 就会加载.
 * 每次都重写成空文件, 账号只允许出现在连接串里.
 */
export function ensureEmptyConf(home: string): string {
  const dir = join(needHome(home), ".config", "rclone-cli");
  mkdirSync(dir, { recursive: true });
  const file = join(dir, "empty.conf");
  writeFileSync(file, "");
  chmodSync(file, 0o600);
  return file;
}

/** 为什么: 运行记录不属于配置, 放进 XDG data, 避免和口令文件缠在一起. */
export function runsFile(home: string): string {
  return join(needHome(home), ".local", "share", "rclone-cli", "runs", "runs.jsonl");
}

/** 为什么: 手写配置允许注释和尾逗号, 和 dsn-cli 同一套 JSONC, 不能逼用户先改成严格 JSON. */
export function parseFileCfg(content: string, path: string, home: string): FileCfg {
  const raw = parseObj(stripJsonc(content), path);
  rejectUnknown(raw, TOP_KEYS, path);
  const sources = readSources(raw.sources, path, home);
  const targets = readTargets(raw.targets, path);
  const tasks = readTasks(raw.tasks, path, sources, targets);
  return {
    rcloneBin: readBin(raw, path),
    sources,
    targets,
    tasks,
  };
}

/** 为什么: 缺配置必须失败. 空配置会让后续命令看起来成功, 实际上没有目标. */
export async function loadFileCfg(path: string, home: string): Promise<FileCfg> {
  const file = Bun.file(path);
  if (!(await file.exists())) {
    throw new RcloneErr(1, `config file not found: ${path}`);
  }
  const cfg = parseFileCfg(await file.text(), path, home);
  assertSecretMode(path, cfg);
  return cfg;
}

export function taskNames(cfg: FileCfg): string[] {
  return cfg.tasks.map((task) => task.name);
}

function needHome(home: string): string {
  if (home === "") {
    throw new RcloneErr(1, "HOME is required");
  }
  return home;
}

/**
 * 为什么: 口令写在目标上. 文件不是 600 时, 同机其他用户能读到 WebDAV 和 crypt 密码.
 * 示例配置没有口令, 允许保持安装时的权限, 填入口令后再强制 600.
 */
function assertSecretMode(path: string, cfg: FileCfg): void {
  const secret = cfg.targets.some((target) => hasSecret(target));
  if (!secret) {
    return;
  }
  const mode = statSync(path).mode & 0o777;
  if (mode !== 0o600) {
    throw new RcloneErr(1, `config file mode must be 600: ${path}`);
  }
}

function hasSecret(target: Target): boolean {
  if (target.user !== "" || target.pass !== "") {
    return true;
  }
  return target.crypt !== undefined;
}

function readBin(raw: Record<string, unknown>, path: string): string {
  if (!("rcloneBin" in raw)) {
    return "rclone";
  }
  const value = raw.rcloneBin;
  if (typeof value !== "string" || value.trim() === "") {
    throw new RcloneErr(1, `${path} rcloneBin must be a non-empty string`);
  }
  return value;
}

function readSources(value: unknown, path: string, home: string): Source[] {
  const items = needArr(value, `${path} sources`);
  const sources = items.map((item, index) => readSource(item, `${path} sources[${index}]`, home));
  assertUnique(sources.map((item) => item.name), `${path} sources`);
  return sources;
}

function readSource(value: unknown, at: string, home: string): Source {
  const raw = needObj(value, at);
  rejectUnknown(raw, SOURCE_KEYS, at);
  return {
    name: needStr(raw, "name", at),
    path: needAbs(raw, at, home),
  };
}

/**
 * 为什么: 配置不能写死本机用户名. 只展开 $HOME / ${HOME} 前缀, 其它环境变量一律不碰.
 */
function needAbs(raw: Record<string, unknown>, at: string, home: string): string {
  const path = expandHome(needStr(raw, "path", at), home);
  if (!path.startsWith("/")) {
    throw new RcloneErr(1, `${at}.path must be absolute`);
  }
  return path;
}

function expandHome(raw: string, home: string): string {
  const base = needHome(home);
  if (raw === "$HOME" || raw === "${HOME}") {
    return base;
  }
  if (raw.startsWith("$HOME/")) {
    return `${base}${raw.slice("$HOME".length)}`;
  }
  if (raw.startsWith("${HOME}/")) {
    return `${base}${raw.slice("${HOME}".length)}`;
  }
  return raw;
}

function readTargets(value: unknown, path: string): Target[] {
  const items = needArr(value, `${path} targets`);
  const targets = items.map((item, index) => readTarget(item, `${path} targets[${index}]`));
  assertUnique(targets.map((item) => item.name), `${path} targets`);
  return targets;
}

function readTarget(value: unknown, at: string): Target {
  const raw = needObj(value, at);
  rejectUnknown(raw, TARGET_KEYS, at);
  const crypt = readCrypt(raw.crypt, `${at}.crypt`);
  const name = needStr(raw, "name", at);
  const url = needStr(raw, "url", at);
  const vendor = optStr(raw, "vendor", at, "other");
  const user = needText(raw, "user", at);
  const pass = needText(raw, "pass", at);
  const root = needStr(raw, "root", at);
  const skipLinks = optBool(raw, "skipLinks", at, true);
  const skipHidden = optBool(raw, "skipHidden", at, true);
  const sizeOnly = optBool(raw, "sizeOnly", at, true);
  const transfers = optNum(raw, "transfers", at, 1);
  const checkers = optNum(raw, "checkers", at, 4);
  const retries = optNum(raw, "retries", at, 10);
  const retriesSleep = optStr(raw, "retriesSleep", at, "15s");
  const lowLevelRetries = optNum(raw, "lowLevelRetries", at, 20);
  const timeout = optStr(raw, "timeout", at, "1h");
  const contimeout = optStr(raw, "contimeout", at, "60s");
  const tpslimit = optNum(raw, "tpslimit", at, 2);
  const pacerMinSleep = optStr(raw, "pacerMinSleep", at, "200ms");
  const encryptFilenames = optBool(raw, "encryptFilenames", at, false);
  if (encryptFilenames && crypt === undefined) {
    throw new RcloneErr(1, `${at}.encryptFilenames requires crypt`);
  }
  return {
    name,
    url,
    vendor,
    user,
    pass,
    root,
    skipLinks,
    skipHidden,
    sizeOnly,
    transfers,
    checkers,
    retries,
    retriesSleep,
    lowLevelRetries,
    timeout,
    contimeout,
    tpslimit,
    pacerMinSleep,
    encryptFilenames,
    ...(crypt === undefined ? {} : { crypt }),
  };
}

/**
 * 为什么: crypt 必须是密钥对数组. 旧的单对象会让人以为只能留一对, 解密没法换钥匙.
 * 明文是省略整个字段; 空数组或标了两对 default 都直接失败.
 * default / algorithm 可省略: 没标 default 时第一对当加密对, algorithm 默认 secretbox.
 */
function readCrypt(value: unknown, at: string): Crypt[] | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!Array.isArray(value)) {
    throw new RcloneErr(1, `${at} must be an array of key pairs`);
  }
  if (value.length === 0) {
    throw new RcloneErr(1, `${at} must have at least one key pair`);
  }
  const pairs = value.map((item, index) => readCryptPair(item, `${at}[${index}]`));
  assertUnique(pairs.map((item) => item.name), at);
  return markDefault(pairs, at);
}

function markDefault(pairs: Crypt[], at: string): Crypt[] {
  const marked = pairs.filter((item) => item.default);
  if (marked.length > 1) {
    throw new RcloneErr(1, `${at} requires exactly one default key pair`);
  }
  if (marked.length === 1) {
    return pairs;
  }
  return pairs.map((item, index) => index === 0 ? { ...item, default: true } : item);
}

function readCryptPair(value: unknown, at: string): Crypt {
  const raw = needObj(value, at);
  if ("password" in raw || "password2" in raw) {
    throw new RcloneErr(1, `${at} use key and salt, not password/password2`);
  }
  rejectUnknown(raw, CRYPT_PAIR_KEYS, at);
  const algorithm = optStr(raw, "algorithm", at, "secretbox");
  if (algorithm !== "secretbox") {
    throw new RcloneErr(1, `${at}.algorithm must be secretbox`);
  }
  return {
    name: needStr(raw, "name", at),
    key: needStr(raw, "key", at),
    salt: needStr(raw, "salt", at),
    algorithm: "secretbox",
    default: optBool(raw, "default", at, false),
  };
}

function readTasks(
  value: unknown,
  path: string,
  sources: Source[],
  targets: Target[],
): Task[] {
  const items = needArr(value, `${path} tasks`);
  const tasks = items.map((item, index) => readTask(item, `${path} tasks[${index}]`, sources, targets));
  assertUnique(tasks.map((item) => item.name), `${path} tasks`);
  return tasks;
}

function readTask(
  value: unknown,
  at: string,
  sources: Source[],
  targets: Target[],
): Task {
  const raw = needObj(value, at);
  rejectUnknown(raw, TASK_KEYS, at);
  const source = needStr(raw, "source", at);
  const target = needStr(raw, "target", at);
  if (!sources.some((item) => item.name === source)) {
    throw new RcloneErr(1, `${at}.source not found: ${source}`);
  }
  if (!targets.some((item) => item.name === target)) {
    throw new RcloneErr(1, `${at}.target not found: ${target}`);
  }
  const task: Task = {
    name: needStr(raw, "name", at),
    source,
    target,
    excludes: readExcludes(raw.excludes, `${at}.excludes`),
  };
  if ("transfers" in raw) {
    task.transfers = needNum(raw, "transfers", at);
  }
  if ("tpslimit" in raw) {
    task.tpslimit = needNum(raw, "tpslimit", at);
  }
  if ("timeout" in raw) {
    task.timeout = needStr(raw, "timeout", at);
  }
  if ("encryptFilenames" in raw) {
    task.encryptFilenames = needBool(raw, "encryptFilenames", at);
  }
  if ("skipHidden" in raw) {
    task.skipHidden = needBool(raw, "skipHidden", at);
  }
  const tgt = targets.find((item) => item.name === target);
  if (tgt === undefined) {
    throw new RcloneErr(1, `${at}.target not found: ${target}`);
  }
  const encNames = task.encryptFilenames ?? tgt.encryptFilenames;
  if (encNames && tgt.crypt === undefined) {
    throw new RcloneErr(1, `${at}.encryptFilenames requires crypt on the target`);
  }
  return task;
}

function readExcludes(value: unknown, at: string): string[] {
  if (value === undefined) {
    return [];
  }
  const items = needArr(value, at);
  return items.map((item, index) => {
    if (typeof item !== "string" || item.trim() === "") {
      throw new RcloneErr(1, `${at}[${index}] must be a non-empty string`);
    }
    return item;
  });
}

function assertUnique(names: string[], at: string): void {
  const seen = new Set<string>();
  for (const name of names) {
    if (seen.has(name)) {
      throw new RcloneErr(1, `${at} name duplicates ${name}`);
    }
    seen.add(name);
  }
}

function rejectUnknown(raw: Record<string, unknown>, allow: string[], at: string): void {
  for (const key of Object.keys(raw)) {
    if (!allow.includes(key)) {
      throw new RcloneErr(1, `${at} ${key} is not allowed`);
    }
  }
}

function parseObj(text: string, path: string): Record<string, unknown> {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new RcloneErr(1, `invalid config file ${path}`);
  }
  return needObj(parsed, path);
}

function needObj(value: unknown, at: string): Record<string, unknown> {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw new RcloneErr(1, `${at} must be an object`);
  }
  return value as Record<string, unknown>;
}

function needArr(value: unknown, at: string): unknown[] {
  if (!Array.isArray(value)) {
    throw new RcloneErr(1, `${at} must be an array`);
  }
  return value;
}

function needStr(raw: Record<string, unknown>, key: string, at: string): string {
  const value = needText(raw, key, at);
  if (value.trim() === "") {
    throw new RcloneErr(1, `${at}.${key} must be a non-empty string`);
  }
  return value;
}

function needText(raw: Record<string, unknown>, key: string, at: string): string {
  if (!(key in raw)) {
    throw new RcloneErr(1, `${at}.${key} is required`);
  }
  const value = raw[key];
  if (typeof value !== "string") {
    throw new RcloneErr(1, `${at}.${key} must be a string`);
  }
  return value;
}

function needBool(raw: Record<string, unknown>, key: string, at: string): boolean {
  if (!(key in raw)) {
    throw new RcloneErr(1, `${at}.${key} is required`);
  }
  return asBool(raw[key], `${at}.${key}`);
}

/**
 * 为什么: 传输开关有固定默认值. 省略就不该逼用户把整份示例抄进自己的配置.
 */
function optBool(
  raw: Record<string, unknown>,
  key: string,
  at: string,
  whenOmit: boolean,
): boolean {
  if (!(key in raw)) {
    return whenOmit;
  }
  return asBool(raw[key], `${at}.${key}`);
}

function asBool(value: unknown, at: string): boolean {
  if (typeof value !== "boolean") {
    throw new RcloneErr(1, `${at} must be a boolean`);
  }
  return value;
}

function needNum(raw: Record<string, unknown>, key: string, at: string): number {
  if (!(key in raw)) {
    throw new RcloneErr(1, `${at}.${key} is required`);
  }
  return asNum(raw[key], `${at}.${key}`);
}

function optNum(
  raw: Record<string, unknown>,
  key: string,
  at: string,
  whenOmit: number,
): number {
  if (!(key in raw)) {
    return whenOmit;
  }
  return asNum(raw[key], `${at}.${key}`);
}

function asNum(value: unknown, at: string): number {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new RcloneErr(1, `${at} must be a number`);
  }
  return value;
}

function optStr(
  raw: Record<string, unknown>,
  key: string,
  at: string,
  whenOmit: string,
): string {
  if (!(key in raw)) {
    return whenOmit;
  }
  const value = raw[key];
  if (typeof value !== "string" || value.trim() === "") {
    throw new RcloneErr(1, `${at}.${key} must be a non-empty string`);
  }
  return value;
}

function skipGap(text: string, start: number): number {
  let look = start;
  while (look < text.length) {
    const ch = text[look] ?? "";
    const next = text[look + 1] ?? "";
    if (/\s/.test(ch)) {
      look += 1;
      continue;
    }
    if (ch === "/" && next === "/") {
      look += 2;
      while (look < text.length && text[look] !== "\n") {
        look += 1;
      }
      continue;
    }
    if (ch === "/" && next === "*") {
      look += 2;
      while (look < text.length && !(text[look] === "*" && text[look + 1] === "/")) {
        look += 1;
      }
      look += 2;
      continue;
    }
    break;
  }
  return look;
}

function stripJsonc(text: string): string {
  let out = "";
  let index = 0;
  let inStr = false;
  let escape = false;
  while (index < text.length) {
    const ch = text[index] ?? "";
    const next = text[index + 1] ?? "";
    if (inStr) {
      out += ch;
      if (escape) {
        escape = false;
      } else if (ch === "\\") {
        escape = true;
      } else if (ch === "\"") {
        inStr = false;
      }
      index += 1;
      continue;
    }
    if (ch === "\"") {
      inStr = true;
      out += ch;
      index += 1;
      continue;
    }
    if (ch === "/" && next === "/") {
      index += 2;
      while (index < text.length && text[index] !== "\n") {
        index += 1;
      }
      continue;
    }
    if (ch === "/" && next === "*") {
      index += 2;
      while (index < text.length && !(text[index] === "*" && text[index + 1] === "/")) {
        index += 1;
      }
      index += 2;
      continue;
    }
    if (ch === ",") {
      const look = skipGap(text, index + 1);
      const end = text[look] ?? "";
      if (end === "}" || end === "]") {
        index += 1;
        continue;
      }
    }
    out += ch;
    index += 1;
  }
  return out;
}
