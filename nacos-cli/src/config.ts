import { join } from "node:path";
import type { FileConfig, GlobalFlags, OutputFmt, Runtime } from "./types";

export const DEFAULT_SERVER = "127.0.0.1:8848";
export const DEFAULT_NS = "public";
export const DEFAULT_OUTPUT: OutputFmt = "text";
export const DEFAULT_CFG_REL = ".config/nacos-cli/config.json";

/** 为什么: 配置文件缺失是常态, 不能把缺文件当成启动失败. */
export function emptyFileCfg(): FileConfig {
  return {
    serverAddr: "",
    username: "",
    password: "",
    namespace: "",
    namespaces: [],
    output: "",
  };
}

/** 为什么: 字段名沿用原 Go JSON, 本机已有 ~/.config/nacos-cli/config.json 才能继续用. */
export function parseFileCfg(content: string, path: string): FileConfig {
  let parsed: unknown;
  try {
    parsed = JSON.parse(content);
  } catch {
    throw new Error(`invalid config file ${path}`);
  }

  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(`invalid config file ${path}`);
  }

  const raw = parsed as Record<string, unknown>;
  const namespaces = raw.namespaces;
  if (namespaces !== undefined && !Array.isArray(namespaces)) {
    throw new Error(`invalid config file ${path}: namespaces must be an array`);
  }

  return {
    serverAddr: readStr(raw.nacos_server_addr),
    username: readStr(raw.nacos_username),
    password: readStr(raw.nacos_password),
    namespace: readStr(raw.nacos_namespace),
    namespaces: Array.isArray(namespaces)
      ? namespaces.filter((item): item is string => typeof item === "string")
      : [],
    output: readStr(raw.nacos_output),
  };
}

/** 为什么: 只认 HOME, 单测才能把配置隔开, 也避免 os.homedir 读到真实家目录里的私有 namespace. */
export function cfgPath(home = process.env.HOME): string {
  if (home === undefined || home === "") {
    throw new Error("HOME is required");
  }
  return join(home, DEFAULT_CFG_REL);
}

/** 为什么: 每次执行读盘, fish 补全和真实命令看到同一份 XDG 配置. */
export async function loadFileCfg(
  path = cfgPath(),
): Promise<FileConfig> {
  const file = Bun.file(path);
  if (!(await file.exists())) {
    return emptyFileCfg();
  }
  return parseFileCfg(await file.text(), path);
}

/** 为什么: 优先级必须是 flags > env > file > default, 否则和旧脚本/CI 注入对不上. */
export function resolveRuntime(
  flags: GlobalFlags,
  fileCfg: FileConfig,
  env: NodeJS.ProcessEnv = process.env,
): Runtime {
  const serverAddr = resolveStr(
    flags.serverAddr,
    env,
    ["nacos_server_addr", "NACOS_SERVER_ADDR"],
    fileCfg.serverAddr,
    DEFAULT_SERVER,
  );
  const username = resolveStr(
    flags.username,
    env,
    ["nacos_username", "NACOS_USERNAME"],
    fileCfg.username,
    "",
  );
  const password = resolveStr(
    flags.password,
    env,
    ["nacos_password", "NACOS_PASSWORD"],
    fileCfg.password,
    "",
  );
  const namespace = resolveNs(flags.namespace, fileCfg, env);
  const outputRaw = resolveStr(
    flags.output,
    env,
    ["nacos_output", "NACOS_OUTPUT"],
    fileCfg.output,
    DEFAULT_OUTPUT,
  ).toLowerCase();

  if (outputRaw !== "text" && outputRaw !== "json") {
    throw new Error(`invalid output: ${outputRaw}`);
  }
  if (serverAddr.trim() === "") {
    throw new Error("server-addr is required");
  }

  return {
    serverAddr,
    username,
    password,
    namespace,
    output: outputRaw,
    dev: flags.dev,
  };
}

/** 为什么: namespace 还多一层 namespaces[], 补全候选和默认值都从这里长出来. */
export function resolveNs(
  flagNs: string | undefined,
  fileCfg: FileConfig,
  env: NodeJS.ProcessEnv = process.env,
): string {
  if (flagNs !== undefined) {
    return flagNs;
  }
  for (const key of ["nacos_namespace", "NACOS_NAMESPACE"]) {
    const value = env[key];
    if (value !== undefined) {
      return value;
    }
  }
  if (fileCfg.namespace.trim() !== "") {
    return fileCfg.namespace;
  }
  for (const item of fileCfg.namespaces) {
    const value = item.trim();
    if (value !== "") {
      return value;
    }
  }
  return DEFAULT_NS;
}

/** 为什么: --namespace 补全只暴露配置里声明过的空间, 避免把线上 ID 手打错. */
export function nsCands(fileCfg: FileConfig): string[] {
  const result: string[] = [];
  const seen = new Set<string>();
  const add = (value: string) => {
    const item = value.trim();
    if (item === "" || seen.has(item)) {
      return;
    }
    seen.add(item);
    result.push(item);
  };

  for (const item of fileCfg.namespaces) {
    add(item);
  }
  add(fileCfg.namespace);
  add(DEFAULT_NS);
  return result.length === 0 ? [DEFAULT_NS] : result;
}

function resolveStr(
  flagValue: string | undefined,
  env: NodeJS.ProcessEnv,
  envKeys: string[],
  fileValue: string,
  defaultValue: string,
): string {
  if (flagValue !== undefined) {
    return flagValue;
  }
  for (const key of envKeys) {
    const value = env[key];
    if (value !== undefined) {
      return value;
    }
  }
  if (fileValue.trim() !== "") {
    return fileValue;
  }
  return defaultValue;
}

function readStr(value: unknown): string {
  return typeof value === "string" ? value : "";
}
