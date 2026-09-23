import type { InvReport, InvRow, InvState } from "./listing";
import type { Audience, FileCfg, RunRec } from "./types";

const INV_ORDER: InvState[] = ["ok", "ok-bin", "missing", "differ", "extra"];
const INV_PROBLEMS: InvState[] = ["missing", "differ", "extra"];

/** 为什么: ls 不联网. 把口令打出来就等于把配置文件的 600 权限废掉. */
export function renderLs(cfg: FileCfg, audience: Audience): string {
  const view = {
    sources: cfg.sources.map((source) => ({ name: source.name, path: source.path })),
    targets: cfg.targets.map((target) => ({
      name: target.name,
      url: target.url,
      vendor: target.vendor,
      root: target.root,
      crypt: target.crypt === undefined ? 0 : target.crypt.length,
      cryptDefault: target.crypt?.find((item) => item.default)?.name,
      encryptFilenames: target.encryptFilenames,
      skipLinks: target.skipLinks,
      skipHidden: target.skipHidden,
      sizeOnly: target.sizeOnly,
    })),
    tasks: cfg.tasks.map((task) => ({
      name: task.name,
      source: task.source,
      target: task.target,
      excludes: task.excludes,
      ...(task.transfers === undefined ? {} : { transfers: task.transfers }),
      ...(task.tpslimit === undefined ? {} : { tpslimit: task.tpslimit }),
      ...(task.timeout === undefined ? {} : { timeout: task.timeout }),
      ...(task.encryptFilenames === undefined ? {} : { encryptFilenames: task.encryptFilenames }),
      ...(task.skipHidden === undefined ? {} : { skipHidden: task.skipHidden }),
    })),
  };
  if (audience === "agent") {
    return JSON.stringify(view, null, 2);
  }
  const lines = ["sources:"];
  for (const source of view.sources) {
    lines.push(`  ${source.name}  ${source.path}`);
  }
  lines.push("targets:");
  for (const target of view.targets) {
    const crypt = target.crypt === 0 ? "plain" : `crypt=${target.crypt}`;
    const names = target.encryptFilenames ? "names=on" : "names=off";
    lines.push(`  ${target.name}  ${target.vendor}  ${target.url}  root=${target.root}  ${crypt}  ${names}`);
  }
  lines.push("tasks:");
  for (const task of view.tasks) {
    lines.push(`  ${task.name}  ${task.source} -> ${task.target}`);
  }
  return lines.join("\n");
}

export function renderRuns(recs: RunRec[], audience: Audience): string {
  if (audience === "agent") {
    return JSON.stringify(recs, null, 2);
  }
  if (recs.length === 0) {
    return "no runs";
  }
  return recs.map((rec) => renderRunLine(rec)).join("\n");
}

/** 为什么: check 给人看的是对账结果, 不是又一次 copy 的 ok/failed. */
export function renderInv(report: InvReport, audience: Audience, verb: "ls" | "check"): string {
  if (audience === "agent") {
    return JSON.stringify({ verb, ...report }, null, 2);
  }
  const lines = [`${verb} ${report.task}${report.rel === "" ? "" : ` ${report.rel}`}`];
  if (report.limit !== undefined && report.total !== undefined) {
    lines.push(`  showing ${report.rows.length}/${report.total}  limit=${report.limit}`);
  }
  for (const state of INV_ORDER) {
    lines.push(`  ${state.padEnd(8)} ${report.counts[state]}`);
  }
  const show = verb === "ls" ? report.rows : report.rows.filter((row) => INV_PROBLEMS.includes(row.state));
  if (show.length === 0) {
    return lines.join("\n");
  }
  lines.push("");
  for (const row of show) {
    lines.push(`  ${formatRow(row)}`);
  }
  return lines.join("\n");
}

export function renderXfer(rec: RunRec, audience: Audience): string {
  if (audience === "agent") {
    return JSON.stringify(rec, null, 2);
  }
  if (rec.code === 0) {
    const dry = rec.dryRun ? " dry-run" : "";
    return `${rec.action} ${rec.task} ok${dry}`;
  }
  const reason = firstLine(rec.error ?? `code ${rec.code}`);
  return `${rec.action} ${rec.task} failed (${rec.code}): ${reason}`;
}

function renderRunLine(rec: RunRec): string {
  const dry = rec.dryRun ? " dry-run" : "";
  const head = `${rec.finishedAt} ${rec.action} ${rec.task} code=${rec.code}${dry} ${rec.remote}`;
  if (rec.error === undefined) {
    return head;
  }
  return `${head} error=${firstLine(rec.error)}`;
}

function formatRow(row: InvRow): string {
  const bits = [row.state.padEnd(8), row.path];
  if (row.local !== undefined) {
    bits.push(`local=${row.local}`);
  }
  if (row.remote !== undefined) {
    bits.push(`remote=${row.remote}`);
  }
  if (row.via !== undefined) {
    bits.push(`via=${row.via}`);
  }
  return bits.join("  ");
}

function firstLine(text: string): string {
  const line = text.split("\n")[0] ?? text;
  if (line.length <= 200) {
    return line;
  }
  return line.slice(0, 200);
}
