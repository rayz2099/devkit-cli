import type { OutputFmt } from "./types";

/** 为什么: text/json 双通道必须和原 Go CLI 对齐, 脚本才能无感切换. */
export function render(
  format: OutputFmt,
  text: string,
  data: unknown,
): string {
  if (format === "json") {
    return `${JSON.stringify(data, null, 2)}\n`;
  }
  if (text === "") {
    return "";
  }
  return `${text}\n`;
}

/** 为什么: list 需要固定列对齐, 方便人眼扫 DATA_ID/GROUP. */
export function renderTable(
  summary: string,
  headers: string[],
  rows: string[][],
): string {
  const lines: string[] = [];
  if (summary.trim() !== "") {
    lines.push(summary);
  }

  const table = [headers, ...rows];
  const widths = headers.map((_, col) =>
    Math.max(...table.map((row) => (row[col] ?? "").length)),
  );

  for (const row of table) {
    lines.push(
      row
        .map((cell, col) => (cell ?? "").padEnd(widths[col] ?? 0))
        .join("  "),
    );
  }
  return lines.join("\n");
}
