import { randomUUID } from "node:crypto";
import type { AppConfig, ImageRecord, SyncStatus } from "./types";
import type { RepoContext } from "./repo";
import { mustRun, runCmd } from "./process";

type RunInfo = {
  databaseId: number;
  status: string;
  conclusion: string | null;
  url: string;
};

const workflows = {
  mirror: "sync-image.yml",
  dockerfile: "build-dockerfile.yml",
} as const;

export type BuildResult = {
  status: Exclude<SyncStatus, "init">;
  runUrl: string;
};

function sleep(ms: number): Promise<void> {
  return Bun.sleep(ms);
}

function findRun(
  repo: RepoContext,
  workflow: string,
  requestId: string,
): RunInfo | undefined {
  const json = mustRun("gh", [
    "run",
    "list",
    "--repo",
    repo.repository,
    "--workflow",
    workflow,
    "--event",
    "workflow_dispatch",
    "--limit",
    "30",
    "--json",
    "databaseId,status,conclusion,url,displayTitle",
  ], repo.root);
  const runs = JSON.parse(json) as Array<RunInfo & { displayTitle: string }>;
  return runs.find((run) => run.displayTitle.includes(requestId));
}

function triggerArgs(
  image: ImageRecord,
  alias: string,
  version: string,
  requestId: string,
  cfg: AppConfig,
): string[] {
  const common = [
    "-f", `alias=${alias}`,
    "-f", `version=${version}`,
    "-f", `request_id=${requestId}`,
    "-f", `registry=${cfg.registry}`,
    "-f", `namespace=${cfg.namespace}`,
  ];
  if (image.type === "mirror") {
    return ["-f", `source=${image.source}`, ...common];
  }
  return ["-f", `script=${image.script}`, ...common];
}

/**
 * request_id 进入 run-name, 并发构建相同 tag 时仍能精确绑定本次 Action.
 */
export async function runBuild(
  repo: RepoContext,
  image: ImageRecord,
  alias: string,
  version: string,
  cfg: AppConfig,
): Promise<BuildResult> {
  const workflow = workflows[image.type];
  const requestId = randomUUID();
  mustRun("gh", [
    "workflow",
    "run",
    workflow,
    "--repo",
    repo.repository,
    "--ref",
    repo.defaultBranch,
    ...triggerArgs(image, alias, version, requestId, cfg),
  ], repo.root);

  const deadline = Date.now() + cfg.timeoutSeconds * 1000;
  let run: RunInfo | undefined;
  let lastStatus = "";
  while (Date.now() < deadline) {
    run = findRun(repo, workflow, requestId);
    if (run === undefined) {
      await sleep(2000);
      continue;
    }
    if (run.status !== lastStatus) {
      console.log(`action: ${run.status}`);
      console.log(`run: ${run.url}`);
      lastStatus = run.status;
    }
    if (run.status === "completed") {
      if (run.conclusion === "success") {
        return { status: "done", runUrl: run.url };
      }
      const logs = runCmd("gh", [
        "run",
        "view",
        String(run.databaseId),
        "--repo",
        repo.repository,
        "--log-failed",
      ], repo.root);
      const detail = logs.stdout || logs.stderr;
      if (detail.trim().length > 0) {
        console.error(detail.trimEnd());
      }
      return { status: "failed", runUrl: run.url };
    }
    await sleep(3000);
  }
  return {
    status: "timeout",
    runUrl: run?.url ?? `https://github.com/${repo.repository}/actions/workflows/${workflow}`,
  };
}
