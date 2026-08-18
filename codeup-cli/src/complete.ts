import { takeCmd } from "./args";
import { hintRepos, loadIndex } from "./cache";
import { loadFileCfg, pickProfile } from "./config";
import { parseOrgId } from "./org";

const ROOT_CMDS = ["init", "repos", "push", "cr", "webhook", "completion", "agent", "human"];
const CR_SUB = ["list", "get", "create"];
const WEBHOOK_SUB = ["list"];

/** 为什么: 补全只喂 Index 里的本地值, 不能为了提示再打 OpenAPI. */
export async function completeLines(tokens: string[], current: string): Promise<string> {
  const values = await completeValues(tokens, current);
  const matched = values.filter((item) => item.startsWith(current));
  return matched.length === 0 ? "" : `${matched.join("\n")}\n`;
}

export async function completeValues(tokens: string[], current: string): Promise<string[]> {
  const { flags, pos } = safeTake(tokens);
  const profile = flags.get("-p") ?? flags.get("--profile");
  if (tokens[tokens.length - 1] === "-p" || tokens[tokens.length - 1] === "--profile") {
    return await loadProfiles();
  }
  if (tokens[tokens.length - 1] === "--repo") {
    return await loadRepoHints(profile);
  }
  if (pos.length === 0) {
    return ROOT_CMDS;
  }
  if (pos.length === 1 && current !== "" && pos[0] === current) {
    return ROOT_CMDS;
  }
  const head = pos[0];
  if (head === "agent" || head === "human") {
    return completeAfterAudience(pos.slice(1), current, profile);
  }
  return completeCmd(pos, current, profile);
}

async function completeAfterAudience(
  pos: string[],
  current: string,
  profile?: string,
): Promise<string[]> {
  if (pos.length === 0 || (pos.length === 1 && pos[0] === current)) {
    return ROOT_CMDS.filter((item) => item !== "agent" && item !== "human");
  }
  return completeCmd(pos, current, profile);
}

async function completeCmd(
  pos: string[],
  current: string,
  profile?: string,
): Promise<string[]> {
  const head = pos[0];
  if (head === "cr") {
    if (pos.length <= 2 && (pos[1] === undefined || pos[1] === current)) {
      return CR_SUB;
    }
    if (pos[1] === "list" && pos.length <= 3) {
      return loadRepoHints(profile);
    }
    return [];
  }
  if (head === "webhook") {
    if (pos.length <= 2 && (pos[1] === undefined || pos[1] === current)) {
      return WEBHOOK_SUB;
    }
    if (pos[1] === "list" && pos.length <= 3) {
      return loadRepoHints(profile);
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

async function loadProfiles(): Promise<string[]> {
  try {
    const cfg = await loadFileCfg();
    return cfg.profiles.map((item) => item.name);
  } catch {
    return [];
  }
}

async function loadRepoHints(profile?: string): Promise<string[]> {
  try {
    const cfg = await loadFileCfg();
    const picked = pickProfile(cfg, profile);
    const idx = await loadIndex(picked.name, parseOrgId(picked.url));
    return hintRepos(idx.repos);
  } catch {
    return [];
  }
}
