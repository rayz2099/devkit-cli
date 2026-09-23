import { cfgFile, loadFileCfg, taskNames } from "./config";

const ROOT = ["push", "pull", "check", "get", "download", "ls", "runs", "completion", "agent", "human"];
const AFTER = ["push", "pull", "check", "get", "download", "ls", "runs", "completion"];

/** 为什么: 补全只给命令和任务名. 路径交给 fish 的文件补全, 不去猜网盘上的密文名. */
export async function completeLines(tokens: string[], current: string, home: string): Promise<string> {
  const values = await completeValues(tokens, current, home);
  const matched = values.filter((item) => item.startsWith(current));
  if (matched.length === 0) {
    return "";
  }
  return `${matched.join("\n")}\n`;
}

export async function completeValues(tokens: string[], current: string, home: string): Promise<string[]> {
  const pos = tokens.filter((item) => !item.startsWith("-"));
  const head = pos[0];
  if (head === undefined || (pos.length === 1 && head === current)) {
    return ROOT;
  }
  if (head === "agent" || head === "human") {
    return completeAfter(pos.slice(1), current, home);
  }
  return completeCmd(pos, current, home);
}

async function completeAfter(pos: string[], current: string, home: string): Promise<string[]> {
  const head = pos[0];
  if (head === undefined || (pos.length === 1 && head === current)) {
    return AFTER;
  }
  return completeCmd(pos, current, home);
}

async function completeCmd(pos: string[], current: string, home: string): Promise<string[]> {
  const head = pos[0];
  if (head === "completion") {
    if (pos.length <= 2 && (pos[1] === undefined || pos[1] === current)) {
      return ["fish"];
    }
    return [];
  }
  if (head === "push" || head === "pull" || head === "check" || head === "get" || head === "download" || head === "ls") {
    if (pos.length <= 2 && (pos[1] === undefined || pos[1] === current)) {
      return loadTasks(home);
    }
    if (head === "push" || head === "pull" || head === "get" || head === "download") {
      const flags: string[] = [];
      if (!pos.includes("--dry-run")) {
        flags.push("--dry-run");
      }
      if ((head === "get" || head === "download") && !pos.includes("-v") && !pos.includes("--verbose")) {
        flags.push("--verbose");
      }
      return flags;
    }
    if (head === "ls" && !pos.includes("--limit") && !pos.includes("-n")) {
      return ["--limit"];
    }
  }
  return [];
}

async function loadTasks(home: string): Promise<string[]> {
  try {
    const cfg = await loadFileCfg(cfgFile(home), home);
    return taskNames(cfg);
  } catch {
    // 为什么: 补全发生在用户还没写完命令时. 配置缺失要给出空候选, 不能把错误文本塞进 fish 的候选列表.
    return [];
  }
}
