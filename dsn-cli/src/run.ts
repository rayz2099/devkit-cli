import { helpText, parseArgs } from "./args";
import { completeLines } from "./complete";
import { loadFileCfg, resolveRuntime } from "./config";
import { isTty, runConsole } from "./console";
import { runDriver } from "./drivers";
import { fishScript } from "./fish";
import { gateQuery } from "./gate";
import { agentLimit, capRows, renderQuery, renderText } from "./output";
import type { RunOut } from "./types";

/** 为什么: 命令分发和渲染绑在一起, 避免 Console 和 Query 各自决定 Audience. */
export async function runCmd(argv: string[]): Promise<RunOut> {
  const cmd = parseArgs(argv);
  if (cmd.kind === "help") {
    return { type: "stdout", body: renderText(helpText(cmd.topic)) };
  }
  if (cmd.kind === "completion-fish") {
    return { type: "stdout", body: fishScript() };
  }
  if (cmd.kind === "complete") {
    return { type: "stdout", body: await completeLines(cmd.tokens, cmd.current) };
  }

  const runtime = resolveRuntime(await loadFileCfg(), cmd.audience, cmd.profile);
  const profile = runtime.profile;

  if (cmd.kind === "console") {
    if (runtime.audience === "agent") {
      throw new Error("agent cannot enter Console");
    }
    if (!isTty()) {
      throw new Error("non-TTY requires query");
    }
    const code = runConsole(profile.kind, profile.url);
    return { type: "exit", code };
  }

  gateQuery(profile.kind, profile.access, cmd.stmt);
  const timeouts = {
    connectMs: Math.round(cmd.connectSec * 1000),
    execMs: Math.round(cmd.execSec * 1000),
  };
  const raw = await runDriver(profile.kind, profile.url, cmd.stmt, timeouts);
  const limit = runtime.audience === "agent" ? agentLimit(cmd.limit) : 0;
  const capped = capRows(raw.rows, limit);
  const body = renderQuery(
    runtime.audience,
    cmd.output,
    { columns: raw.columns, rows: capped.rows },
    capped.truncated,
  );
  return { type: "stdout", body };
}
