import { takeCmd } from "./args";
import { loadFileCfg, profileNames } from "./config";

const ROOT_CMDS = ["job", "run", "log", "queue", "completion", "agent", "human"];
const AGENT_CMDS = ["status", "list", "info", "log", "trigger", "job", "run", "queue"];
const JOB_SUB = ["ls", "view"];
const RUN_SUB = ["ls", "view", "start", "cancel", "rerun"];
const QUEUE_SUB = ["ls"];

/** 为什么: 补全只给已经能确定的候选, 不去猜 JobPath 拼写. */
export async function completeLines(tokens: string[], current: string): Promise<string> {
  const values = await completeValues(tokens, current);
  const matched = values.filter((item) => item.startsWith(current));
  return matched.length === 0 ? "" : `${matched.join("\n")}\n`;
}

export async function completeValues(tokens: string[], current: string): Promise<string[]> {
  const { flags, pos } = safeTake(tokens);
  if (tokens[tokens.length - 1] === "-p" || tokens[tokens.length - 1] === "--profile") {
    return await loadProfiles();
  }
  if (pos.length === 0) {
    return unique([...ROOT_CMDS, ...await loadProfilesIfFlag(flags)]);
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
  if (pos.length === 0 || (pos.length === 1 && pos[0] === current)) {
    return AGENT_CMDS;
  }
  return completeCmd(pos, current);
}

function completeCmd(pos: string[], current: string): string[] {
  const head = pos[0];
  if (head === "job") {
    return pos.length <= 2 && (pos[1] === undefined || pos[1] === current) ? JOB_SUB : [];
  }
  if (head === "run") {
    return pos.length <= 2 && (pos[1] === undefined || pos[1] === current) ? RUN_SUB : [];
  }
  if (head === "queue") {
    return pos.length <= 2 && (pos[1] === undefined || pos[1] === current) ? QUEUE_SUB : [];
  }
  if (head === "completion") {
    return pos.length <= 2 && (pos[1] === undefined || pos[1] === current) ? ["fish"] : [];
  }
  if (head === "status" || head === "list" || head === "info" || head === "trigger" || head === "log") {
    return [];
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
    return profileNames(await loadFileCfg());
  } catch {
    return [];
  }
}

async function loadProfilesIfFlag(flags: Map<string, string>): Promise<string[]> {
  if (flags.has("-p") || flags.has("--profile")) {
    return [];
  }
  return [];
}

function unique(values: string[]): string[] {
  return [...new Set(values)];
}
