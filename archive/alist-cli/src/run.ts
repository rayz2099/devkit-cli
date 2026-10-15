import { helpText, parseArgs } from "./args";
import { AlistClient, type AlistApi } from "./client";
import { completeLines } from "./complete";
import { loadFileCfg, resolveRuntime } from "./config";
import { fishScript } from "./fish";
import { pickBody, lsText, putText, renderText, syncText } from "./output";
import { joinRemote } from "./path";
import { AlistErr } from "./types";
import type { CliCmd, PutOut, Runtime, SyncOut } from "./types";
import { planLocal } from "./walk";

/** 为什么: 命令分发和渲染绑在一起, 避免每个 verb 自己决定 Audience. */
export async function runCmd(
  argv: string[],
  createClient: (runtime: Runtime) => AlistApi = (runtime) => new AlistClient(runtime),
  env: NodeJS.ProcessEnv = process.env,
): Promise<string> {
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

  const runtime = resolveRuntime(
    await loadFileCfg(cmd.config),
    cmd.audience,
    cmd.profile,
    env,
  );
  const client = createClient(runtime);
  return dispatch(cmd, runtime, client);
}

export async function dispatch(
  cmd: CliCmd,
  runtime: Runtime,
  client: AlistApi,
): Promise<string> {
  if (cmd.kind === "ls") {
    const entries = await client.listDir({
      path: cmd.path,
      password: cmd.password,
      page: cmd.page,
      size: cmd.size,
      refresh: cmd.refresh,
    });
    return pickBody(runtime.audience, lsText(entries), entries);
  }
  if (cmd.kind === "put") {
    const out = await doPut(client, cmd.src, cmd.dst, cmd.asTask, cmd.password);
    return pickBody(runtime.audience, putText(out), out);
  }
  if (cmd.kind === "mkdir") {
    await client.mkdir(cmd.path, cmd.password);
    return pickBody(runtime.audience, `ok ${cmd.path}\n`, { path: cmd.path, ok: true });
  }
  if (cmd.kind === "sync") {
    const out = await doSync(client, cmd.src, cmd.dst, cmd.asTask, cmd.password);
    return pickBody(runtime.audience, syncText(out), out);
  }
  throw new Error(`unhandled command: ${cmd.kind}`);
}

async function doPut(
  client: AlistApi,
  src: string,
  dst: string,
  asTask: boolean,
  password: string,
): Promise<PutOut> {
  const remote = joinRemote(dst);
  const task = await client.putFile(
    src,
    remote,
    asTask,
    password,
  );
  if (task !== undefined && task.error !== "") {
    throw new AlistErr(task.error, 2);
  }
  return { src, dst: remote, task };
}

async function doSync(
  client: AlistApi,
  src: string,
  dst: string,
  asTask: boolean,
  password: string,
): Promise<SyncOut> {
  const plan = planLocal(src);
  if (plan.kind === "file") {
    const file = await doPut(client, plan.srcPath, dst, asTask, password);
    return { dst: joinRemote(dst), count: 1, files: [file] };
  }
  if (plan.files.length === 0) {
    await client.mkdir(joinRemote(dst), password);
    return { dst: joinRemote(dst), count: 0, files: [] };
  }
  const files: PutOut[] = [];
  for (const file of plan.files) {
    files.push(
      await doPut(
        client,
        file.srcPath,
        joinRemote(dst, file.relPath),
        asTask,
        password,
      ),
    );
  }
  return { dst: joinRemote(dst), count: files.length, files };
}
