import type { Audience, OutputFmt, QueryOut } from "./types";

/** 为什么: agent 截断发生在读结果之后, 禁止把 LIMIT 写进用户语句. */
export function capRows(
  rows: Array<Record<string, unknown>>,
  limit: number,
): { rows: Array<Record<string, unknown>>; truncated: boolean } {
  if (limit === 0 || rows.length <= limit) {
    return { rows, truncated: false };
  }
  return { rows: rows.slice(0, limit), truncated: true };
}

export function agentLimit(limit: number | undefined): number {
  return limit === undefined ? 1000 : limit;
}

export function renderQuery(
  audience: Audience,
  output: OutputFmt,
  data: QueryOut,
  truncated: boolean,
  pretty = false,
): string {
  if (audience === "agent") {
    return `${JSON.stringify({ rows: data.rows.map(plainRow), truncated }, jsonReplacer, 2)}\n`;
  }
  if (output === "json") {
    const rows = data.rows.map(plainRow);
    if (pretty) {
      return `${JSON.stringify(rows, jsonReplacer, 2)}\n`;
    }
    if (rows.length === 0) {
      return "";
    }
    return `${rows.map((row) => JSON.stringify(row, jsonReplacer)).join("\n")}\n`;
  }
  if (output === "csv") {
    return renderCsv(data);
  }
  if (output === "plain") {
    return renderPlain(data);
  }
  return renderTable(data);
}

function renderTable(data: QueryOut): string {
  if (data.rows.length === 0) {
    return "";
  }
  const cols = columnsOf(data);
  const lines = [cols, ...data.rows.map((row) => cols.map((col) => cellText(row[col])))];
  const widths = cols.map((_, col) => Math.max(...lines.map((line) => (line[col] ?? "").length)));
  return `${lines
    .map((line) => line.map((cell, col) => cell.padEnd(widths[col] ?? 0)).join("  "))
    .join("\n")}\n`;
}

function renderCsv(data: QueryOut): string {
  const cols = columnsOf(data);
  const lines = [
    cols.map(csvCell).join(","),
    ...data.rows.map((row) => cols.map((col) => csvCell(cellText(row[col]))).join(",")),
  ];
  return `${lines.join("\n")}\n`;
}

function renderPlain(data: QueryOut): string {
  if (data.rows.length === 0) {
    return "";
  }
  const cols = columnsOf(data);
  return `${data.rows.map((row) => cols.map((col) => cellText(row[col])).join("\t")).join("\n")}\n`;
}

function columnsOf(data: QueryOut): string[] {
  if (data.columns.length > 0) {
    return data.columns;
  }
  const seen = new Set<string>();
  const cols: string[] = [];
  for (const row of data.rows) {
    for (const key of Object.keys(row)) {
      if (!seen.has(key)) {
        seen.add(key);
        cols.push(key);
      }
    }
  }
  return cols;
}

function csvCell(value: string): string {
  if (/[",\n]/.test(value)) {
    return `"${value.replaceAll("\"", "\"\"")}"`;
  }
  return value;
}

function cellText(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (typeof value === "bigint") {
    return value.toString();
  }
  if (value instanceof Date) {
    return value.toISOString();
  }
  return JSON.stringify(value, jsonReplacer) ?? "";
}

function plainRow(row: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(row)) {
    out[key] = value;
  }
  return out;
}

function jsonReplacer(_key: string, value: unknown): unknown {
  if (typeof value === "bigint") {
    return value.toString();
  }
  return value;
}

export function renderText(text: string): string {
  if (text === "") {
    return "";
  }
  return text.endsWith("\n") ? text : `${text}\n`;
}
