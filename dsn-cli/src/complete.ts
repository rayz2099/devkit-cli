import { cfgFlag, takeCmd } from "./args";
import { loadFileCfg, pickProfile, profileNames } from "./config";
import { kafkaComplete } from "./kafka-stmt";
import { readTopicCache } from "./kafka-cache";
import { pgCatalogComplete } from "./pg-stmt";

const ROOT_CMDS = ["query", "doctor", "completion", "agent", "human"];

/** 为什么: kafka topic 补全只读缓存; postgres 只补目录头, 不补表名. */
export async function completeLines(tokens: string[], current: string): Promise<string> {
  const values = await completeValues(tokens, current);
  const matched = values.filter((item) => item.startsWith(current));
  return matched.length === 0 ? "" : `${matched.join("\n")}\n`;
}

export async function completeValues(tokens: string[], current: string): Promise<string[]> {
  const last = tokens[tokens.length - 1];
  if (last === "-c" || last === "--config") {
    return [];
  }
  if (last === "-p" || last === "--profile") {
    return await loadProfiles(safeTake(tokens).flags);
  }
  if (last === "--output") {
    return ["json", "csv", "plain"];
  }
  const { pos } = safeTake(tokens);
  if (pos.length === 0 || (pos.length === 1 && pos[0] === current)) {
    return ROOT_CMDS;
  }
  if (pos[0] === "agent" || pos[0] === "human") {
    const rest = pos.slice(1);
    if (rest.length === 0 || (rest.length === 1 && rest[0] === current)) {
      return ["query", "doctor"];
    }
    if (rest[0] === "completion") {
      return ["fish"];
    }
  }
  if (pos[0] === "completion") {
    return pos.length <= 2 && (pos[1] === undefined || pos[1] === current) ? ["fish"] : [];
  }
  return await kindStmtComplete(tokens, pos);
}

function safeTake(tokens: string[]): { flags: Map<string, string>; pos: string[] } {
  try {
    return takeCmd(tokens);
  } catch {
    return { flags: new Map(), pos: tokens.filter((item) => !item.startsWith("-")) };
  }
}

async function loadProfiles(flags: Map<string, string>): Promise<string[]> {
  try {
    return profileNames(await loadFileCfg(cfgFlag(flags)));
  } catch {
    return [];
  }
}

async function kindStmtComplete(tokens: string[], pos: string[]): Promise<string[]> {
  const rest = queryRest(pos);
  if (rest === undefined) {
    return [];
  }
  const { flags } = safeTake(tokens);
  const name = flags.get("-p") ?? flags.get("--profile");
  if (name === undefined) {
    return [];
  }
  try {
    const profile = pickProfile(await loadFileCfg(cfgFlag(flags)), name);
    if (profile.kind === "kafka") {
      return kafkaComplete(rest, await readTopicCache(profile.name));
    }
    if (profile.kind === "postgres") {
      return pgCatalogComplete(rest);
    }
    return [];
  } catch {
    return [];
  }
}

function queryRest(pos: string[]): string[] | undefined {
  let rest = pos;
  if (rest[0] === "agent" || rest[0] === "human") {
    rest = rest.slice(1);
  }
  if (rest[0] !== "query") {
    return undefined;
  }
  return rest.slice(1);
}
