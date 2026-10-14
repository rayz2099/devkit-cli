import { takeCmd } from "./args";
import { loadFileCfg, cfgPath, profileNames } from "./config";

const ROOT = ["auth", "project", "meta", "report", "query", "completion", "agent", "human", "help"];
const AUTH = ["login", "status"];
const PROJECT = ["ls"];
const META = ["events", "props"];
const REPORT = ["ls", "get"];
const QUERY = ["event", "property", "run", "report"];

/** 为什么: 补全只给已经能确定的命令词, 不去猜项目号和事件名. */
export async function completeLines(tokens: string[], current: string): Promise<string> {
  const values = await completeValues(tokens, current);
  const matched = values.filter((item) => item.startsWith(current));
  return matched.length === 0 ? "" : `${matched.join("\n")}\n`;
}

async function completeValues(tokens: string[], current: string): Promise<string[]> {
  const last = tokens[tokens.length - 1];
  if (last === "-c" || last === "--config" || last === "--qp") {
    return [];
  }
  let flags: Map<string, string>;
  let pos: string[];
  try {
    const taken = takeCmd(tokens);
    flags = taken.flags;
    pos = taken.pos;
  } catch {
    return [];
  }
  if (last === "-p" || last === "--profile") {
    return await names(flags);
  }
  if (pos.length === 0 || (pos.length === 1 && pos[0] === current)) {
    return ROOT;
  }
  const head = pos[0];
  const rest = head === "agent" || head === "human" ? pos.slice(1) : pos;
  if (rest.length === 0 || (rest.length === 1 && rest[0] === current)) {
    return ROOT.filter((item) => item !== "agent" && item !== "human");
  }
  return sub(rest, current);
}

function sub(pos: string[], current: string): string[] {
  const head = pos[0];
  const child = pos[1];
  if (child === undefined || child === current) {
    if (head === "auth") {
      return AUTH;
    }
    if (head === "project") {
      return PROJECT;
    }
    if (head === "meta") {
      return META;
    }
    if (head === "report") {
      return REPORT;
    }
    if (head === "query") {
      return QUERY;
    }
  }
  return [];
}

async function names(flags: Map<string, string>): Promise<string[]> {
  const path = flags.get("-c") ?? flags.get("--config") ?? cfgPath();
  try {
    return profileNames(loadFileCfg(path));
  } catch {
    return [];
  }
}
