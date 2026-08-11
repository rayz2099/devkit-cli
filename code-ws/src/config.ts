import { existsSync, readFileSync, realpathSync } from "node:fs";
import { dirname, join } from "node:path";
import type {
  CodeWsCfg,
  ProfileCfg,
  RawCodeWsCfg,
  RawProfileCfg,
  RepoCfg,
} from "./types";

type RawObj = Record<string, unknown>;
type ProjectEntry = {
  name: string;
  group: string;
  branch?: string;
  description?: string;
};

type ResolveOpts = {
  home: string;
  moduleRoot: string;
};

function isObj(value: unknown): value is RawObj {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function reqStr(
  obj: RawObj,
  key: string,
): string {
  const val = obj[key];
  if (typeof val !== "string" || val.trim().length === 0) {
    throw new Error(`${key} must be a non-empty string`);
  }
  return val;
}

function parseRepo(value: unknown): string {
  if (typeof value !== "string" || value.trim().length === 0) {
    throw new Error("repo must be a non-empty string");
  }

  return value;
}

function yamlStr(value: string): string {
  return value.replace(/^"(.+)"$/, "$1");
}

/**
 * profile 只保存业务引用, 因为真实路径由 project.yml 统一维护。
 */
function parseProfile(value: unknown): RawProfileCfg {
  if (!isObj(value)) {
    throw new Error("profile must be an object");
  }

  const repos = value.repos;
  if (!Array.isArray(repos) || repos.length === 0) {
    throw new Error("profile.repos must be a non-empty array");
  }

  return {
    name: reqStr(value, "name"),
    agentsTemplate: reqStr(value, "agentsTemplate"),
    repos: repos.map(parseRepo),
  };
}

/**
 * 校验配置而不是信任 JSON, 因为这个 CLI 会批量执行 git 和文件写入。
 */
export function parseConfig(raw: unknown): RawCodeWsCfg {
  if (!isObj(raw)) {
    throw new Error("config must be an object");
  }

  const profiles = raw.profiles;
  if (!isObj(profiles)) {
    throw new Error("profiles must be an object");
  }

  const entries = Object.entries(profiles).map(([key, val]) => [
    key,
    parseProfile(val),
  ]);

  return {
    workspaceRoot: reqStr(raw, "workspaceRoot"),
    baseBranch: reqStr(raw, "baseBranch"),
    remote: reqStr(raw, "remote"),
    initAgentsTemplate: reqStr(raw, "initAgentsTemplate"),
    profiles: Object.fromEntries(entries),
  };
}

/**
 * 只展开显式 $HOME, 因为配置需要跨 macOS/Linux, 但不应该悄悄解释 shell 表达式。
 */
function expandHome(
  val: string,
  home: string,
): string {
  if (val === "$HOME") {
    return home;
  }

  if (val.startsWith("$HOME/")) {
    return join(home, val.slice("$HOME/".length));
  }

  return val;
}

/**
 * 解析当前项目约定的 project.yml 子集, 因为引入完整 YAML 依赖对这个固定 schema 没有收益。
 */
function parseProjectCatalog(txt: string): {
  home: string;
  projects: ProjectEntry[];
} {
  let home = "";
  const projects: ProjectEntry[] = [];
  let current: Partial<ProjectEntry> | undefined;

  for (const rawLine of txt.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (line.length === 0 || line.startsWith("#")) {
      continue;
    }

    const homeVal = line.match(/^home:\s*"?([^"]+)"?$/);
    if (homeVal?.[1] !== undefined) {
      home = yamlStr(homeVal[1]);
      continue;
    }

    const repoVal = line.match(/^-\s+repo-name:\s*"?([^"]+)"?$/);
    if (repoVal?.[1] !== undefined) {
      if (current?.name !== undefined && current.group !== undefined) {
        projects.push({
          name: current.name,
          group: current.group,
          branch: current.branch,
          description: current.description,
        });
      }
      current = {
        name: yamlStr(repoVal[1]),
      };
      continue;
    }

    const groupVal = line.match(/^group:\s*"?([^"]+)"?$/);
    if (groupVal?.[1] !== undefined && current !== undefined) {
      current.group = yamlStr(groupVal[1]);
    }

    const descVal = line.match(/^description:\s*(.+)$/);
    if (descVal?.[1] !== undefined && current !== undefined) {
      current.description = yamlStr(descVal[1]);
    }

    const branchVal = line.match(/^branch:\s*"?([^"]+)"?$/);
    if (branchVal?.[1] !== undefined && current !== undefined) {
      current.branch = yamlStr(branchVal[1]);
    }
  }

  if (current?.name !== undefined && current.group !== undefined) {
    projects.push({
      name: current.name,
      group: current.group,
      branch: current.branch,
      description: current.description,
    });
  }

  if (home.length === 0) {
    throw new Error("project.yml home must be a non-empty string");
  }

  return {
    home,
    projects,
  };
}

function repoPath(
  home: string,
  entry: ProjectEntry,
): string {
  if (entry.group === ".") {
    return join(home, entry.name);
  }

  return join(home, entry.group, entry.name);
}

function repoBranch(
  defaultBranch: string,
  entry: ProjectEntry,
): string {
  return entry.branch ?? defaultBranch;
}

function repoCfg(
  name: string,
  home: string,
  entry: ProjectEntry,
  defaultBranch: string,
): RepoCfg {
  const repo: RepoCfg = {
    name,
    path: repoPath(home, entry),
    group: entry.group,
    branch: repoBranch(
      defaultBranch,
      entry,
    ),
  };

  if (entry.description !== undefined) {
    repo.description = entry.description;
  }

  return repo;
}

/**
 * repo name 必须解析成唯一物理仓库, 因为 git worktree 不能接受歧义目录。
 */
function resolveRepo(
  name: string,
  home: string,
  projects: ProjectEntry[],
  defaultBranch: string,
): RepoCfg {
  const matched = projects.filter((project) => project.name === name);
  if (matched.length === 0) {
    throw new Error(`repo not found in project.yml: ${name}`);
  }

  if (matched.length > 1) {
    const groups = matched.map((project) => project.group).join(", ");
    throw new Error(`repo name is ambiguous in project.yml: ${name} (${groups})`);
  }

  const [project] = matched;
  if (project === undefined) {
    throw new Error(`repo not found in project.yml: ${name}`);
  }

  return repoCfg(
    name,
    home,
    project,
    defaultBranch,
  );
}

function resolveProfile(
  profile: RawProfileCfg,
  projects: ProjectEntry[],
  projectHome: string,
  defaultBranch: string,
  opts: ResolveOpts,
): ProfileCfg {
  const repos = profile.repos.map((repo) =>
    resolveRepo(
      repo,
      projectHome,
      projects,
      defaultBranch,
    ),
  );

  return {
    name: profile.name,
    agentsTemplate: join(
      opts.moduleRoot,
      "templates",
      "agents",
      profile.agentsTemplate,
    ),
    repos,
  };
}

/**
 * 把轻量配置解析成执行配置, 因为 config.json 只表达业务引用, 文件系统路径统一由 project.yml 和 HOME 派生。
 */
export function resolveConfig(
  cfg: RawCodeWsCfg,
  projectTxt: string,
  opts: ResolveOpts,
): CodeWsCfg {
  const catalog = parseProjectCatalog(projectTxt);
  const projectHome = expandHome(catalog.home, opts.home);
  const projects = catalog.projects.map((project) =>
    repoCfg(
      project.name,
      projectHome,
      project,
      cfg.baseBranch,
    ),
  );
  const profiles = Object.entries(cfg.profiles).map(([key, profile]) => [
    key,
    resolveProfile(
      profile,
      catalog.projects,
      projectHome,
      cfg.baseBranch,
      opts,
    ),
  ]);

  return {
    workspaceRoot: expandHome(cfg.workspaceRoot, opts.home),
    baseBranch: cfg.baseBranch,
    remote: cfg.remote,
    initAgentsTemplate: join(
      opts.moduleRoot,
      "templates",
      "agents",
      cfg.initAgentsTemplate,
    ),
    projects,
    profiles: Object.fromEntries(profiles),
  };
}

/**
 * HOME 是路径配置的根输入, 缺失时直接失败比生成相对路径更安全。
 */
function reqHome(): string {
  const home = process.env.HOME;
  if (home === undefined || home.length === 0) {
    throw new Error("HOME must be set");
  }

  return home;
}

/**
 * 单独暴露文件读取, 让测试可以绕开磁盘专注校验规则。
 */
/**
 * XDG 配置与仓库 templates 解耦: 优先读 conf 目录下 module-root 标记.
 * 兼容旧布局 conf 位于 <module>/conf 时回退到 dirname(confDir).
 */
export function resolveModuleRoot(confDir: string): string {
  const marker = join(confDir, "module-root");
  if (existsSync(marker)) {
    const value = readFileSync(marker, "utf8").trim();
    if (value.length === 0) {
      throw new Error(`module-root is empty: ${marker}`);
    }
    return value;
  }

  return dirname(confDir);
}

export function loadConfig(path: string): CodeWsCfg {
  if (!existsSync(path)) {
    throw new Error(`config not found: ${path}`);
  }

  const txt = readFileSync(path, "utf8");
  const raw = parseConfig(JSON.parse(txt));
  const configFile = realpathSync(path);
  const confDir = dirname(configFile);
  const projectFile = join(confDir, "project.yml");
  if (!existsSync(projectFile)) {
    throw new Error(`project config not found: ${projectFile}`);
  }

  return resolveConfig(
    raw,
    readFileSync(projectFile, "utf8"),
    {
      home: reqHome(),
      moduleRoot: resolveModuleRoot(confDir),
    },
  );
}
