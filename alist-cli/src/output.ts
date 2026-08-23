import type { Audience, FsEntry, PutOut, SyncOut } from "./types";

/** 为什么: agent 只要稳定 JSON, human 才需要对齐扫读. */
export function renderJson(data: unknown): string {
  return `${JSON.stringify(data, null, 2)}\n`;
}

export function renderText(text: string): string {
  if (text === "") {
    return "";
  }
  return text.endsWith("\n") ? text : `${text}\n`;
}

export function renderTable(
  headers: string[],
  rows: string[][],
): string {
  if (rows.length === 0) {
    return "";
  }
  const table = [headers, ...rows];
  const widths = headers.map((_, col) =>
    Math.max(...table.map((row) => (row[col] ?? "").length)),
  );
  return `${table
    .map((row) =>
      row.map((cell, col) => (cell ?? "").padEnd(widths[col] ?? 0)).join("  "),
    )
    .join("\n")}\n`;
}

export function pickBody(
  audience: Audience,
  text: string,
  data: unknown,
): string {
  return audience === "agent" ? renderJson(data) : text;
}

export function lsText(entries: FsEntry[]): string {
  if (entries.length === 0) {
    return "empty\n";
  }
  return renderTable(
    ["NAME", "SIZE", "MODIFIED", "TYPE"],
    entries.map((item) => [
      item.name,
      String(item.size),
      item.modified,
      item.isDir ? "dir" : "file",
    ]),
  );
}

export function putText(out: PutOut): string {
  if (out.task === undefined) {
    return `ok ${out.dst}\n`;
  }
  if (out.task.error !== "") {
    return `${out.dst} error ${out.task.error}\n`;
  }
  return `task ${out.task.id} ${out.dst}\n`;
}

export function syncText(out: SyncOut): string {
  const lines = out.files.map((item) => putText(item).trimEnd());
  lines.push(`uploaded ${out.count} files`);
  return `${lines.join("\n")}\n`;
}
