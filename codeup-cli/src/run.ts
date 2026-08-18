import { parseArgs, helpText } from "./args";
import { CodeupClient } from "./client";
import { completeLines } from "./complete";
import { loadFileCfg, resolveRuntime } from "./config";
import { fishScript } from "./fish";
import { saveIndex } from "./cache";
import { currentBranch, pushBranch, resolveRepo } from "./git";
import {
  crListText,
  crViewText,
  maskHooks,
  pickBody,
  renderText,
  reposText,
  webhookText,
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
  const client = new CodeupClient(runtime);
  return dispatch(cmd, runtime, client);
}

export async function dispatch(
  cmd: CliCmd,
  runtime: Runtime,
  client: CodeupClient,
): Promise<string> {
  if (cmd.kind === "init") {
    const repositories = await client.listAllRepos();
    const idx = await saveIndex(runtime, repositories);
    return pickBody(
      runtime.audience,
      `indexed ${idx.repos.length} repositories\n`,
      {
        profile: idx.profile,
        count: idx.repos.length,
        updatedAt: idx.updatedAt,
        repositories: idx.repos.map((repo) => repo.pathNs),
      },
    );
  }
  if (cmd.kind === "repos") {
    const repositories = await client.listRepos({
      search: cmd.search,
      page: cmd.page,
      perPage: cmd.perPage,
    });
    return pickBody(
      runtime.audience,
      reposText(repositories),
      { repositories, count: repositories.length },
    );
  }
  if (cmd.kind === "push") {
    const branch = cmd.branch ?? await currentBranch();
    await pushBranch(cmd.remote, branch);
    const out = { remote: cmd.remote, branch, pushed: true as const };
    return pickBody(runtime.audience, `pushed ${branch} to ${cmd.remote}\n`, out);
  }
  if (cmd.kind === "cr-list") {
    const repo = await resolveRepo(
      runtime.orgId,
      runtime.profile.name,
      cmd.repo,
      undefined,
    );
    const changeRequests = await client.listCrs({
      repo,
      state: cmd.state,
      source: cmd.source,
      target: cmd.target,
      search: cmd.search,
      page: cmd.page,
      perPage: cmd.perPage,
    });
    return pickBody(
      runtime.audience,
      crListText(repo, changeRequests),
      { repo, changeRequests, count: changeRequests.length },
    );
  }
  if (cmd.kind === "cr-get") {
    const repo = await resolveRepo(
      runtime.orgId,
      runtime.profile.name,
      cmd.repo,
      undefined,
    );
    const cr = await client.getCr(repo, cmd.localId);
    return pickBody(runtime.audience, crViewText(repo, cr), { repo, ...cr });
  }
  if (cmd.kind === "cr-create") {
    const repo = await resolveRepo(
      runtime.orgId,
      runtime.profile.name,
      cmd.repo,
      undefined,
    );
    const target = cmd.target ?? (await client.getRepo(repo)).defBranch;
    if (target === "") {
      throw new Error(`repository ${repo} has empty defaultBranch; pass --target`);
    }
    const description = await readBody(cmd.body, cmd.bodyFile);
    const cr = await client.createCr({
      repo,
      source: cmd.source,
      target,
      title: cmd.title,
      description,
    });
    return pickBody(
      runtime.audience,
      `created CR #${cr.localId} in ${repo}: ${cr.title}\n${cr.crUrl === "" ? "" : `${cr.crUrl}\n`}`,
      { repo, ...cr },
    );
  }
  if (cmd.kind === "webhook-list") {
    const repo = await resolveRepo(
      runtime.orgId,
      runtime.profile.name,
      cmd.repo,
      undefined,
    );
    const hooks = maskHooks(
      await client.listHooks(repo, cmd.page, cmd.perPage),
      cmd.showSecrets,
    );
    return pickBody(
      runtime.audience,
      webhookText(hooks),
      { repo, webhooks: hooks, count: hooks.length },
    );
  }
  throw new Error(`unhandled command: ${cmd.kind}`);
}

async function readBody(
  body: string | undefined,
  bodyFile: string | undefined,
): Promise<string | undefined> {
  if (body !== undefined && bodyFile !== undefined) {
    throw new Error("use either --body or --body-file");
  }
  if (bodyFile !== undefined) {
    const file = Bun.file(bodyFile);
    if (!(await file.exists())) {
      throw new Error(`body file not found: ${bodyFile}`);
    }
    return await file.text();
  }
  return body;
}
