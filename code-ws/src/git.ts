import { existsSync, rmSync } from "node:fs";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import type { Cmd, RepoCfg } from "./types";

export type GitPlanArgs = {
  branch: string;
  baseBranch: string;
  remote: string;
  workspaceDir: string;
  repos: RepoCfg[];
  branchExists?: (repo: RepoCfg, branch: string) => boolean;
};

export type SyncBaseBranchPlanArgs = {
  baseBranch: string;
  remote: string;
  repos: RepoCfg[];
  branchWorktreePath?: (repo: RepoCfg, branch: string) => string | undefined;
};

export type ForkGitPlanArgs = {
  srcWorkspaceDir: string;
  dstWorkspaceDir: string;
  dstBranch: string;
  repos: RepoCfg[];
  branchExists?: (repo: RepoCfg, branch: string) => boolean;
};

function repoBaseBranch(
  repo: RepoCfg,
  defaultBranch: string,
): string {
  return repo.branch ?? defaultBranch;
}

/**
 * 只读 repo 可固定 checkout 分支, 因为它不一定跟随 workspace 任务分支开发。
 */
function repoWorkspaceBranch(
  repo: RepoCfg,
  workspaceBranch: string,
): string {
  return repo.checkoutBranch ?? workspaceBranch;
}

/**
 * 固定 checkout 分支需要先拉取自身, 因为它可能不是 repo 的主分支。
 */
function repoFetchBranch(
  repo: RepoCfg,
  defaultBranch: string,
): string {
  return repo.checkoutBranch ?? repoBaseBranch(
    repo,
    defaultBranch,
  );
}

function buildAddWorktreeArgs(
  repo: RepoCfg,
  branchExists: (repo: RepoCfg, branch: string) => boolean,
  dst: string,
  branch: string,
  remoteRef: string,
): string[] {
  if (repo.checkoutBranch !== undefined) {
    return [
      "git",
      "worktree",
      "add",
      "--detach",
      dst,
      remoteRef,
    ];
  }

  if (branchExists(
      repo,
      branch,
    )) {
    return ["git", "worktree", "add", dst, branch];
  }

  return [
    "git",
    "worktree",
    "add",
    "-b",
    branch,
    dst,
    remoteRef,
  ];
}

/**
 * 将命令编排和执行拆开, 因为 git 操作需要先能被测试精确验证。
 */
export function buildGitPlan(args: GitPlanArgs): Cmd[] {
  const cmds: Cmd[] = [];
  const branchExists = args.branchExists ?? localBranchExists;

  for (const repo of args.repos) {
    const fetchBranch = repoFetchBranch(
      repo,
      args.baseBranch,
    );
    const worktreeBranch = repoWorkspaceBranch(
      repo,
      args.branch,
    );
    const dst = join(args.workspaceDir, repo.name);
    cmds.push({
      repo: repo.name,
      branch: "workspace",
      step: "worktree-prune",
      cwd: repo.path,
      args: ["git", "worktree", "prune"],
    });
    cmds.push({
      repo: repo.name,
      branch: fetchBranch,
      step: "fetch",
      cwd: repo.path,
      args: ["git", "fetch", args.remote, fetchBranch],
    });
    const worktreeArgs = buildAddWorktreeArgs(
      repo,
      branchExists,
      dst,
      worktreeBranch,
      `${args.remote}/${fetchBranch}`,
    );
    cmds.push({
      repo: repo.name,
      branch: worktreeBranch,
      step: "worktree",
      cwd: repo.path,
      args: worktreeArgs,
    });
  }

  return cmds;
}

/**
 * 同步本地 base branch, 因为 workspace 分支后续会手动 merge 这个本地引用。
 */
export function buildSyncBaseBranchPlan(args: SyncBaseBranchPlanArgs): Cmd[] {
  const branchWorktreePath = args.branchWorktreePath ?? checkedOutBranchPath;

  return args.repos.map((repo) => {
    const baseBranch = repoFetchBranch(
      repo,
      args.baseBranch,
    );
    const checkoutPath = branchWorktreePath(
      repo,
      baseBranch,
    );
    if (checkoutPath !== undefined) {
      return {
        repo: repo.name,
        branch: baseBranch,
        step: "sync",
        cwd: checkoutPath,
        args: [
          "git",
          "pull",
          "--ff-only",
          args.remote,
          baseBranch,
        ],
      };
    }

    return {
      repo: repo.name,
      branch: baseBranch,
      step: "sync",
      cwd: repo.path,
      args: [
        "git",
        "fetch",
        args.remote,
        `${baseBranch}:${baseBranch}`,
      ],
    };
  });
}

/**
 * 从源 workspace 的当前 HEAD 派生新 worktree, 因为 fork 要保留已提交任务进度。
 */
export function buildForkGitPlan(args: ForkGitPlanArgs): Cmd[] {
  const cmds: Cmd[] = [];
  const branchExists = args.branchExists ?? localBranchExists;

  for (const repo of args.repos) {
    const src = join(args.srcWorkspaceDir, repo.name);
    const dstBranch = repoWorkspaceBranch(
      repo,
      args.dstBranch,
    );
    if (repo.checkoutBranch === undefined && branchExists(
        repo,
        dstBranch,
      )) {
      throw new Error(`branch already exists: ${repo.name} ${dstBranch}`);
    }

    cmds.push({
      repo: repo.name,
      branch: "workspace",
      step: "worktree-prune",
      cwd: repo.path,
      args: ["git", "worktree", "prune"],
    });
    const worktreeArgs = repo.checkoutBranch === undefined
      ? [
          "git",
          "worktree",
          "add",
          "-b",
          dstBranch,
          join(args.dstWorkspaceDir, repo.name),
          "HEAD",
        ]
      : [
          "git",
          "worktree",
          "add",
          "--detach",
          join(args.dstWorkspaceDir, repo.name),
          "HEAD",
        ];
    cmds.push({
      repo: repo.name,
      branch: dstBranch,
      step: "worktree-fork",
      cwd: src,
      args: worktreeArgs,
    });
  }

  return cmds;
}

/**
 * 本地分支是否存在会决定 worktree add 是否需要创建新分支。
 */
function localBranchExists(
  repo: RepoCfg,
  branch: string,
): boolean {
  const ret = spawnSync(
    "git",
    [
      "show-ref",
      "--verify",
      "--quiet",
      `refs/heads/${branch}`,
    ],
    {
      cwd: repo.path,
      stdio: "ignore",
    },
  );

  return ret.status === 0;
}

export function parseBranchWorktreePath(
  text: string,
  branch: string,
): string | undefined {
  let path = "";

  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (line.length === 0) {
      path = "";
      continue;
    }

    const worktree = line.match(/^worktree\s+(.+)$/);
    if (worktree?.[1] !== undefined) {
      path = worktree[1];
      continue;
    }

    if (line === `branch refs/heads/${branch}`) {
      return path.length > 0 ? path : undefined;
    }
  }

  return undefined;
}

/**
 * Git 不允许 fetch 直接改写已 checkout 的分支, 这种情况必须进入对应 worktree 拉取。
 */
function checkedOutBranchPath(
  repo: RepoCfg,
  branch: string,
): string | undefined {
  const ret = spawnSync(
    "git",
    [
      "worktree",
      "list",
      "--porcelain",
    ],
    {
      cwd: repo.path,
      encoding: "utf8",
      stdio: "pipe",
    },
  );

  if (ret.status !== 0) {
    const stderr = ret.stderr ?? "";
    const stdout = ret.stdout ?? "";
    const err = stderr.length > 0 ? stderr : stdout;
    throw new Error(`git check failed: ${err.trim()}`);
  }

  return parseBranchWorktreePath(
    ret.stdout,
    branch,
  );
}

export function buildWorktreeRemovePlan(
  wsDir: string,
  repos: RepoCfg[],
): Cmd[] {
  return repos
    .map((repo) => ({
      repo: repo.name,
      branch: "workspace",
      step: "worktree-remove",
      cwd: repo.path,
      args: [
        "git",
        "worktree",
        "remove",
        "--force",
        join(wsDir, repo.name),
      ],
      ignoredErrors: ["is not a working tree"],
    }));
}

export function assertGitTargets(
  wsDir: string,
  repos: RepoCfg[],
): void {
  if (existsSync(wsDir)) {
    throw new Error(existingDirMsg("workspace dir", wsDir));
  }

  for (const repo of repos) {
    if (!existsSync(repo.path)) {
      throw new Error(`repo not found: ${repo.path}`);
    }
    const dst = join(wsDir, repo.name);
    if (existsSync(dst)) {
      throw new Error(existingDirMsg("worktree dir", dst));
    }
  }
}

export function confirmDeletePrompt(path: string): string {
  return [
    `workspace already exists: ${path}`,
    "delete it and continue? [y/N] ",
  ].join("\n");
}

export function shouldDeleteExistingDir(
  answer: string | null | undefined,
): boolean {
  return answer === "y" || answer === "Y";
}

/**
 * 提前列出 remove 后果, 因为 worktree 删除会让未提交内容离开文件系统。
 */
export function confirmRemoveProjectPrompt(
  repo: string,
  path: string,
  baseBranch: string,
): string {
  return [
    `remove repo from workspace: ${repo}`,
    `worktree path: ${path}`,
    "effects:",
    "  - runs git worktree remove --force for this repo",
    "  - removes it from project.yml and .code-workspace folders",
    "  - does not merge, push, or delete the git branch",
    `confirm you have merged the branch into ${baseBranch}. continue? [y/N] `,
  ].join("\n");
}

/**
 * 只接受明确单字符确认, 因为 remove 会删除当前 worktree 目录中的未提交内容。
 */
export function shouldConfirmRemoveProject(
  answer: string | null | undefined,
): boolean {
  return answer === "y" || answer === "Y";
}

export function hasUnsafeWork(
  status: string,
  commits: string,
): boolean {
  const commitCount = Number.parseInt(commits.trim(), 10);
  return status.trim().length > 0 || commitCount > 0;
}

export function hasUncommittedWork(status: string): boolean {
  return status.trim().length > 0;
}

export function unsafeWorkMsg(
  repo: string,
  path: string,
  reason: string,
): string {
  return [
    `workspace cleanup aborted: repo=${repo} path=${path}`,
    `reason: ${reason}`,
  ].join("\n");
}

function gitOutput(
  cwd: string,
  args: string[],
): string {
  const ret = spawnSync(
    args[0] ?? "git",
    args.slice(1),
    {
      cwd,
      encoding: "utf8",
      stdio: "pipe",
    },
  );

  if (ret.status !== 0) {
    const err = ret.stderr.length > 0 ? ret.stderr : ret.stdout;
    throw new Error(`git check failed: ${err.trim()}`);
  }

  return ret.stdout;
}

function isGitWorktree(path: string): boolean {
  const ret = spawnSync(
    "git",
    [
      "-C",
      path,
      "rev-parse",
      "--is-inside-work-tree",
    ],
    {
      encoding: "utf8",
      stdio: "pipe",
    },
  );

  return ret.status === 0 && ret.stdout.trim() === "true";
}

/**
 * 用合入后的 tree 判断是否已合并, 因为 squash/rebase 合入后 commit SHA 不再是 master 祖先.
 */
function isMergedByTree(
  cwd: string,
  head: string,
  target: string,
): boolean {
  const mergeRet = spawnSync(
    "git",
    [
      "merge-tree",
      "--write-tree",
      target,
      head,
    ],
    {
      cwd,
      encoding: "utf8",
      stdio: "pipe",
    },
  );

  if (mergeRet.status === 1) {
    return false;
  }
  if (mergeRet.status !== 0) {
    const err = mergeRet.stderr.length > 0 ? mergeRet.stderr : mergeRet.stdout;
    throw new Error(`git check failed: ${err.trim()}`);
  }

  const treeRet = spawnSync(
    "git",
    [
      "rev-parse",
      `${target}^{tree}`,
    ],
    {
      cwd,
      encoding: "utf8",
      stdio: "pipe",
    },
  );

  if (treeRet.status !== 0) {
    const err = treeRet.stderr.length > 0 ? treeRet.stderr : treeRet.stdout;
    throw new Error(`git check failed: ${err.trim()}`);
  }

  return mergeRet.stdout.trim() === treeRet.stdout.trim();
}

export type DisposableWorktreeDeps = {
  pathExists?: (path: string) => boolean;
  isTree?: (path: string) => boolean;
  gitOut?: (cwd: string, args: string[]) => string;
  isMerged?: (
    cwd: string,
    head: string,
    target: string,
  ) => boolean;
};

/**
 * 删除 worktree 前必须确认没有未提交改动, 且 HEAD 相对最新远端目标分支没有独有变更.
 */
export function assertDisposableWorktrees(
  wsDir: string,
  repos: RepoCfg[],
  remote: string,
  baseBranch: string,
  deps: DisposableWorktreeDeps = {},
): void {
  const pathExists = deps.pathExists ?? existsSync;
  const isTree = deps.isTree ?? isGitWorktree;
  const gitOut = deps.gitOut ?? gitOutput;
  const isMerged = deps.isMerged ?? isMergedByTree;

  for (const repo of repos) {
    const repoBranch = repoBaseBranch(
      repo,
      baseBranch,
    );
    const dst = join(wsDir, repo.name);
    if (!pathExists(dst) || !isTree(dst)) {
      continue;
    }

    const status = gitOut(
      dst,
      ["git", "status", "--porcelain"],
    );

    if (status.trim().length > 0) {
      throw new Error(unsafeWorkMsg(repo.name, dst, "uncommitted changes"));
    }

    gitOut(
      dst,
      [
        "git",
        "fetch",
        "--quiet",
        remote,
        repoBranch,
      ],
    );
    const remoteRef = `${remote}/${repoBranch}`;
    if (!isMerged(dst, "HEAD", remoteRef)) {
      const commits = gitOut(
        dst,
        [
          "git",
          "rev-list",
          "--count",
          `${remoteRef}..HEAD`,
        ],
      );
      throw new Error(unsafeWorkMsg(
        repo.name,
        dst,
        `unmerged commits: ${commits.trim()}`,
      ));
    }
  }
}

/**
 * fork 前只阻断未提交内容, 因为已提交但未 push 的 commit 是新分支的合法起点。
 */
export function assertCleanWorktrees(
  wsDir: string,
  repos: RepoCfg[],
): void {
  for (const repo of repos) {
    const dst = join(wsDir, repo.name);
    if (!existsSync(dst)) {
      throw new Error(`workspace repo path not found: ${dst}`);
    }

    const status = gitOutput(
      dst,
      ["git", "status", "--porcelain"],
    );
    if (hasUncommittedWork(status)) {
      throw new Error(unsafeWorkMsg(repo.name, dst, "uncommitted changes"));
    }
  }
}

/**
 * 已存在的 workspace 可能来自上次失败的 init, 所以允许显式确认后清理。
 */
export function prepareGitTargets(
  wsDir: string,
  repos: RepoCfg[],
  remote: string,
  baseBranch: string,
  verbose = false,
  readAnswer: (msg: string) => string | null = prompt,
  removeDir: (path: string) => void = (path) => {
    rmSync(path, {
      recursive: true,
      force: true,
    });
  },
): void {
  if (existsSync(wsDir)) {
    const answer = readAnswer(confirmDeletePrompt(wsDir));
    if (!shouldDeleteExistingDir(answer)) {
      throw new Error(`workspace init aborted: ${wsDir}`);
    }
    assertDisposableWorktrees(
      wsDir,
      repos,
      remote,
      baseBranch,
    );
    runGitPlan(
      buildWorktreeRemovePlan(wsDir, repos),
      verbose,
    );
    removeDir(wsDir);
  }

  assertGitTargets(
    wsDir,
    repos,
  );
}

export function existingDirMsg(
  label: string,
  path: string,
): string {
  return [
    `${label} already exists: ${path}`,
    "delete it and continue? [y/N]",
  ].join("\n");
}

export function gitStepLabel(cmd: Cmd): string {
  return `[${cmd.repo}] ${cmd.step} ${cmd.branch}`;
}

function summaryLine(text: string): string {
  const lines = text
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0);

  const err = lines.find((line) => {
    const lower = line.toLowerCase();
    return lower.startsWith("error:") || lower.startsWith("fatal:");
  });

  return err ?? lines.at(-1) ?? "git exited with non-zero status";
}

export function gitFailureMsg(
  cmd: Cmd,
  stderr: string,
): string {
  return [
    `git failed: repo=${cmd.repo} branch=${cmd.branch} step=${cmd.step}`,
    `reason: ${summaryLine(stderr)}`,
  ].join("\n");
}

export function shouldIgnoreCmdFailure(
  cmd: Cmd,
  text: string,
): boolean {
  return cmd.ignoredErrors?.some((err) => text.includes(err)) ?? false;
}

function detailLog(
  stdout: string,
  stderr: string,
): string {
  return [stdout.trim(), stderr.trim()]
    .filter((text) => text.length > 0)
    .join("\n");
}

export function runCmd(
  cmd: Cmd,
  verbose = false,
): void {
  const [bin, ...args] = cmd.args;
  if (bin === undefined) {
    throw new Error("command args must not be empty");
  }

  console.log(gitStepLabel(cmd));
  const ret = spawnSync(bin, args, {
    cwd: cmd.cwd,
    encoding: "utf8",
    stdio: "pipe",
  });

  if (ret.status !== 0) {
    const err = ret.stderr.length > 0 ? ret.stderr : ret.stdout;
    if (shouldIgnoreCmdFailure(cmd, err)) {
      return;
    }
    const msg = gitFailureMsg(cmd, err);
    const detail = detailLog(
      ret.stdout,
      ret.stderr,
    );
    if (verbose && detail.length > 0) {
      throw new Error(`${msg}\n\n${detail}`);
    }
    throw new Error(`${msg}\nrun with -v for full git output`);
  }
}

export function runGitPlan(
  cmds: Cmd[],
  verbose = false,
): void {
  for (const cmd of cmds) {
    runCmd(
      cmd,
      verbose,
    );
  }
}

/**
 * 同步多个 repo 时保留整体进度, 因为单仓失败不应该阻断其它仓库更新。
 */
export function runGitPlanKeepGoing(
  cmds: Cmd[],
  verbose = false,
  run: (cmd: Cmd, verbose: boolean) => void = runCmd,
  onFailure: (err: Error) => void = (err) => console.error(err.message),
): number {
  let failures = 0;

  for (const cmd of cmds) {
    try {
      run(
        cmd,
        verbose,
      );
    } catch (err) {
      failures += 1;
      const msg = err instanceof Error ? err : new Error(String(err));
      onFailure(msg);
    }
  }

  return failures;
}
