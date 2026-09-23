import type { Plan } from "./plan";
import { RcloneErr } from "./types";
import type { SpawnFn } from "./types";
import { noopSink } from "./spawn";

/** 为什么: 任务文件数按有限集处理. 一次 lsjson 拉全量, 才能把同名和 .bin 对齐, 不必再跑 rclone check. */
export type FileEnt = {
  path: string;
  size: number;
};

export type InvState = "ok" | "ok-bin" | "missing" | "differ" | "extra";

export type InvVia = "name" | "bin";

export type InvRow = {
  path: string;
  state: InvState;
  local?: number;
  remote?: number;
  via?: InvVia;
};

export type InvReport = {
  task: string;
  rel: string;
  rows: InvRow[];
  counts: Record<InvState, number>;
  limit?: number;
  total?: number;
};

export function parseLsJson(stdout: string): FileEnt[] {
  const text = stdout.trim();
  if (text === "") {
    return [];
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new RcloneErr(1, "rclone lsjson is not JSON");
  }
  if (!Array.isArray(parsed)) {
    throw new RcloneErr(1, "rclone lsjson must be an array");
  }
  const out: FileEnt[] = [];
  for (const item of parsed) {
    const ent = readEnt(item);
    if (ent !== undefined) {
      out.push(ent);
    }
  }
  return out;
}

/**
 * 为什么: 关文件名加密时远端同时可能有 foo 和 foo.bin.
 * 同名优先; .bin 只在同名不存在时当成同一份对象; 多出来的远端文件不让 check 失败.
 */
export function computeInv(task: string, rel: string, local: FileEnt[], remote: FileEnt[], allowBin: boolean): InvReport {
  const remoteBy = indexFiles(remote);
  const localBy = indexFiles(local);
  const used = new Set<string>();
  const rows: InvRow[] = [];
  const keys = [...localBy.keys()].sort();
  for (const path of keys) {
    const localSize = localBy.get(path);
    if (localSize === undefined) {
      throw new RcloneErr(1, `inventory lost local ${path}`);
    }
    const named = remoteBy.get(path);
    if (named !== undefined) {
      used.add(path);
      rows.push(sizeRow(path, localSize, named, "name"));
      continue;
    }
    const binPath = `${path}.bin`;
    const legacy = allowBin ? remoteBy.get(binPath) : undefined;
    if (legacy !== undefined) {
      used.add(binPath);
      rows.push(sizeRow(path, localSize, legacy, "bin"));
      continue;
    }
    rows.push({ path, state: "missing", local: localSize });
  }
  const extras = [...remoteBy.keys()].filter((path) => !used.has(path)).sort();
  for (const path of extras) {
    const remoteSize = remoteBy.get(path);
    if (remoteSize === undefined) {
      throw new RcloneErr(1, `inventory lost remote ${path}`);
    }
    rows.push({
      path,
      state: "extra",
      remote: remoteSize,
      via: allowBin && path.endsWith(".bin") ? "bin" : "name",
    });
  }
  return { task, rel, rows, counts: countStates(rows) };
}

export function checkFailed(report: InvReport): boolean {
  return report.counts.missing > 0 || report.counts.differ > 0;
}

/** 为什么: ls 默认只看前 10 条. 计数仍用全量, 避免截断后把 missing 藏掉. */
export function clipInv(report: InvReport, limit: number): InvReport {
  return {
    ...report,
    rows: report.rows.slice(0, limit),
    limit,
    total: report.rows.length,
  };
}

/** 为什么: 清单不是 copy. 不能带 --size-only/--progress, 也不能走 check/sync. */
export function buildListArgv(plan: Plan, cfgFile: string, root: string): string[] {
  return listArgv(plan, cfgFile, root, false);
}

/**
 * 为什么: download/get 先确认远端是同名还是 .bin.
 * 用满重试去 copyto 一个不存在的名字, 123 会把 10×15s 睡完, 看起来像卡死.
 */
export function buildProbeArgv(plan: Plan, cfgFile: string, root: string): string[] {
  return listArgv(plan, cfgFile, root, true);
}

export async function listFiles(bin: string, args: string[], spawn: SpawnFn): Promise<FileEnt[]> {
  return lsJson(bin, args, spawn, "remote");
}

function listArgv(plan: Plan, cfgFile: string, root: string, probe: boolean): string[] {
  const args = ["--config", cfgFile, "lsjson", root, "--files-only"];
  if (!probe) {
    args.push("--recursive");
  }
  if (!probe) {
    for (const pattern of plan.excludes) {
      args.push("--exclude", pattern);
    }
  }
  if (plan.target.skipLinks && !probe) {
    args.push("--skip-links");
  }
  if (probe) {
    args.push("--retries", "1");
    args.push("--retries-sleep", "1s");
    args.push("--low-level-retries", "1");
  } else {
    args.push("--retries", String(plan.target.retries));
    args.push("--retries-sleep", plan.target.retriesSleep);
    args.push("--low-level-retries", String(plan.target.lowLevelRetries));
  }
  args.push("--timeout", plan.timeout);
  args.push("--contimeout", plan.target.contimeout);
  args.push("--tpslimit", String(plan.tpslimit));
  return args;
}

export async function loadInv(
  bin: string,
  localArgs: string[],
  remoteArgs: string[],
  spawn: SpawnFn,
  task: string,
  rel: string,
  allowBin: boolean,
): Promise<InvReport> {
  const local = await lsJson(bin, localArgs, spawn, "local");
  const remote = await lsJson(bin, remoteArgs, spawn, "remote");
  return computeInv(task, rel, local, remote, allowBin);
}

async function lsJson(bin: string, args: string[], spawn: SpawnFn, side: "local" | "remote"): Promise<FileEnt[]> {
  const ret = await spawn(bin, args, noopSink());
  if (ret.code !== 0) {
    if (isMissingList(ret.stderr)) {
      return [];
    }
    const text = ret.stderr.trim();
    throw new RcloneErr(ret.code, text === "" ? `rclone lsjson ${side} exited ${ret.code}` : text);
  }
  return parseLsJson(ret.stdout);
}

function readEnt(value: unknown): FileEnt | undefined {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw new RcloneErr(1, "rclone lsjson entry must be an object");
  }
  const raw = value as Record<string, unknown>;
  if (raw.IsDir === true) {
    return undefined;
  }
  if (typeof raw.Path !== "string" || raw.Path.trim() === "") {
    throw new RcloneErr(1, "rclone lsjson entry.Path must be a non-empty string");
  }
  if (typeof raw.Size !== "number" || !Number.isFinite(raw.Size)) {
    throw new RcloneErr(1, `rclone lsjson entry.Size missing: ${raw.Path}`);
  }
  return { path: raw.Path.replaceAll("\\", "/"), size: raw.Size };
}

function indexFiles(items: FileEnt[]): Map<string, number> {
  const out = new Map<string, number>();
  for (const item of items) {
    out.set(item.path, item.size);
  }
  return out;
}

function sizeRow(path: string, local: number, remote: number, via: InvVia): InvRow {
  if (local === remote) {
    return { path, state: via === "bin" ? "ok-bin" : "ok", local, remote, via };
  }
  return { path, state: "differ", local, remote, via };
}

function countStates(rows: InvRow[]): Record<InvState, number> {
  const counts: Record<InvState, number> = {
    ok: 0,
    "ok-bin": 0,
    missing: 0,
    differ: 0,
    extra: 0,
  };
  for (const row of rows) {
    counts[row.state] += 1;
  }
  return counts;
}

function isMissingList(stderr: string): boolean {
  const text = stderr.toLowerCase();
  return text.includes("not found") || text.includes("directory not found") || text.includes("object not found");
}
