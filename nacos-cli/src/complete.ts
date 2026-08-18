import { nsCands, resolveRuntime } from "./config";
import type { FileConfig, GlobalFlags, Runtime } from "./types";
import { groupsByDataId, uniqueDataIds } from "./fish";
import type { NacosClient } from "./client";

const VALUE_FLAGS = new Set([
  "--server-addr",
  "--username",
  "--password",
  "--namespace",
  "-o",
  "--output",
  "--data-id",
  "--group",
  "--content",
  "--search",
  "--page-no",
  "--page-size",
  "--service",
  "--ip",
  "--port",
  "--cluster",
  "--clusters",
  "--weight",
  "--ephemeral",
  "--healthy-only",
  "--prefix",
  "--to-complete",
]);

const BOOL_FLAGS = new Set(["--dev", "-h", "--help"]);

/** 为什么: 补全决策必须复用当前命令行里的 flags, 否则 --namespace 和位置参数会对不上. */
export function posArgs(tokens: string[]): string[] {
  const result: string[] = [];
  for (let index = 0; index < tokens.length; index += 1) {
    const token = tokens[index];
    if (token === undefined) {
      continue;
    }
    if (VALUE_FLAGS.has(token)) {
      index += 1;
      continue;
    }
    if (BOOL_FLAGS.has(token) || token.startsWith("-")) {
      continue;
    }
    result.push(token);
  }
  return result;
}

export function lastValueFlag(tokens: string[]): string | undefined {
  const last = tokens[tokens.length - 1];
  if (last !== undefined && VALUE_FLAGS.has(last)) {
    return last;
  }
  return undefined;
}

export function flagValue(tokens: string[], name: string): string {
  for (let index = 0; index < tokens.length; index += 1) {
    if (tokens[index] === name) {
      return tokens[index + 1] ?? "";
    }
  }
  return "";
}

function filterPrefix(values: string[], prefix: string): string[] {
  return values.filter((item) => item.startsWith(prefix));
}

/** 为什么: 把 cobra ValidArgsFunction 的分支收口到一处, fish 只负责把整行交回来. */
export async function completeLine(
  tokens: string[],
  current: string,
  fileCfg: FileConfig,
  createClient: (runtime: Runtime) => NacosClient,
): Promise<string[]> {
  const lastFlag = lastValueFlag(tokens);
  if (lastFlag === "--namespace") {
    return filterPrefix(nsCands(fileCfg), current);
  }
  if (lastFlag === "-o" || lastFlag === "--output") {
    return filterPrefix(["text", "json"], current);
  }
  if (lastFlag === "--search") {
    return filterPrefix(["blur", "accurate"], current);
  }
  if (lastFlag === "--ephemeral" || lastFlag === "--healthy-only") {
    return filterPrefix(["true", "false"], current);
  }

  const positionals = posArgs(tokens);
  const needDataIds =
    lastFlag === "--data-id" ||
    (lastFlag === undefined && positionals[0] === "config" && positionals[1] === "get" && positionals.length === 2);
  const needGroups =
    lastFlag === "--group" ||
    (lastFlag === undefined && positionals[0] === "config" && positionals[1] === "get" && positionals.length === 3);

  if (needDataIds || needGroups) {
    const { global } = takeLooseGlobals(tokens);
    const runtime = resolveRuntime(global, fileCfg);
    const items = await createClient(runtime).listItemsCached();
    if (needDataIds) {
      return uniqueDataIds(items, current);
    }
    const dataId = resolveDataId(tokens, positionals, lastFlag);
    return groupsByDataId(items, dataId, current);
  }

  if (lastFlag !== undefined) {
    return [];
  }
  if (positionals.length === 0) {
    return filterPrefix(["config", "naming", "completion"], current);
  }
  if (positionals[0] === "config" && positionals.length === 1) {
    return filterPrefix(["get", "put", "delete", "list"], current);
  }
  if (positionals[0] === "naming" && positionals.length === 1) {
    return filterPrefix(["register", "deregister", "instances"], current);
  }
  if (positionals[0] === "completion" && positionals.length === 1) {
    return filterPrefix(["fish"], current);
  }
  return [];
}

function takeLooseGlobals(tokens: string[]): { global: GlobalFlags } {
  const global: GlobalFlags = { dev: tokens.includes("--dev") };
  const read = (name: string): string | undefined => {
    const value = flagValue(tokens, name);
    return value === "" ? undefined : value;
  };
  const serverAddr = read("--server-addr");
  const username = read("--username");
  const password = read("--password");
  const namespace = read("--namespace");
  const output = read("--output") ?? read("-o");
  if (serverAddr !== undefined) {
    global.serverAddr = serverAddr;
  }
  if (username !== undefined) {
    global.username = username;
  }
  if (password !== undefined) {
    global.password = password;
  }
  if (namespace !== undefined) {
    global.namespace = namespace;
  }
  if (output === "text" || output === "json") {
    global.output = output;
  }
  return { global };
}

function resolveDataId(
  tokens: string[],
  positionals: string[],
  lastFlag: string | undefined,
): string {
  if (lastFlag === "--group") {
    const flagged = flagValue(tokens, "--data-id");
    if (flagged !== "") {
      return flagged;
    }
  }
  return positionals[2] ?? "";
}
