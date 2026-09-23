import { mkdirSync } from "node:fs";
import { appendFile } from "node:fs/promises";
import { dirname } from "node:path";
import { runsFile } from "./config";
import { RcloneErr } from "./types";
import type { RunRec } from "./types";

/** 为什么: 追加一行 JSON, 中断时不会把整份历史重写成半截文件. */
export async function appendRun(home: string, rec: RunRec): Promise<void> {
  const path = runsFile(home);
  mkdirSync(dirname(path), { recursive: true });
  await appendFile(path, `${runLine(rec)}\n`);
}

/** 为什么: 最新的一次最有用. 文件按时间追加, 读的时候倒序, 不在配置里再存一份索引. */
export async function readRuns(home: string): Promise<RunRec[]> {
  const path = runsFile(home);
  const file = Bun.file(path);
  if (!(await file.exists())) {
    return [];
  }
  const text = await file.text();
  const lines = text.split("\n");
  const recs: RunRec[] = [];
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index] ?? "";
    if (line.trim() === "") {
      continue;
    }
    recs.push(parseLine(line, index + 1));
  }
  return recs.reverse();
}

export function runLine(rec: RunRec): string {
  const body: Record<string, unknown> = {
    id: rec.id,
    task: rec.task,
    action: rec.action,
    startedAt: rec.startedAt,
    finishedAt: rec.finishedAt,
    code: rec.code,
    local: rec.local,
    remote: rec.remote,
    dryRun: rec.dryRun,
    args: rec.args,
  };
  if (rec.bytes !== undefined) {
    body.bytes = rec.bytes;
  }
  if (rec.checks !== undefined) {
    body.checks = rec.checks;
  }
  if (rec.transfers !== undefined) {
    body.transfers = rec.transfers;
  }
  if (rec.error !== undefined) {
    body.error = rec.error;
  }
  return JSON.stringify(body);
}

function parseLine(line: string, number: number): RunRec {
  let parsed: unknown;
  try {
    parsed = JSON.parse(line);
  } catch {
    throw new RcloneErr(1, `runs.jsonl line ${number} is invalid`);
  }
  if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new RcloneErr(1, `runs.jsonl line ${number} is invalid`);
  }
  const raw = parsed as Record<string, unknown>;
  const action = raw.action;
  if (action !== "push" && action !== "pull" && action !== "check") {
    throw new RcloneErr(1, `runs.jsonl line ${number} is invalid`);
  }
  if (typeof raw.id !== "string" || typeof raw.task !== "string") {
    throw new RcloneErr(1, `runs.jsonl line ${number} is invalid`);
  }
  if (typeof raw.code !== "number" || typeof raw.dryRun !== "boolean") {
    throw new RcloneErr(1, `runs.jsonl line ${number} is invalid`);
  }
  const rec: RunRec = {
    id: raw.id,
    task: raw.task,
    action,
    startedAt: needRecStr(raw.startedAt, number),
    finishedAt: needRecStr(raw.finishedAt, number),
    code: raw.code,
    local: needRecStr(raw.local, number),
    remote: needRecStr(raw.remote, number),
    dryRun: raw.dryRun,
    args: readArgs(raw.args, number),
  };
  const bytes = optNum(raw, "bytes", number);
  const checks = optNum(raw, "checks", number);
  const transfers = optNum(raw, "transfers", number);
  if (bytes !== undefined) {
    rec.bytes = bytes;
  }
  if (checks !== undefined) {
    rec.checks = checks;
  }
  if (transfers !== undefined) {
    rec.transfers = transfers;
  }
  if (typeof raw.error === "string") {
    rec.error = raw.error;
  }
  return rec;
}

function needRecStr(value: unknown, number: number): string {
  if (typeof value !== "string") {
    throw new RcloneErr(1, `runs.jsonl line ${number} is invalid`);
  }
  return value;
}

function readArgs(value: unknown, number: number): string[] {
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string")) {
    throw new RcloneErr(1, `runs.jsonl line ${number} is invalid`);
  }
  return value;
}

function optNum(raw: Record<string, unknown>, key: string, number: number): number | undefined {
  if (!(key in raw) || raw[key] === undefined) {
    return undefined;
  }
  const value = raw[key];
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new RcloneErr(1, `runs.jsonl line ${number} is invalid`);
  }
  return value;
}
