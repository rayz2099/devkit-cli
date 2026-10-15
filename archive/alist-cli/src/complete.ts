import { cfgFlag, takeCmd } from "./args";
import { loadFileCfg, profileNames } from "./config";

const ROOT_CMDS = ["ls", "put", "mkdir", "sync", "completion", "agent", "human"];

/** 为什么: 补全只给已经能确定的候选, 不去猜远端路径拼写. */
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
  if (last === "-p" || last === "--profile") {
    return await loadProfiles(flags);
  }
  if (pos.length === 0) {
    return ROOT_CMDS;
  }
  if (pos.length === 1 && current !== "" && pos[0] === current) {
    return ROOT_CMDS;
  }
  const head = pos[0];
  if (head === "agent" || head === "human") {
    return completeAfterAudience(pos.slice(1), current);
  }
  return completeCmd(pos, current);
}

function completeAfterAudience(pos: string[], current: string): string[] {
  const cmds = ROOT_CMDS.filter((item) => item !== "agent" && item !== "human");
  if (pos.length === 0 || (pos.length === 1 && pos[0] === current)) {
    return cmds;
  }
  return completeCmd(pos, current);
}

function completeCmd(pos: string[], current: string): string[] {
  const head = pos[0];
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
    return profileNames(await loadFileCfg(cfgFlag(flags)));
  } catch {
    return [];
  }
}
