import { cfgFlag, takeCmd } from "./args";
import { hintRepos, loadIndex } from "./cache";
import { loadFileCfg, pickProfile } from "./config";
import { parseOrgId } from "./org";
import { MERGE_TYPES } from "./types";

const ROOT_CMDS = ["init", "repos", "push", "cr", "webhook", "completion", "agent", "human"];
const CR_SUB = ["list", "get", "create", "merge"];
const WEBHOOK_SUB = ["list"];

/** 为什么: 补全只喂 Index 里的本地值, 不能为了提示再打 OpenAPI. */
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
  const { flags, pos } = safeTake(tokens);
  const profile = flags.get("-p") ?? flags.get("--profile");
  if (last === "-p" || last === "--profile") {
    return await loadProfiles(flags);
  }
  if (tokens[tokens.length - 1] === "--repo") {
    return await loadRepoHints(flags, profile);
  }
  if (last === "--type") {
    return [...MERGE_TYPES];
  }
  if (pos.length === 0) {
    return ROOT_CMDS;
  }
  if (pos.length === 1 && current !== "" && pos[0] === current) {
    return ROOT_CMDS;
  }
  const head = pos[0];
  if (head === "agent" || head === "human") {
    return completeAfterAudience(pos.slice(1), current, flags, profile);
  }
  return completeCmd(pos, current, flags, profile);
}

async function completeAfterAudience(
  pos: string[],
  current: string,
  flags: Map<string, string>,
  profile?: string,
): Promise<string[]> {
  if (pos.length === 0 || (pos.length === 1 && pos[0] === current)) {
    return ROOT_CMDS.filter((item) => item !== "agent" && item !== "human");
  }
  return completeCmd(pos, current, flags, profile);
}

async function completeCmd(
  pos: string[],
  current: string,
  flags: Map<string, string>,
  profile?: string,
): Promise<string[]> {
  const head = pos[0];
  if (head === "cr") {
    if (pos.length <= 2 && (pos[1] === undefined || pos[1] === current)) {
      return CR_SUB;
    }
    if (pos[1] === "list" && pos.length <= 3) {
      return loadRepoHints(flags, profile);
    }
    return [];
  }
  if (head === "webhook") {
    if (pos.length <= 2 && (pos[1] === undefined || pos[1] === current)) {
      return WEBHOOK_SUB;
    }
    if (pos[1] === "list" && pos.length <= 3) {
      return loadRepoHints(flags, profile);
    }
    return [];
  }
  if (head === "completion") {
    return pos.length <= 2 && (pos[1] === undefined || pos[1] === current) ? ["fish"] : [];
  }
  return [];
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
    const cfg = await loadFileCfg(cfgFlag(flags));
    return cfg.profiles.map((item) => item.name);
  } catch {
    return [];
  }
}

async function loadRepoHints(flags: Map<string, string>, profile?: string): Promise<string[]> {
  try {
    const cfg = await loadFileCfg(cfgFlag(flags));
    const picked = pickProfile(cfg, profile);
    const idx = await loadIndex(picked.name, parseOrgId(picked.url));
    return hintRepos(idx.repos);
  } catch {
    return [];
  }
}
