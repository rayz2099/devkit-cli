import { helpText, parseArgs } from "./args";
import { completeLines } from "./complete";
import { loadFileCfg, pickProfile, resolveRuntime } from "./config";
import { isTty, runConsole } from "./console";
import { checkProfiles, doctorOut, driverProbe } from "./doctor";
import { runDriver } from "./drivers";
import { fishScript } from "./fish";
import { gateQuery } from "./gate";
import { writeTopicCache } from "./kafka-cache";
import { isKafkaListen, isKafkaTopics } from "./kafka-stmt";
import { agentLimit, capRows, renderQuery, renderText } from "./output";
import type { CliCmd, FileCfg, RunOut } from "./types";

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

  const fileCfg = await loadFileCfg(cmd.config);
  if (cmd.kind === "doctor") {
    return await execDoctor(cmd, fileCfg);
  }

  const runtime = resolveRuntime(fileCfg, cmd.audience, cmd.profile);
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
  if (profile.kind === "kafka" && isKafkaTopics(cmd.stmt)) {
    await writeTopicCache(
      profile.name,
      raw.rows.map((row) => String(row.name ?? "")),
    );
  }
  const limit = runtime.audience === "agent" ? agentLimit(cmd.limit) : 0;
  const capped = capRows(raw.rows, limit);
  const humanListen =
    profile.kind === "kafka" && runtime.audience === "human" && isKafkaListen(cmd.stmt);
  const output = humanListen && cmd.output === "table" ? "json" : cmd.output;
  const body = renderQuery(
    runtime.audience,
    output,
    { columns: raw.columns, rows: capped.rows },
    capped.truncated,
    cmd.pretty,
  );
  return { type: "stdout", body };
}

/** 为什么: doctor 面向整份 Profile 列表, 失败要出表还要非 0, 不能在第一处 throw. */
async function execDoctor(
  cmd: Extract<CliCmd, { kind: "doctor" }>,
  fileCfg: FileCfg,
): Promise<RunOut> {
  const wanted = cmd.profile;
  const profiles = wanted === undefined ? fileCfg.profiles : [pickProfile(fileCfg, wanted)];
  const timeouts = {
    connectMs: Math.round(cmd.connectSec * 1000),
    execMs: Math.round(cmd.execSec * 1000),
  };
  const rows = await checkProfiles(profiles, timeouts, driverProbe);
  const failed = rows.some((row) => row.status === "fail");
  const data = doctorOut(rows);
  const body = renderQuery(cmd.audience, cmd.output, data, false, cmd.pretty);
  return { type: "stdout", body, code: failed ? 3 : 0 };
}
