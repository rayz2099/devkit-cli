#!/usr/bin/env bun

import { existsSync, mkdirSync, readFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { copyAgentsTemplate, copyForkWorkspaceEntries } from "./agents";
import { renderFishCompletion } from "./completion";
import { loadConfig } from "./config";
import type { Cmd } from "./types";
import {
  assertCleanWorktrees,
  assertDisposableWorktrees,
  assertGitTargets,
  buildForkGitPlan,
  buildGitPlan,
  buildSyncBaseBranchPlan,
  buildWorktreeRemovePlan,
  confirmDeletePrompt,
  confirmRemoveProjectPrompt,
  prepareGitTargets,
  runGitPlan,
  runGitPlanKeepGoing,
  shouldConfirmRemoveProject,
  shouldDeleteExistingDir,
} from "./git";
import {
  addRepoToWorkspaceJson,
  findWorkspaceDir,
  parseWorkspaceProject,
  removeRepoFromWorkspaceJsonFile,
  type WsProject,
  workspaceNameFromBranch,
  writeWorkspaceFile,
  writeWorkspaceProjectFile,
} from "./workspace";
import type { CodeWsCfg, ProfileCfg, RepoCfg } from "./types";

type InitProfileArgs = {
  cmd: "init";
  branch: string;
  profile: string;
  config?: string;
  verbose: boolean;
};

type InitProjectArgs = {
  cmd: "init";
  branch: string;
  project: string;
  config?: string;
  verbose: boolean;
};

type InitArgs = InitProfileArgs | InitProjectArgs;

type ListArgs = {
  cmd: "list";
  config?: string;
};

type ProjectsArgs = {
  cmd: "projects";
  config?: string;
};

type CfgCheckArgs = {
  cmd: "config-check";
  config?: string;
};

type CompletionFishArgs = {
  cmd: "completion-fish";
};

type AddProjectArgs = {
  cmd: "add-project";
  repo: string;
  branch?: string;
  config?: string;
  verbose: boolean;
};

type RemoveProjectArgs = {
  cmd: "remove-project";
  repo: string;
  config?: string;
  verbose: boolean;
};

type SyncMasterArgs = {
  cmd: "sync-master";
  config?: string;
  verbose: boolean;
};

type DestroyWorkspaceArgs = {
  cmd: "destroy-workspace";
  config?: string;
  verbose: boolean;
};

type ForkWorkspaceArgs = {
  cmd: "fork-workspace";
  branch: string;
  config?: string;
  verbose: boolean;
};

type HelpArgs = {
  cmd: "help";
};

type CliArgs =
  | InitArgs
  | ListArgs
  | ProjectsArgs
  | CfgCheckArgs
  | CompletionFishArgs
  | AddProjectArgs
  | RemoveProjectArgs
  | SyncMasterArgs
  | DestroyWorkspaceArgs
  | ForkWorkspaceArgs
  | HelpArgs;

/**
 * Bun compile 会把 import.meta.url 指到 /$bunfs, 所以二进制运行时要改用 execPath。
 */
export function resolveDefaultConfig(
  metaUrl: string,
  execPath: string,
  env: NodeJS.ProcessEnv = process.env,
): string {
  const xdg = env.XDG_CONFIG_HOME;
  if (xdg !== undefined && xdg.length > 0) {
    return join(xdg, "code-ws", "config.json");
  }

  const home = env.HOME;
  if (home === undefined || home.length === 0) {
    throw new Error("HOME must be set");
  }

  return join(home, ".config", "code-ws", "config.json");
}

const defaultCfg = resolveDefaultConfig(import.meta.url, process.execPath);

function valueAfter(
  args: string[],
  idx: number,
  flag: string,
): string {
  const val = args[idx + 1];
  if (val === undefined || val.startsWith("-")) {
    throw new Error(`${flag} requires a value`);
  }
  return val;
}

function parseOpts(args: string[]): {
  rest: string[];
  profile?: string;
  branch?: string;
  config?: string;
  help: boolean;
  verbose: boolean;
} {
  const rest: string[] = [];
  let profile: string | undefined;
  let branch: string | undefined;
  let config: string | undefined;
  let help = false;
  let verbose = false;

  for (let i = 0; i < args.length; i += 1) {
    const arg = args[i];
    if (arg === "-t" || arg === "--template") {
      profile = valueAfter(args, i, arg);
      i += 1;
    } else if (arg === "-b" || arg === "--branch") {
      branch = valueAfter(args, i, arg);
      i += 1;
    } else if (arg === "--config") {
      config = valueAfter(args, i, arg);
      i += 1;
    } else if (arg === "-h" || arg === "--help") {
      help = true;
    } else if (arg === "-v" || arg === "--verbose") {
      verbose = true;
    } else if (arg !== undefined) {
      rest.push(arg);
    }
  }

  return {
    rest,
    profile,
    branch,
    config,
    help,
    verbose,
  };
}

/**
 * 参数解析保持无副作用, 因为 CLI 行为需要能独立测试。
 */
export function parseCliArgs(args: string[]): CliArgs {
  const parsed = parseOpts(args);
  const [cmd, sub, ...tail] = parsed.rest;

  if (parsed.help) {
    return {
      cmd: "help",
    };
  }

  if (cmd === "init") {
    if (sub === undefined || tail.length > 1) {
      throw new Error(
        "usage: code-ws init <branch> <project> or code-ws init <branch> -t <profile>",
      );
    }
    const [project] = tail;
    if (project !== undefined && parsed.profile !== undefined) {
      throw new Error("usage: code-ws init <branch> <project>");
    }
    if (project !== undefined) {
      return {
        cmd: "init",
        branch: sub,
        project,
        config: parsed.config,
        verbose: parsed.verbose,
      };
    }
    if (parsed.profile === undefined) {
      throw new Error("profile is required: -t <profile>");
    }
    return {
      cmd: "init",
      branch: sub,
      profile: parsed.profile,
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "projects") {
    if (sub !== undefined) {
      throw new Error("usage: code-ws projects");
    }
    return {
      cmd: "projects",
      config: parsed.config,
    };
  }

  if (cmd === "completion" && sub === "fish" && tail.length === 0) {
    return {
      cmd: "completion-fish",
    };
  }

  if (cmd === "list") {
    if (sub !== undefined) {
      throw new Error("usage: code-ws list");
    }
    return {
      cmd: "list",
      config: parsed.config,
    };
  }

  if (cmd === "add" && sub === "project") {
    const [repo, ...extra] = tail;
    if (repo === undefined || extra.length > 0) {
      throw new Error(
        "usage: code-ws add project <repo> [-b|--branch <branch>] [-v|--verbose] [--config <path>]",
      );
    }
    return {
      cmd: "add-project",
      repo,
      branch: parsed.branch,
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "remove" && sub === "project") {
    const [repo, ...extra] = tail;
    if (repo === undefined || extra.length > 0) {
      throw new Error(
        "usage: code-ws remove project <repo> [-v|--verbose] [--config <path>]",
      );
    }
    return {
      cmd: "remove-project",
      repo,
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "sync" && sub === "master") {
    if (tail.length > 0) {
      throw new Error(
        "usage: code-ws sync master [-v|--verbose] [--config <path>]",
      );
    }
    return {
      cmd: "sync-master",
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "destroy") {
    if (sub !== undefined) {
      throw new Error(
        "usage: code-ws destroy [-v|--verbose] [--config <path>]",
      );
    }
    return {
      cmd: "destroy-workspace",
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "fork") {
    if (sub === undefined || tail.length > 0) {
      throw new Error(
        "usage: code-ws fork <branch> [-v|--verbose] [--config <path>]",
      );
    }
    return {
      cmd: "fork-workspace",
      branch: sub,
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "config" && sub === "check" && tail.length === 0) {
    return {
      cmd: "config-check",
      config: parsed.config,
    };
  }

  if (cmd === undefined || cmd === "help" || cmd === "--help" || cmd === "-h") {
    return {
      cmd: "help",
    };
  }

  throw new Error(`unknown command: ${parsed.rest.join(" ")}`);
}

function cfgPath(
  args:
    | ListArgs
    | ProjectsArgs
    | CfgCheckArgs
    | InitArgs
    | AddProjectArgs
    | RemoveProjectArgs
    | SyncMasterArgs
    | DestroyWorkspaceArgs
    | ForkWorkspaceArgs,
): string {
  return args.config ?? defaultCfg;
}

function getProfile(
  cfg: CodeWsCfg,
  name: string,
): ProfileCfg {
  const profile = cfg.profiles[name];
  if (profile === undefined) {
    throw new Error(`profile not found: ${name}`);
  }
  return profile;
}

function checkProfile(profile: ProfileCfg): void {
  checkAgentsTemplate(profile.agentsTemplate);

  checkRepos(profile.repos);
}

function checkAgentsTemplate(path: string): void {
  if (!existsSync(path)) {
    throw new Error(`agents template not found: ${path}`);
  }
}

/**
 * 初始化 project 模式不绑定 agents template, 但仍要提前阻断无效 git 源路径。
 */
function checkRepos(repos: RepoCfg[]): void {
  for (const repo of repos) {
    if (!existsSync(repo.path)) {
      throw new Error(`repo not found: ${repo.path}`);
    }
  }
}

function listProfiles(cfg: CodeWsCfg): void {
  for (const [name, profile] of Object.entries(cfg.profiles)) {
    const repos = profile.repos.map((repo) => repo.name).join(", ");
    console.log(`${name}: ${repos}`);
  }
}

function listProjects(cfg: CodeWsCfg): void {
  for (const project of cfg.projects) {
    console.log(project.name);
  }
}

function getRepo(
  cfg: CodeWsCfg,
  name: string,
): RepoCfg {
  const repo = cfg.projects.find((project) => project.name === name);
  if (repo === undefined) {
    throw new Error(`repo not found in project.yml: ${name}`);
  }
  return repo;
}

function reposByName(
  cfg: CodeWsCfg,
  names: string[],
): RepoCfg[] {
  return names.map((name) =>
    getRepo(
      cfg,
      name,
    ),
  );
}

function repoWithCheckoutBranch(
  repo: RepoCfg,
  branch: string | undefined,
): RepoCfg {
  if (branch === undefined) {
    return repo;
  }

  return {
    ...repo,
    checkoutBranch: branch,
  };
}

function reposByWorkspaceProject(
  cfg: CodeWsCfg,
  wsProject: WsProject,
): RepoCfg[] {
  return wsProject.repos.map((name) =>
    repoWithCheckoutBranch(
      getRepo(
        cfg,
        name,
      ),
      wsProject.checkoutBranches[name],
    ),
  );
}

function initWorkspace(args: InitArgs): void {
  const cfg = loadConfig(cfgPath(args));
  let profile: ProfileCfg | undefined;
  let agentsTemplate: string;
  let repos: RepoCfg[];
  if ("profile" in args) {
    profile = getProfile(cfg, args.profile);
    agentsTemplate = profile.agentsTemplate;
    repos = profile.repos;
  } else {
    agentsTemplate = cfg.initAgentsTemplate;
    repos = [
      getRepo(
        cfg,
        args.project,
      ),
    ];
  }
  const wsName = workspaceNameFromBranch(args.branch);
  const wsDir = join(cfg.workspaceRoot, wsName);

  if (profile !== undefined) {
    checkProfile(profile);
  } else {
    checkAgentsTemplate(agentsTemplate);
    checkRepos(repos);
  }
  prepareGitTargets(
    wsDir,
    repos,
    cfg.remote,
    cfg.baseBranch,
    args.verbose,
  );
  mkdirSync(wsDir, {
    recursive: true,
  });

  const gitPlan = buildGitPlan({
    branch: args.branch,
    baseBranch: cfg.baseBranch,
    remote: cfg.remote,
    workspaceDir: wsDir,
    repos,
  });

  runGitPlan(
    gitPlan,
    args.verbose,
  );
  copyAgentsTemplate(agentsTemplate, wsDir);
  const wsFile = writeWorkspaceFile(
    wsDir,
    args.branch,
    repos.map((repo) => repo.name),
  );
  writeWorkspaceProjectFile(
    wsDir,
    args.branch,
    repos,
  );

  console.log(`workspace created: ${join(wsDir, wsFile)}`);
}

function addProject(args: AddProjectArgs): void {
  const cfg = loadConfig(cfgPath(args));
  const repo = repoWithCheckoutBranch(
    getRepo(
      cfg,
      args.repo,
    ),
    args.branch,
  );
  const wsDir = findWorkspaceDir(process.cwd());
  const wsProject = parseWorkspaceProject(
    readFileSync(
      join(wsDir, "project.yml"),
      "utf8",
    ),
  );
  if (wsProject.repos.includes(repo.name)) {
    throw new Error(
      `repo already exists in workspace project.yml: ${repo.name}`,
    );
  }

  mkdirSync(wsDir, {
    recursive: true,
  });
  runGitPlan(
    buildGitPlan({
      branch: wsProject.branch,
      baseBranch: cfg.baseBranch,
      remote: cfg.remote,
      workspaceDir: wsDir,
      repos: [
        repo,
      ],
    }),
    args.verbose,
  );

  addRepoToWorkspaceJson(
    wsDir,
    repo.name,
  );
  writeWorkspaceProjectFile(
    wsDir,
    wsProject.branch,
    [
      ...reposByWorkspaceProject(
        cfg,
        wsProject,
      ),
      repo,
    ],
  );

  console.log(`project added: ${join(wsDir, repo.name)}`);
}

/**
 * 先确认再删除 worktree, 因为 CLI 无法替用户判断分支是否已完成合并。
 */
function removeProject(args: RemoveProjectArgs): void {
  const cfg = loadConfig(cfgPath(args));
  const repo = getRepo(
    cfg,
    args.repo,
  );
  const wsDir = findWorkspaceDir(process.cwd());
  const wsProject = parseWorkspaceProject(
    readFileSync(
      join(wsDir, "project.yml"),
      "utf8",
    ),
  );
  if (!wsProject.repos.includes(repo.name)) {
    throw new Error(
      `repo not found in workspace project.yml: ${repo.name}`,
    );
  }

  const worktreePath = join(wsDir, repo.name);
  const answer = prompt(confirmRemoveProjectPrompt(
    repo.name,
    worktreePath,
    repo.branch,
  ));
  if (!shouldConfirmRemoveProject(answer)) {
    throw new Error(`project remove aborted: ${repo.name}`);
  }

  runGitPlan(
    buildWorktreeRemovePlan(
      wsDir,
      [
        repo,
      ],
    ),
    args.verbose,
  );
  removeRepoFromWorkspaceJsonFile(
    wsDir,
    repo.name,
  );
  const nextRepos = wsProject.repos.filter((name) => name !== repo.name);
  writeWorkspaceProjectFile(
    wsDir,
    wsProject.branch,
    reposByName(
      cfg,
      nextRepos,
    ),
  );

  console.log(`project removed: ${worktreePath}`);
}

/**
 * 只更新本地主分支引用, 因为当前分支合并动作由用户在各 repo 手动执行。
 */
function syncMaster(args: SyncMasterArgs): void {
  const cfg = loadConfig(cfgPath(args));
  const wsDir = findWorkspaceDir(process.cwd());
  const wsProject = parseWorkspaceProject(
    readFileSync(
      join(wsDir, "project.yml"),
      "utf8",
    ),
  );
  const repos = reposByWorkspaceProject(
    cfg,
    wsProject,
  );

  const failures = runGitPlanKeepGoing(
    buildSyncBaseBranchPlan({
      baseBranch: cfg.baseBranch,
      remote: cfg.remote,
      repos,
    }),
    args.verbose,
  );

  if (failures > 0) {
    throw new Error(`base branch sync failed: ${failures} repo(s)`);
  }

  console.log(`base branches synced: ${wsDir}`);
}

type DestroyWorkspaceDeps = {
  assertDisposable?: (
    wsDir: string,
    repos: RepoCfg[],
    remote: string,
    baseBranch: string,
  ) => void;
  runPlan?: (plan: Cmd[], verbose: boolean) => void;
  log?: (msg: string) => void;
};

type ForkWorkspaceDeps = {
  dstExists?: (path: string) => boolean;
  assertSrcClean?: (wsDir: string, repos: RepoCfg[]) => void;
  assertDstDisposable?: (
    wsDir: string,
    repos: RepoCfg[],
    remote: string,
    baseBranch: string,
  ) => void;
  assertDstTargets?: (wsDir: string, repos: RepoCfg[]) => void;
  readAnswer?: (msg: string) => string | null;
  removeDir?: (path: string) => void;
  mkdir?: (path: string) => void;
  runPlan?: (plan: Cmd[], verbose: boolean) => void;
  copyEntries?: (srcWsDir: string, dstWsDir: string) => void;
  writeWsFile?: (
    wsDir: string,
    branch: string,
    repos: string[],
  ) => string;
  writeWsProject?: (wsDir: string, branch: string, repos: RepoCfg[]) => void;
  log?: (msg: string) => void;
};

function removeWsDir(path: string): void {
  rmSync(path, {
    recursive: true,
    force: true,
  });
}

/**
 * 只卸载 git worktree, 因为 workspace 文档与元数据要保留给后续淬炼复用.
 * 卸载前拦截未提交改动和未合入主分支的本地 commit, 避免丢代码.
 */
export function destroyWorkspace(
  args: DestroyWorkspaceArgs,
  cfg: CodeWsCfg,
  wsDir: string,
  wsProject: WsProject,
  deps: DestroyWorkspaceDeps = {},
): void {
  const assertDisposable = deps.assertDisposable ?? assertDisposableWorktrees;
  const runPlan = deps.runPlan ?? runGitPlan;
  const log = deps.log ?? console.log;
  const repos = reposByWorkspaceProject(
    cfg,
    wsProject,
  );

  assertDisposable(
    wsDir,
    repos,
    cfg.remote,
    cfg.baseBranch,
  );
  runPlan(
    buildWorktreeRemovePlan(
      wsDir,
      repos,
    ),
    args.verbose,
  );

  log(`workspace worktrees cleared: ${wsDir}`);
  log(`workspace directory kept: ${wsDir}`);
}

function destroyCurrentWorkspace(args: DestroyWorkspaceArgs): void {
  const cfg = loadConfig(cfgPath(args));
  const wsDir = findWorkspaceDir(process.cwd());
  const wsProject = parseWorkspaceProject(
    readFileSync(
      join(wsDir, "project.yml"),
      "utf8",
    ),
  );

  destroyWorkspace(
    args,
    cfg,
    wsDir,
    wsProject,
  );
}

/**
 * 从当前 workspace 派生新 workspace, 因为同一任务可能需要切到后续分支继续工作。
 */
export function forkWorkspace(
  args: ForkWorkspaceArgs,
  cfg: CodeWsCfg,
  srcWsDir: string,
  wsProject: WsProject,
  deps: ForkWorkspaceDeps = {},
): void {
  const dstExists = deps.dstExists ?? existsSync;
  const assertSrcClean = deps.assertSrcClean ?? assertCleanWorktrees;
  const assertDstDisposable = deps.assertDstDisposable ??
    assertDisposableWorktrees;
  const assertDstTargets = deps.assertDstTargets ?? assertGitTargets;
  const readAnswer = deps.readAnswer ?? prompt;
  const removeDir = deps.removeDir ?? removeWsDir;
  const mkdir = deps.mkdir ?? ((path: string) => {
    mkdirSync(path, {
      recursive: true,
    });
  });
  const runPlan = deps.runPlan ?? runGitPlan;
  const copyEntries = deps.copyEntries ?? copyForkWorkspaceEntries;
  const writeWsFile = deps.writeWsFile ?? writeWorkspaceFile;
  const writeWsProject = deps.writeWsProject ?? writeWorkspaceProjectFile;
  const log = deps.log ?? console.log;
  const repos = reposByWorkspaceProject(
    cfg,
    wsProject,
  );
  const dstWsDir = join(
    cfg.workspaceRoot,
    workspaceNameFromBranch(args.branch),
  );

  assertSrcClean(
    srcWsDir,
    repos,
  );

  if (dstExists(dstWsDir)) {
    const answer = readAnswer(confirmDeletePrompt(dstWsDir));
    if (!shouldDeleteExistingDir(answer)) {
      throw new Error(`workspace fork aborted: ${dstWsDir}`);
    }
    assertDstDisposable(
      dstWsDir,
      repos,
      cfg.remote,
      cfg.baseBranch,
    );
    runPlan(
      buildWorktreeRemovePlan(
        dstWsDir,
        repos,
      ),
      args.verbose,
    );
    removeDir(dstWsDir);
  }

  assertDstTargets(
    dstWsDir,
    repos,
  );
  const gitPlan = buildForkGitPlan({
    srcWorkspaceDir: srcWsDir,
    dstWorkspaceDir: dstWsDir,
    dstBranch: args.branch,
    repos,
  });
  mkdir(dstWsDir);
  runPlan(
    gitPlan,
    args.verbose,
  );
  copyEntries(
    srcWsDir,
    dstWsDir,
  );
  const wsFile = writeWsFile(
    dstWsDir,
    args.branch,
    repos.map((repo) => repo.name),
  );
  writeWsProject(
    dstWsDir,
    args.branch,
    repos,
  );

  log(`workspace forked: ${join(dstWsDir, wsFile)}`);
}

/**
 * 从磁盘加载当前 workspace 元数据, 因为 CLI 入口只负责装配运行上下文。
 */
function forkCurrentWorkspace(args: ForkWorkspaceArgs): void {
  const cfg = loadConfig(cfgPath(args));
  const srcWsDir = findWorkspaceDir(process.cwd());
  const wsProject = parseWorkspaceProject(
    readFileSync(
      join(srcWsDir, "project.yml"),
      "utf8",
    ),
  );

  forkWorkspace(
    args,
    cfg,
    srcWsDir,
    wsProject,
  );
}

function checkConfig(args: CfgCheckArgs): void {
  const cfg = loadConfig(cfgPath(args));
  for (const profile of Object.values(cfg.profiles)) {
    checkProfile(profile);
  }
  console.log("config ok");
}

function printHelp(): void {
  console.log(`code-ws

Usage:
  code-ws init <branch> -t <profile> [-v|--verbose] [--config <path>]
  code-ws init <branch> <project> [-v|--verbose] [--config <path>]
  code-ws add project <repo> [-b|--branch <branch>] [-v|--verbose] [--config <path>]
  code-ws remove project <repo> [-v|--verbose] [--config <path>]
  code-ws sync master [-v|--verbose] [--config <path>]
  code-ws destroy [-v|--verbose] [--config <path>]
  code-ws fork <branch> [-v|--verbose] [--config <path>]
  code-ws list [--config <path>]
  code-ws projects [--config <path>]
  code-ws config check [--config <path>]
  code-ws completion fish
`);
}

export function main(args: string[]): void {
  const parsed = parseCliArgs(args);
  if (parsed.cmd === "help") {
    printHelp();
    return;
  }

  if (parsed.cmd === "completion-fish") {
    console.log(renderFishCompletion());
    return;
  }

  if (parsed.cmd === "init") {
    initWorkspace(parsed);
    return;
  }

  if (parsed.cmd === "list") {
    listProfiles(loadConfig(cfgPath(parsed)));
    return;
  }

  if (parsed.cmd === "projects") {
    listProjects(loadConfig(cfgPath(parsed)));
    return;
  }

  if (parsed.cmd === "add-project") {
    addProject(parsed);
    return;
  }

  if (parsed.cmd === "remove-project") {
    removeProject(parsed);
    return;
  }

  if (parsed.cmd === "sync-master") {
    syncMaster(parsed);
    return;
  }

  if (parsed.cmd === "destroy-workspace") {
    destroyCurrentWorkspace(parsed);
    return;
  }

  if (parsed.cmd === "fork-workspace") {
    forkCurrentWorkspace(parsed);
    return;
  }

  checkConfig(parsed);
}

if (import.meta.main) {
  try {
    main(Bun.argv.slice(2));
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error(`code-ws: ${msg}`);
    process.exit(1);
  }
}
