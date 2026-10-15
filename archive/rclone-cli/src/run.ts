import { copyFileSync, mkdirSync, mkdtempSync, rmSync, unlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { basename, dirname, join } from "node:path";
import { usageText, helpText, parseArgs } from "./args";
import { buildArgv } from "./argv";
import { completeLines } from "./complete";
import { cfgFile, ensureEmptyConf, loadFileCfg, taskNames } from "./config";
import { buildLocalCrypt, buildPlainRemote, buildRemote, cryptOrder } from "./conn";
import type { CryptSecret, Secrets } from "./conn";
import { fishScript } from "./fish";
import { buildListArgv, buildProbeArgv, checkFailed, clipInv, listFiles, loadInv } from "./listing";
import type { InvReport } from "./listing";
import { renderInv, renderLs, renderRuns, renderXfer } from "./output";
import { makePlan } from "./plan";
import type { Plan } from "./plan";
import { appendRun, readRuns } from "./runs";
import { noopSink, obscureOne, spawnProc, stderrSink } from "./spawn";
import { parseStats, redactArgs, redactText } from "./stats";
import { RcloneErr } from "./types";
import type { CliCmd, CmdOut, CryptSuffix, ErrSink, FileCfg, RunRec, SpawnFn, Target } from "./types";

/** 为什么: 测试要换掉 HOME、时钟和 rclone 进程, 不能让单测打到真实网盘或用户配置. */
export type Deps = {
  home: string;
  spawn: SpawnFn;
  now: () => Date;
  id: () => string;
  cwd: string;
};

export function defaultDeps(env: NodeJS.ProcessEnv = process.env): Deps {
  const home = env.HOME;
  if (home === undefined || home === "") {
    throw new RcloneErr(1, "HOME is required");
  }
  return {
    home,
    spawn: spawnProc,
    now: () => new Date(),
    id: () => crypto.randomUUID(),
    cwd: process.cwd(),
  };
}

/** 为什么: 缺任务名必须在真正 spawn rclone 之前结束, 退出码 2, 并且把任务名列出来. */
export async function runCmd(argv: string[], deps: Deps = defaultDeps()): Promise<CmdOut> {
  const cmd = parseArgs(argv);
  if (cmd.kind === "help") {
    return { text: helpText(), code: 0 };
  }
  if (cmd.kind === "completion-fish") {
    return { text: fishScript(), code: 0 };
  }
  if (cmd.kind === "complete") {
    const text = await completeLines(cmd.tokens, cmd.current, deps.home);
    return { text, code: 0 };
  }
  if (cmd.kind === "usage") {
    return failUsage(deps, cmd.detail);
  }
  const cfg = await loadFileCfg(cfgFile(deps.home), deps.home);
  if (cmd.kind === "ls") {
    if (cmd.task === undefined) {
      return { text: renderLs(cfg, cmd.audience), code: 0 };
    }
    const report = clipInv(await runInv(cfg, {
      kind: "xfer",
      action: "check",
      audience: cmd.audience,
      task: cmd.task,
      ...(cmd.path === undefined ? {} : { path: cmd.path }),
      dryRun: false,
      verbose: false,
    }, deps, false), cmd.limit ?? 10);
    return { text: renderInv(report, cmd.audience, "ls"), code: 0 };
  }
  if (cmd.kind === "runs") {
    const recs = await readRuns(deps.home);
    return { text: renderRuns(recs, cmd.audience), code: 0 };
  }
  if (cmd.action === "check") {
    const report = await runInv(cfg, cmd, deps, true);
    return { text: renderInv(report, cmd.audience, "check"), code: checkFailed(report) ? 1 : 0 };
  }
  const rec = await runXfer(cfg, cmd, deps);
  return { text: renderXfer(rec, cmd.audience), code: rec.code };
}

async function failUsage(deps: Deps, detail?: string): Promise<never> {
  let names: string[] = [];
  let extra = detail;
  try {
    const cfg = await loadFileCfg(cfgFile(deps.home), deps.home);
    names = taskNames(cfg);
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    extra = extra === undefined || extra === "" ? msg : `${extra}\n${msg}`;
  }
  throw new RcloneErr(2, usageText(names, extra));
}

async function runXfer(cfg: FileCfg, cmd: XferCmd, deps: Deps): Promise<RunRec> {
  const startedAt = deps.now().toISOString();
  let plan: Plan | undefined;
  let secrets: Secrets | undefined;
  let args: string[] = [];
  let code = 1;
  let error: string | undefined;
  let bytes: number | undefined;
  let checks: number | undefined;
  let transfers: number | undefined;
  try {
    plan = makePlan(cfg, cmd.action, cmd.task, cmd.path, cmd.dryRun, cmd.audience, cmd.dest, deps.cwd, cmd.verbose);
    assertCreds(plan.target);
    const empty = ensureEmptyConf(deps.home);
    secrets = await obscureTarget(cfg.rcloneBin, empty, plan.target, deps.spawn);
    const sink = xferSink(plan);
    const bag = secretBag(plan.target, secrets);
    const result = await runAttempts(cfg.rcloneBin, empty, plan, secrets, deps.spawn, sink);
    args = result.args;
    bytes = result.bytes;
    checks = result.checks;
    transfers = result.transfers;
    code = result.code;
    if (code !== 0) {
      const cleaned = redactText(result.stderr.trim(), bag);
      error = cleaned === "" ? `rclone exited ${code}` : cleaned;
    }
  } catch (err) {
    code = err instanceof RcloneErr ? err.code : 1;
    const msg = err instanceof Error ? err.message : String(err);
    error = secrets === undefined || plan === undefined
      ? msg
      : redactText(msg, secretBag(plan.target, secrets));
  }
  const label = plan?.label ?? "";
  const rec: RunRec = {
    id: deps.id(),
    task: plan?.task.name ?? cmd.task,
    action: cmd.action,
    startedAt,
    finishedAt: deps.now().toISOString(),
    code,
    local: plan?.local ?? "",
    remote: label,
    dryRun: cmd.dryRun,
    args: redactArgs(args, label),
  };
  if (bytes !== undefined) {
    rec.bytes = bytes;
  }
  if (checks !== undefined) {
    rec.checks = checks;
  }
  if (transfers !== undefined) {
    rec.transfers = transfers;
  }
  if (error !== undefined) {
    rec.error = error;
  }
  await appendRun(deps.home, rec);
  return rec;
}

/**
 * 为什么: check/ls <task> 用两边 lsjson 算状态, 不再 spawn rclone check.
 * 123 没有可靠 hash, 只比 size; .bin 只在关文件名加密时当成同一份对象.
 */
async function runInv(cfg: FileCfg, cmd: XferCmd, deps: Deps, record: boolean): Promise<InvReport> {
  const startedAt = deps.now().toISOString();
  let plan: Plan | undefined;
  let secrets: Secrets | undefined;
  let args: string[] = [];
  let report: InvReport | undefined;
  let code = 1;
  let error: string | undefined;
  try {
    plan = makePlan(cfg, "check", cmd.task, cmd.path, false, cmd.audience);
    assertCreds(plan.target);
    const empty = ensureEmptyConf(deps.home);
    secrets = await obscureTarget(cfg.rcloneBin, empty, plan.target, deps.spawn);
    const remote = buildRemote(plan.target, secrets, plan.rel, plan.nameEnc, "none");
    const localArgs = buildListArgv(plan, empty, plan.local);
    const remoteArgs = buildListArgv(plan, empty, remote);
    args = remoteArgs;
    report = await loadInv(
      cfg.rcloneBin,
      localArgs,
      remoteArgs,
      deps.spawn,
      plan.task.name,
      plan.rel,
      usesLegacyBin(plan),
    );
    code = checkFailed(report) ? 1 : 0;
    if (code !== 0) {
      error = `missing=${report.counts.missing} differ=${report.counts.differ}`;
    }
  } catch (err) {
    code = err instanceof RcloneErr ? err.code : 1;
    const msg = err instanceof Error ? err.message : String(err);
    error = secrets === undefined || plan === undefined
      ? msg
      : redactText(msg, secretBag(plan.target, secrets));
  }
  if (record) {
    const rec: RunRec = {
      id: deps.id(),
      task: plan?.task.name ?? cmd.task,
      action: "check",
      startedAt,
      finishedAt: deps.now().toISOString(),
      code,
      local: plan?.local ?? "",
      remote: plan?.label ?? "",
      dryRun: false,
      args: redactArgs(args, plan?.label ?? ""),
    };
    if (report !== undefined) {
      rec.checks = report.rows.length;
    }
    if (error !== undefined) {
      rec.error = error;
    }
    await appendRun(deps.home, rec);
  }
  if (report === undefined) {
    throw new RcloneErr(code, error ?? "inventory failed");
  }
  return report;
}

type AttemptOut = {
  args: string[];
  code: number;
  stderr: string;
  bytes?: number;
  checks?: number;
  transfers?: number;
};

/**
 * 为什么: 关文件名加密后新对象不再带 .bin, 但线下网盘还留着 filename.bin.
 * push 只写同名; get 先同名再 .bin; pull 先铺同名, 再用 .bin 补本地还没有的文件.
 */
async function runAttempts(
  bin: string,
  cfgPath: string,
  plan: Plan,
  secrets: Secrets,
  spawn: SpawnFn,
  sink: ErrSink,
): Promise<AttemptOut> {
  if ((plan.action === "get" || plan.action === "download") && usesLegacyBin(plan)) {
    return runGet(bin, cfgPath, plan, secrets, spawn, sink);
  }
  if (plan.action === "pull" && usesLegacyBin(plan)) {
    return runPull(bin, cfgPath, plan, secrets, spawn, sink);
  }
  return spawnOnce(bin, cfgPath, plan, secrets, spawn, sink, "none");
}

async function runGet(
  bin: string,
  cfgPath: string,
  plan: Plan,
  secrets: Secrets,
  spawn: SpawnFn,
  sink: ErrSink,
): Promise<AttemptOut> {
  const names = cryptOrder(plan.target);
  if (names.length === 0) {
    throw new RcloneErr(1, `targets.${plan.target.name}.crypt requires a default key pair`);
  }
  const suffix = await probeSuffix(bin, cfgPath, plan, secrets, spawn);
  if (suffix === undefined) {
    return {
      args: [],
      code: 1,
      stderr: `remote file not found: ${plan.rel}`,
    };
  }
  const blobRel = suffix === ".bin" ? `${plan.rel}.bin` : plan.rel;
  const blob = basename(blobRel);
  const tmpDir = mkdtempSync(join(tmpdir(), "rclone-cli-"));
  const tmpIn = join(tmpDir, blob);
  try {
    const remote = buildPlainRemote(plan.target, secrets.pass, blobRel);
    if (plan.verbose) {
      sink.writeErr(`fetch ${blobRel}\n`);
    }
    const fetched = await spawnCopy(bin, cfgPath, plan, spawn, sink, remote, tmpIn);
    if (fetched.code !== 0 || plan.dryRun) {
      return fetched;
    }
    return await unlockLocal(bin, cfgPath, plan, secrets, spawn, sink, names, tmpDir, blob, fetched);
  } finally {
    rmSync(tmpDir, { recursive: true, force: true });
  }
}

/**
 * 为什么: 网盘只拉一次密文. 换钥匙是本地 crypt, 解错了不会再打 123.
 */
async function unlockLocal(
  bin: string,
  cfgPath: string,
  plan: Plan,
  secrets: Secrets,
  spawn: SpawnFn,
  sink: ErrSink,
  names: string[],
  tmpDir: string,
  blob: string,
  fetched: AttemptOut,
): Promise<AttemptOut> {
  let last = fetched;
  for (const pair of names) {
    const wire = needWire(secrets, plan.target.name, pair);
    const tmpOut = join(tmpDir, `out-${pair}`);
    const remote = buildLocalCrypt(tmpDir, wire, blob);
    if (plan.verbose) {
      sink.writeErr(`decrypt ${pair}\n`);
    }
    last = await spawnCopy(bin, cfgPath, plan, spawn, sink, remote, tmpOut);
    if (last.code !== 0) {
      continue;
    }
    // 为什么: 临时目录和目标可能不在同一卷, rename 会 EXDEV. 复制再删才是跨卷落地.
    mkdirSync(dirname(plan.local), { recursive: true });
    copyFileSync(tmpOut, plan.local);
    unlinkSync(tmpOut);
    return mergeAttempts(fetched, last);
  }
  return last;
}

function needWire(secrets: Secrets, target: string, pair: string): CryptSecret {
  const wire = secrets.crypt?.find((item) => item.name === pair);
  if (wire === undefined) {
    throw new RcloneErr(1, `targets.${target}.crypt.${pair} requires key and salt`);
  }
  return wire;
}

/**
 * 为什么: 文件名没加密时目录清单走 WebDAV, 不经过 crypt, 列目录不再被错钥匙拖住.
 */
async function probeSuffix(
  bin: string,
  cfgPath: string,
  plan: Plan,
  secrets: Secrets,
  spawn: SpawnFn,
): Promise<CryptSuffix | undefined> {
  const name = plan.rel.includes("/") ? plan.rel.slice(plan.rel.lastIndexOf("/") + 1) : plan.rel;
  const parent = plan.rel.includes("/") ? plan.rel.slice(0, plan.rel.lastIndexOf("/")) : "";
  const remote = buildPlainRemote(plan.target, secrets.pass, parent);
  const found = await listFiles(bin, buildProbeArgv(plan, cfgPath, remote), spawn);
  const names = new Set(found.map((item) => {
    const path = item.path;
    return path.includes("/") ? path.slice(path.lastIndexOf("/") + 1) : path;
  }));
  if (names.has(name)) {
    return "none";
  }
  if (names.has(`${name}.bin`)) {
    return ".bin";
  }
  return undefined;
}

async function runPull(
  bin: string,
  cfgPath: string,
  plan: Plan,
  secrets: Secrets,
  spawn: SpawnFn,
  sink: ErrSink,
): Promise<AttemptOut> {
  const names = cryptOrder(plan.target);
  if (names.length === 0) {
    throw new RcloneErr(1, `targets.${plan.target.name}.crypt requires a default key pair`);
  }
  let acc: AttemptOut | undefined;
  for (const [index, pair] of names.entries()) {
    const extraNone = index === 0 ? ["--exclude", "*.bin"] : ["--ignore-existing", "--exclude", "*.bin"];
    const first = await spawnOnce(bin, cfgPath, plan, secrets, spawn, sink, "none", extraNone, pair);
    const step = await fillBin(bin, cfgPath, plan, secrets, spawn, sink, first, pair);
    acc = acc === undefined ? step : mergeAttempts(acc, step);
  }
  if (acc === undefined) {
    throw new RcloneErr(1, `targets.${plan.target.name}.crypt requires a default key pair`);
  }
  return acc;
}

/**
 * 为什么: 第一对密钥先铺同名再补 .bin. 后面的对只填本地还没有的文件, 避免把已经解开的照片盖掉.
 */
async function fillBin(
  bin: string,
  cfgPath: string,
  plan: Plan,
  secrets: Secrets,
  spawn: SpawnFn,
  sink: ErrSink,
  first: AttemptOut,
  pair: string | undefined,
): Promise<AttemptOut> {
  if (first.code !== 0) {
    if (isMissingRemote(first.stderr, first.code)) {
      return spawnOnce(bin, cfgPath, plan, secrets, spawn, sink, ".bin", [], pair);
    }
    return first;
  }
  const second = await spawnOnce(
    bin,
    cfgPath,
    plan,
    secrets,
    spawn,
    sink,
    ".bin",
    ["--ignore-existing"],
    pair,
  );
  if (second.code !== 0 && isMissingRemote(second.stderr, second.code)) {
    return first;
  }
  if (second.code !== 0) {
    return second;
  }
  return mergeAttempts(first, second);
}

async function spawnOnce(
  bin: string,
  cfgPath: string,
  plan: Plan,
  secrets: Secrets,
  spawn: SpawnFn,
  sink: ErrSink,
  suffix: CryptSuffix,
  extra: string[] = [],
  pair?: string,
): Promise<AttemptOut> {
  const remote = buildRemote(plan.target, secrets, plan.rel, plan.nameEnc, suffix, pair);
  return spawnCopy(bin, cfgPath, plan, spawn, sink, remote, plan.local, extra);
}

async function spawnCopy(
  bin: string,
  cfgPath: string,
  plan: Plan,
  spawn: SpawnFn,
  sink: ErrSink,
  remote: string,
  dest: string,
  extra: string[] = [],
): Promise<AttemptOut> {
  const args = [...buildArgv(plan, cfgPath, remote, dest), ...extra];
  const ret = await spawn(bin, args, sink);
  return {
    args,
    code: ret.code,
    stderr: ret.stderr,
    ...pickStats(parseStats(ret.stderr)),
  };
}

function usesLegacyBin(plan: Plan): boolean {
  return plan.target.crypt !== undefined && plan.nameEnc === "off";
}

function mergeAttempts(first: AttemptOut, second: AttemptOut): AttemptOut {
  const out: AttemptOut = {
    args: first.args,
    code: first.code === 0 || second.code === 0 ? 0 : second.code,
    stderr: [first.stderr, second.stderr].filter((item) => item !== "").join("\n"),
  };
  out.bytes = sumOpt(first.bytes, second.bytes);
  out.checks = sumOpt(first.checks, second.checks);
  out.transfers = sumOpt(first.transfers, second.transfers);
  return out;
}

function sumOpt(left?: number, right?: number): number | undefined {
  if (left === undefined && right === undefined) {
    return undefined;
  }
  return (left ?? 0) + (right ?? 0);
}

function isMissingRemote(stderr: string, code: number): boolean {
  if (code === 0) {
    return false;
  }
  const text = stderr.toLowerCase();
  return text.includes("not found") || text.includes("directory not found") || text.includes("object not found");
}

function pickStats(stats: { bytes?: number; checks?: number; transfers?: number }): {
  bytes?: number;
  checks?: number;
  transfers?: number;
} {
  const out: { bytes?: number; checks?: number; transfers?: number } = {};
  if (stats.bytes !== undefined) {
    out.bytes = stats.bytes;
  }
  if (stats.checks !== undefined) {
    out.checks = stats.checks;
  }
  if (stats.transfers !== undefined) {
    out.transfers = stats.transfers;
  }
  return out;
}

function xferSink(plan: Plan): ErrSink {
  if (plan.verbose) {
    return stderrSink();
  }
  if (plan.action === "get" || plan.action === "download") {
    return noopSink();
  }
  if (plan.progress) {
    return stderrSink();
  }
  return noopSink();
}

function assertCreds(target: Target): void {
  if (target.user === "") {
    throw new RcloneErr(1, `targets.${target.name}.user is required`);
  }
  if (target.pass === "") {
    throw new RcloneErr(1, `targets.${target.name}.pass is required`);
  }
}

async function obscureTarget(
  bin: string,
  cfgFilePath: string,
  target: Target,
  spawn: SpawnFn,
): Promise<Secrets> {
  const pass = await obscureOne(bin, cfgFilePath, target.pass, spawn);
  if (target.crypt === undefined) {
    return { pass };
  }
  const crypt = [];
  for (const pair of target.crypt) {
    const password = await obscureOne(bin, cfgFilePath, pair.key, spawn);
    const password2 = await obscureOne(bin, cfgFilePath, pair.salt, spawn);
    crypt.push({ name: pair.name, password, password2 });
  }
  return { pass, crypt };
}

type XferCmd = Extract<CliCmd, { kind: "xfer" }>;

function secretBag(target: Target, secrets: Secrets): string[] {
  const bag = [target.pass, secrets.pass];
  if (target.crypt !== undefined) {
    for (const pair of target.crypt) {
      bag.push(pair.key, pair.salt);
    }
  }
  if (secrets.crypt !== undefined) {
    for (const pair of secrets.crypt) {
      bag.push(pair.password, pair.password2);
    }
  }
  return bag;
}
