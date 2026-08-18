import { parseArgs, helpText } from "./args";
import { JenkinsClient } from "./client";
import { completeLines } from "./complete";
import { loadFileCfg, resolveRuntime } from "./config";
import { fishScript } from "./fish";
import {
  jobLsText,
  jobViewText,
  pickBody,
  queueLsText,
  renderJson,
  renderText,
  runLsText,
} from "./output";
import type { CliCmd, Runtime } from "./types";

/** 为什么: 命令分发和渲染绑在一起, 避免每个 verb 自己决定 Audience. */
export async function runCmd(argv: string[]): Promise<string> {
  const cmd = parseArgs(argv);
  if (cmd.kind === "help") {
    return renderText(helpText(cmd.topic));
  }
  if (cmd.kind === "completion-fish") {
    return fishScript();
  }
  if (cmd.kind === "complete") {
    return completeLines(cmd.tokens, cmd.current);
  }

  const runtime = resolveRuntime(await loadFileCfg(), cmd.audience, cmd.profile);
  const client = new JenkinsClient(runtime);
  return dispatch(cmd, runtime, client);
}

export async function dispatch(
  cmd: CliCmd,
  runtime: Runtime,
  client: JenkinsClient,
): Promise<string> {
  if (cmd.kind === "job-ls") {
    const jobs = await client.listJobs(cmd.folder);
    return pickBody(runtime.audience, jobLsText(jobs), jobs);
  }
  if (cmd.kind === "job-view") {
    const job = await client.viewJob(cmd.jobPath);
    return pickBody(runtime.audience, jobViewText(job), job);
  }
  if (cmd.kind === "run-ls") {
    const items = await client.listRuns(cmd.jobPath, cmd.limit);
    return pickBody(runtime.audience, runLsText(items), items);
  }
  if (cmd.kind === "run-view") {
    if (cmd.slim) {
      const status = await client.viewStatus(cmd.jobPath, cmd.buildNo);
      return runtime.audience === "agent"
        ? renderJson(status)
        : renderText(`${status.job} #${status.build} ${status.result}`);
    }
    const info = await client.viewRun(cmd.jobPath, cmd.buildNo);
    if (runtime.audience === "agent") {
      return renderJson(info);
    }
    return renderText(
      [
        `${info.job} #${info.build} ${info.result}`,
        info.url,
        info.causes.join("; "),
      ].filter((line) => line !== "").join("\n"),
    );
  }
  if (cmd.kind === "run-start") {
    const out = await client.startRun(cmd.jobPath);
    return pickBody(
      runtime.audience,
      `triggered ${out.job}${out.queueLocation ? `\n${out.queueLocation}` : ""}\n`,
      out,
    );
  }
  if (cmd.kind === "run-rerun") {
    const out = await client.rerun(cmd.jobPath, cmd.buildNo);
    return pickBody(
      runtime.audience,
      `reran ${out.job} from #${cmd.buildNo}${out.queueLocation ? `\n${out.queueLocation}` : ""}\n`,
      out,
    );
  }
  if (cmd.kind === "run-cancel") {
    await client.cancelRun(cmd.jobPath, cmd.buildNo);
    const out = {
      jobPath: cmd.jobPath,
      build: cmd.buildNo,
      action: "stop" as const,
      status: "requested" as const,
    };
    return pickBody(
      runtime.audience,
      `cancel requested ${cmd.jobPath} #${cmd.buildNo}\n`,
      out,
    );
  }
  if (cmd.kind === "log") {
    return renderText(await client.readLog(cmd.jobPath, cmd.buildNo, cmd.tail));
  }
  if (cmd.kind === "queue-ls") {
    const items = await client.listQueue();
    return pickBody(runtime.audience, queueLsText(items), items);
  }
  throw new Error(`unhandled command: ${cmd.kind}`);
}
