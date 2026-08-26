import { takeCmd } from "./args";
import { loadFileCfg, profileNames } from "./config";

const ROOT_CMDS = ["query", "completion", "agent", "human"];

/** 为什么: 补全只给已经能确定的候选, 不去猜 statement 正文. */
export async function completeLines(tokens: string[], current: string): Promise<string> {
  const values = await completeValues(tokens, current);
  const matched = values.filter((item) => item.startsWith(current));
  return matched.length === 0 ? "" : `${matched.join("\n")}\n`;
}

export async function completeValues(tokens: string[], current: string): Promise<string[]> {
  const last = tokens[tokens.length - 1];
  if (last === "-p" || last === "--profile") {
    return await loadProfiles();
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
      return ["query"];
    }
    if (rest[0] === "completion") {
      return ["fish"];
    }
    return [];
  }
  if (pos[0] === "completion") {
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
    return profileNames(await loadFileCfg());
  } catch {
    return [];
  }
}
