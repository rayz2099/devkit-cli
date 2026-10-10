import type { RepoCfg } from "./types";

/**
 * `--from` 用仓库名定位合并目标，重复同一个仓库会让 agent 读到不确定的合入分支。
 */
export function parseFromSpecs(raw: string[]): Record<string, string> {
  const specs: Record<string, string> = {};

  for (const item of raw) {
    const eq = item.indexOf("=");
    const repo = eq > 0 ? item.slice(0, eq) : "";
    const ref = eq > 0 ? item.slice(eq + 1) : "";
    if (repo.length === 0 || ref.length === 0) {
      throw new Error(`--from requires repo=branch: ${item}`);
    }
    if (specs[repo] !== undefined) {
      throw new Error(`--from repeated for repo: ${repo}`);
    }
    specs[repo] = ref;
  }

  return specs;
}

/**
 * 每个仓库都带上 merge，agent 才能从 project.yml 直接读到合入分支。
 */
export function applyMergeSpecs(
  repos: RepoCfg[],
  specs: Record<string, string>,
  resolve: (repo: RepoCfg, ref: string) => string,
): RepoCfg[] {
  const names = new Set(repos.map((repo) => repo.name));
  for (const name of Object.keys(specs)) {
    if (!names.has(name)) {
      throw new Error(`--from repo is not selected: ${name}`);
    }
  }

  return repos.map((repo) => {
    const spec = specs[repo.name];
    const merge = spec === undefined
      ? (repo.merge ?? repo.branch)
      : resolve(repo, spec);
    return {
      ...repo,
      merge,
    };
  });
}
