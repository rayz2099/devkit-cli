import { existsSync, statSync } from "node:fs";
import { basename, extname, isAbsolute, relative, resolve, sep } from "node:path";
import { RcloneErr } from "./types";
import type { Action, Audience, FileCfg, NameEnc, Target, Task } from "./types";

/** 为什么: 这些目录不是照片. 写死在命令里, 避免每个 target 都复制一份, 也避免漏掉 .grok. */
export const BUILTIN_EXCLUDES = [".git/**", ".grok/**", ".DS_Store"];

/**
 * 为什么: 根上的点前缀和子目录里的点前缀要两条规则.
 * 目录一旦命中, rclone 不再往里走, 隐藏文件夹整棵树都不会上传.
 */
export const HIDDEN_EXCLUDES = [".*", "**/.*"];

/** 为什么: argv 只消费已经合并好的计划, 任务覆盖和路径检查不能散落在拼参数的地方. */
export type Plan = {
  action: Action;
  task: Task;
  target: Target;
  local: string;
  rel: string;
  dest: string;
  label: string;
  excludes: string[];
  transfers: number;
  tpslimit: number;
  timeout: string;
  nameEnc: NameEnc;
  dryRun: boolean;
  progress: boolean;
  verbose: boolean;
};

export function findTask(cfg: FileCfg, name: string): Task {
  const task = cfg.tasks.find((item) => item.name === name);
  if (task === undefined) {
    const names = cfg.tasks.map((item) => item.name).join(", ");
    const suffix = names === "" ? "" : `. available: ${names}`;
    throw new RcloneErr(1, `task not found: ${name}${suffix}`);
  }
  return task;
}

/** 为什么: download/get 允许用户带上旧的 .bin, 但远端逻辑名必须是去掉后缀的那一个. */
export function stripBin(rel: string): string {
  if (rel.length > 4 && rel.toLowerCase().endsWith(".bin")) {
    return rel.slice(0, -4);
  }
  return rel;
}

function locatePath(root: string, given: string | undefined, action: Action): { local: string; rel: string } {
  const located = resolveRel(root, given);
  if (action !== "get" && action !== "download") {
    return located;
  }
  return { local: stripBin(located.local), rel: stripBin(located.rel) };
}

/** 为什么: 子路径必须落在 source 里面. 用 basename 当新的远端根, 会把相册传到另一个目录. */
export function resolveRel(root: string, given?: string): { local: string; rel: string } {
  const base = resolve(root);
  if (given === undefined || given === "") {
    return { local: base, rel: "" };
  }
  const local = resolve(base, given);
  const rel = relative(base, local);
  if (rel === "") {
    return { local: base, rel: "" };
  }
  if (rel.startsWith("..") || isAbsolute(rel)) {
    throw new RcloneErr(1, `path outside source: ${given}`);
  }
  return { local, rel: rel.split(sep).join("/") };
}

export function joinRoot(root: string, rel: string): string {
  const base = root.replace(/^\/+|\/+$/g, "");
  const sub = rel.replace(/^\/+|\/+$/g, "");
  if (sub === "") {
    return base;
  }
  if (base === "") {
    return sub;
  }
  return `${base}/${sub}`;
}

export function makePlan(
  cfg: FileCfg,
  action: Action,
  taskName: string,
  given: string | undefined,
  dryRun: boolean,
  audience: Audience,
  destPath?: string,
  cwd = process.cwd(),
  verbose = false,
): Plan {
  const task = findTask(cfg, taskName);
  const source = cfg.sources.find((item) => item.name === task.source);
  const target = cfg.targets.find((item) => item.name === task.target);
  if (source === undefined) {
    throw new RcloneErr(1, `tasks.${task.name}.source not found: ${task.source}`);
  }
  if (target === undefined) {
    throw new RcloneErr(1, `tasks.${task.name}.target not found: ${task.target}`);
  }
  const srcName = basename(source.path);
  if (target.root !== srcName) {
    throw new RcloneErr(1, `targets.${target.name}.root must match source folder ${srcName}`);
  }
  if ((action === "get" || action === "download") && (given === undefined || given === "")) {
    throw new RcloneErr(2, `${action} requires a path`);
  }
  const located = locatePath(source.path, given, action);
  const dest = joinRoot(target.root, located.rel);
  // 为什么: 文件名加密跟已经在网盘上的对象对得上. 任务覆盖 target, 漏任务字段用 target 的值.
  const encNames = task.encryptFilenames ?? target.encryptFilenames;
  const nameEnc: NameEnc = encNames ? "standard" : "off";
  const local = resolveWriteLocal(action, located, destPath, cwd);
  assertPullTree(action, task.name, located.rel, local);
  return {
    action,
    task,
    target,
    local,
    rel: located.rel,
    dest,
    label: `${target.name}:${dest}`,
    excludes: mergeExcludes(task.excludes, task.skipHidden ?? target.skipHidden),
    transfers: task.transfers ?? target.transfers,
    tpslimit: task.tpslimit ?? target.tpslimit,
    timeout: task.timeout ?? target.timeout,
    nameEnc,
    dryRun,
    progress: audience === "human" && action !== "check" && (verbose || (action !== "get" && action !== "download")),
    verbose,
  };
}

/**
 * 为什么: download 默认落到 cwd, 避免 copy 把文件路径当成目录.
 * dest 若已是目录, 再拼 basename, 这样 `download t a/b.JPG ~/tmp/x` 能直接用.
 */
function resolveWriteLocal(
  action: Action,
  located: { local: string; rel: string },
  destPath: string | undefined,
  cwd: string,
): string {
  if (action !== "download") {
    if (destPath === undefined || destPath === "") {
      return located.local;
    }
    return resolve(destPath);
  }
  const name = basename(located.rel);
  if (name === "" || name === "." || name === "..") {
    throw new RcloneErr(2, "download requires a file path");
  }
  if (destPath === undefined || destPath === "") {
    return resolve(cwd, name);
  }
  const dest = resolve(destPath);
  if (existsSync(dest) && statSync(dest).isDirectory()) {
    return resolve(dest, name);
  }
  return dest;
}

/** 为什么: rclone copy 的目标必须是目录. 单文件拉回 cwd 是 download, 不能让 pull 把库里的原图路径当目录. */
function assertPullTree(action: Action, task: string, rel: string, local: string): void {
  if (action !== "pull") {
    return;
  }
  const isFile = existsSync(local) ? statSync(local).isFile() : extname(local) !== "";
  if (!isFile) {
    return;
  }
  throw new RcloneErr(
    2,
    `pull expects a directory. download one file: rclone-cli download ${task} ${rel}`,
  );
}

function mergeExcludes(extra: string[], skipHidden: boolean): string[] {
  const out = [...BUILTIN_EXCLUDES];
  if (skipHidden) {
    for (const item of HIDDEN_EXCLUDES) {
      if (!out.includes(item)) {
        out.push(item);
      }
    }
  }
  for (const item of extra) {
    if (!out.includes(item)) {
      out.push(item);
    }
  }
  return out;
}
