import { extractGroups } from "./api";
import type { OutputMode } from "./types";

/** 为什么：groups 原始返回字段可能随 UQL 改变，human 模式只做轻量可读化。 */
export function formatGroups(body: unknown, mode: OutputMode): string {
  if (mode === "plain") {
    return `${JSON.stringify(body, null, 2)}\n`;
  }

  const groups = extractGroups(body);
  if (mode === "agent") {
    return `${JSON.stringify({ groups }, null, 2)}\n`;
  }

  if (groups.length === 0) {
    return "No groups.\n";
  }

  const lines = ["Groups:"];
  for (const group of groups) {
    lines.push(`- ${JSON.stringify(group)}`);
  }
  return `${lines.join("\n")}\n`;
}

export function formatJson(body: unknown): string {
  return `${JSON.stringify(body, null, 2)}\n`;
}
