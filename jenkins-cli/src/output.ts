import type { Audience, JobRef, JobView, QueueItem, RunItem } from "./types";

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

export function jobLsText(jobs: JobRef[]): string {
  if (jobs.length === 0) {
    return "No jobs found\n";
  }
  return renderTable(
    ["NAME", "CLASS"],
    jobs.map((job) => [job.name, classTail(job.className)]),
  );
}

export function jobViewText(job: JobView): string {
  return [
    `name\t${job.name}`,
    `url\t${job.url}`,
    `buildable\t${job.buildable ? "true" : "false"}`,
    `lastBuild\t${fmtNum(job.lastBuild)}`,
    `lastSuccessfulBuild\t${fmtNum(job.lastSuccessfulBuild)}`,
    `lastFailedBuild\t${fmtNum(job.lastFailedBuild)}`,
    "",
  ].join("\n");
}

export function runLsText(items: RunItem[]): string {
  if (items.length === 0) {
    return "No runs found\n";
  }
  return renderTable(
    ["NUMBER", "RESULT", "TIMESTAMP", "DURATION"],
    items.map((item) => [
      String(item.number),
      item.result,
      fmtTime(item.timestamp),
      fmtDur(item.durationMs),
    ]),
  );
}

export function queueLsText(items: QueueItem[]): string {
  if (items.length === 0) {
    return "Queue is empty\n";
  }
  return renderTable(
    ["ID", "JOB", "WHY"],
    items.map((item) => [String(item.id), item.name, item.why]),
  );
}

export function pickBody(
  audience: Audience,
  text: string,
  data: unknown,
): string {
  return audience === "agent" ? renderJson(data) : text;
}

function classTail(className: string): string {
  const parts = className.split(".");
  return parts[parts.length - 1] ?? className;
}

function fmtNum(value: number | null): string {
  return value === null ? "-" : String(value);
}

function fmtTime(ts: number): string {
  if (ts <= 0) {
    return "-";
  }
  return new Date(ts).toISOString();
}

function fmtDur(ms: number): string {
  if (ms <= 0) {
    return "-";
  }
  const sec = Math.floor(ms / 1000);
  const min = Math.floor(sec / 60);
  const rem = sec % 60;
  return min > 0 ? `${min}m${rem}s` : `${rem}s`;
}
