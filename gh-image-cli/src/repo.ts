import { existsSync } from "node:fs";
import { join } from "node:path";
import { mustRun } from "./process";

export type RepoContext = {
  root: string;
  imagesPath: string;
  repository: string;
  defaultBranch: string;
};

export type LocalRepoContext = Pick<RepoContext, "root" | "imagesPath">;

/**
 * 以 Git 根目录为项目边界, 保证从任意子目录执行时读取同一份镜像台账.
 */
export function resolveLocalRepo(requireImages = true): LocalRepoContext {
  const root = mustRun("git", ["rev-parse", "--show-toplevel"]);
  const imagesPath = join(root, "images.yaml");
  if (requireImages && !existsSync(imagesPath)) {
    throw new Error(`images.yaml not found at repository root: ${imagesPath}`);
  }

  return { root, imagesPath };
}

/**
 * 只有触发 Action 时才访问 GitHub, list/add/completion 保持纯本地操作.
 */
export function resolveRepo(requireImages = true): RepoContext {
  const local = resolveLocalRepo(requireImages);
  const json = mustRun("gh", [
    "repo",
    "view",
    "--json",
    "nameWithOwner,defaultBranchRef",
  ], local.root);
  const info = JSON.parse(json) as {
    nameWithOwner: string;
    defaultBranchRef: { name: string };
  };
  return {
    ...local,
    repository: info.nameWithOwner,
    defaultBranch: info.defaultBranchRef.name,
  };
}
