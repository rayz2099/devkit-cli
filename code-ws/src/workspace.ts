import {
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  writeFileSync,
} from "node:fs";
import { basename, dirname, join } from "node:path";
import type { RepoCfg } from "./types";

export type VsCodeWs = {
  folders: {
    name: string;
    path: string;
  }[];
  settings: Record<string, never>;
};

/**
 * 只取分支末段, 因为 feature/spec101 这类 Git 命名不适合作为目录层级。
 */
export function workspaceNameFromBranch(branch: string): string {
  const parts = branch.split("/").filter((part) => part.length > 0);
  const last = parts.at(-1);
  if (last === undefined || last.length === 0) {
    throw new Error("branch must be a non-empty string");
  }

  return `${last}-workspace`;
}

/**
 * 使用相对路径, 让整个 workspace 目录移动后 VS Code 配置仍然有效。
 */
export function createWorkspaceJson(repoNames: string[]): VsCodeWs {
  return {
    folders: repoNames.map((name) => ({
      name,
      path: name,
    })),
    settings: {},
  };
}

export function workspaceFileName(branch: string): string {
  const name = workspaceNameFromBranch(branch);
  return `${name.replace(/-workspace$/, "")}.code-workspace`;
}

export function writeWorkspaceFile(
  wsDir: string,
  branch: string,
  repoNames: string[],
): string {
  mkdirSync(wsDir, {
    recursive: true,
  });

  const file = join(wsDir, workspaceFileName(branch));
  const json = createWorkspaceJson(repoNames);
  const txt = `${JSON.stringify(json, null, 2)}\n`;
  writeFileSync(file, txt, {
    flag: "wx",
  });

  return basename(file);
}

/**
 * workspace 内的 project.yml 给 agent 补项目语义, 避免只靠目录树推断上下文。
 */
export function createWorkspaceProjectYaml(
  branch: string,
  repos: RepoCfg[],
  topLines: string[] = [],
): string {
  const lines = [
    `branch: ${branch}`,
    ...topLines,
    "projects:",
  ];

  for (const repo of repos) {
    lines.push(`  - repo-name: ${repo.name}`);
    lines.push(`    group: ${repo.group}`);
    if (repo.description !== undefined && repo.description.length > 0) {
      lines.push(`    description: ${repo.description}`);
    }
    if (repo.checkoutBranch !== undefined && repo.checkoutBranch.length > 0) {
      lines.push(`    branch: ${repo.checkoutBranch}`);
    }
  }

  return `${lines.join("\n")}\n`;
}

/**
 * 保留用户写在顶层的业务变量, 因为 add project 只应该同步项目集合而不是清空 workspace 元数据。
 */
function readCustomProjectTopLines(txt: string): string[] {
  const lines: string[] = [];

  for (const rawLine of txt.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (line.length === 0) {
      continue;
    }

    if (line === "projects:") {
      break;
    }

    if (line.startsWith("branch:")) {
      continue;
    }

    if (rawLine.startsWith(" ") || rawLine.startsWith("\t")) {
      continue;
    }

    lines.push(rawLine);
  }

  return lines;
}

/**
 * 每次重写完整描述文件, 因为它是当前 workspace 项目集合的派生结果。
 */
export function writeWorkspaceProjectFile(
  wsDir: string,
  branch: string,
  repos: RepoCfg[],
): string {
  mkdirSync(wsDir, {
    recursive: true,
  });

  const file = join(wsDir, "project.yml");
  const topLines = existsSync(file)
    ? readCustomProjectTopLines(readFileSync(file, "utf8"))
    : [];
  writeFileSync(
    file,
    createWorkspaceProjectYaml(
      branch,
      repos,
      topLines,
    ),
  );
  return basename(file);
}

export type WsProject = {
  branch: string;
  repos: string[];
  checkoutBranches: Record<string, string>;
};

/**
 * 读取 workspace 固定分支和 repo 覆盖, 因为 add/fork/sync 必须复用已生成的选择。
 */
export function parseWorkspaceProject(txt: string): WsProject {
  let branch = "";
  const repos: string[] = [];
  const checkoutBranches: Record<string, string> = {};
  let currentRepo = "";

  for (const rawLine of txt.split(/\r?\n/)) {
    const line = rawLine.trim();
    const topBranchVal = rawLine.match(/^branch:\s*(.+)$/);
    if (topBranchVal?.[1] !== undefined) {
      branch = topBranchVal[1];
      continue;
    }

    const repoBranchVal = line.match(/^branch:\s*(.+)$/);
    if (repoBranchVal?.[1] !== undefined && currentRepo.length > 0) {
      checkoutBranches[currentRepo] = repoBranchVal[1];
      continue;
    }

    const legacyCheckoutBranchVal = line.match(/^checkout-branch:\s*(.+)$/);
    if (
      legacyCheckoutBranchVal?.[1] !== undefined &&
      currentRepo.length > 0
    ) {
      checkoutBranches[currentRepo] = legacyCheckoutBranchVal[1];
      continue;
    }

    const repoVal = line.match(/^-\s+repo-name:\s*(.+)$/);
    if (repoVal?.[1] !== undefined) {
      currentRepo = repoVal[1];
      repos.push(currentRepo);
      continue;
    }

  }

  if (branch.length === 0) {
    throw new Error("workspace project.yml branch must be a non-empty string");
  }

  return {
    branch,
    repos,
    checkoutBranches,
  };
}

/**
 * 允许在 workspace 子目录里执行 add project, 让命令使用方式贴近 git 子目录操作。
 */
export function findWorkspaceDir(start: string): string {
  let cur = start;
  while (true) {
    if (existsSync(join(cur, "project.yml"))) {
      return cur;
    }

    const parent = dirname(cur);
    if (parent === cur) {
      throw new Error(`workspace project.yml not found from: ${start}`);
    }
    cur = parent;
  }
}

function readWorkspaceFilePath(wsDir: string): string {
  const matched = readdirSync(wsDir)
    .filter((name) => name.endsWith(".code-workspace"));

  if (matched.length !== 1) {
    throw new Error(`workspace file must be unique in: ${wsDir}`);
  }

  const name = matched[0];
  if (name === undefined) {
    throw new Error(`workspace file not found in: ${wsDir}`);
  }

  return join(wsDir, name);
}

/**
 * VS Code workspace 要同步追加 folder, 否则新增 worktree 不会出现在编辑器项目组。
 */
export function addRepoToWorkspaceJson(
  wsDir: string,
  repoName: string,
): string {
  const file = readWorkspaceFilePath(wsDir);
  const json = JSON.parse(readFileSync(file, "utf8")) as VsCodeWs;
  const exists = json.folders.some((folder) => folder.name === repoName);
  if (exists) {
    throw new Error(`repo already exists in workspace: ${repoName}`);
  }

  json.folders.push({
    name: repoName,
    path: repoName,
  });
  writeFileSync(
    file,
    `${JSON.stringify(json, null, 2)}\n`,
  );
  return basename(file);
}

/**
 * 先在内存中变更 JSON, 因为删除 repo 时测试不需要依赖真实 workspace 文件。
 */
export function removeRepoFromWorkspaceJson(
  json: VsCodeWs,
  repoName: string,
): VsCodeWs {
  const folders = json.folders.filter((folder) => folder.name !== repoName);
  if (folders.length === json.folders.length) {
    throw new Error(`repo not found in workspace: ${repoName}`);
  }

  return {
    ...json,
    folders,
  };
}

/**
 * VS Code workspace 需要同步删除 folder, 否则编辑器仍会打开已移除的 worktree 路径。
 */
export function removeRepoFromWorkspaceJsonFile(
  wsDir: string,
  repoName: string,
): string {
  const file = readWorkspaceFilePath(wsDir);
  const json = JSON.parse(readFileSync(file, "utf8")) as VsCodeWs;
  writeFileSync(
    file,
    `${JSON.stringify(removeRepoFromWorkspaceJson(json, repoName), null, 2)}\n`,
  );
  return basename(file);
}
