import { mkdir } from "node:fs/promises";
import { join } from "node:path";
import type { Repo, Runtime } from "./types";

export type CachedRepo = {
  id: string;
  name: string;
  pathNs: string;
};

export type RepoIndex = {
  profile: string;
  orgId: string;
  updatedAt: string;
  repos: CachedRepo[];
};

/** 为什么: 人要在配置目录看见 Init 产物, 不能藏到 XDG cache. */
export function indexPath(profile: string, home = process.env.HOME): string {
  if (home === undefined || home === "") {
    throw new Error("HOME is required");
  }
  if (profile.trim() === "") {
    throw new Error("profile is required for repo index");
  }
  return join(home, ".config", "codeup-cli", `repos-${profile}.json`);
}

export async function loadIndex(
  profile: string,
  orgId: string,
  home = process.env.HOME,
): Promise<RepoIndex> {
  const path = indexPath(profile, home);
  const file = Bun.file(path);
  if (!(await file.exists())) {
    throw new Error(`repo index not found: ${path}; run codeup-cli init`);
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(await file.text());
  } catch {
    throw new Error(`invalid repo index: ${path}; run codeup-cli init`);
  }
  const idx = parseIndex(parsed, path);
  if (idx.orgId !== orgId) {
    throw new Error(`repo index org mismatch; run codeup-cli init`);
  }
  return idx;
}

export async function saveIndex(
  runtime: Runtime,
  repos: Repo[],
  home = process.env.HOME,
): Promise<RepoIndex> {
  const idx: RepoIndex = {
    profile: runtime.profile.name,
    orgId: runtime.orgId,
    updatedAt: new Date().toISOString(),
    repos: repos.map((repo) => ({
      id: repo.id,
      name: repo.name,
      pathNs: shortPath(repo.pathNs === "" ? repo.path : repo.pathNs, runtime.orgId),
    })),
  };
  const path = indexPath(runtime.profile.name, home);
  await mkdir(join(path, ".."), { recursive: true });
  await Bun.write(path, `${JSON.stringify(idx, null, 2)}\n`);
  return idx;
}

/** 为什么: 短名只查 Index, 禁止再打一次 OpenAPI 当模糊搜索. */
export function pickCachedRepo(query: string, repos: CachedRepo[]): CachedRepo {
  const q = query.trim().toLowerCase();
  if (q === "") {
    throw new Error("repository is required");
  }
  let best = -1;
  const hits: CachedRepo[] = [];
  for (const repo of repos) {
    const score = scoreRepo(q, repo);
    if (score < 0) {
      continue;
    }
    if (score > best) {
      best = score;
      hits.length = 0;
      hits.push(repo);
      continue;
    }
    if (score === best) {
      hits.push(repo);
    }
  }
  if (hits.length === 0) {
    throw new Error(`no repository matched ${query}; run codeup-cli init`);
  }
  if (hits.length > 1) {
    const names = hits.map((repo) => repo.pathNs).join(", ");
    throw new Error(`ambiguous repository ${query}: ${names}`);
  }
  const hit = hits[0];
  if (hit === undefined) {
    throw new Error(`no repository matched ${query}; run codeup-cli init`);
  }
  return hit;
}

export function scoreRepo(query: string, repo: CachedRepo): number {
  const name = repo.name.toLowerCase();
  const pathNs = repo.pathNs.toLowerCase();
  if (name === query || pathNs === query) {
    return 100;
  }
  if (pathNs.endsWith(`/${query}`)) {
    return 80;
  }
  if (name.startsWith(query) || pathNs.startsWith(query)) {
    return 60;
  }
  if (name.includes(query) || pathNs.includes(query)) {
    return 40;
  }
  return -1;
}

export function hintRepos(repos: CachedRepo[]): string[] {
  const out: string[] = [];
  for (const repo of repos) {
    out.push(repo.pathNs);
    if (repo.name !== repo.pathNs) {
      out.push(repo.name);
    }
  }
  return [...new Set(out)];
}

function parseIndex(raw: unknown, path: string): RepoIndex {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    throw new Error(`invalid repo index: ${path}; run codeup-cli init`);
  }
  const rec = raw as Record<string, unknown>;
  if (typeof rec.profile !== "string" || rec.profile.trim() === "") {
    throw new Error(`invalid repo index: ${path}; run codeup-cli init`);
  }
  if (typeof rec.orgId !== "string" || rec.orgId.trim() === "") {
    throw new Error(`invalid repo index: ${path}; run codeup-cli init`);
  }
  if (typeof rec.updatedAt !== "string" || rec.updatedAt.trim() === "") {
    throw new Error(`invalid repo index: ${path}; run codeup-cli init`);
  }
  if (!Array.isArray(rec.repos)) {
    throw new Error(`invalid repo index: ${path}; run codeup-cli init`);
  }
  return {
    profile: rec.profile,
    orgId: rec.orgId,
    updatedAt: rec.updatedAt,
    repos: rec.repos.map((item, index) => parseCached(item, path, index)),
  };
}

function parseCached(item: unknown, path: string, index: number): CachedRepo {
  if (!item || typeof item !== "object" || Array.isArray(item)) {
    throw new Error(`invalid repo index: ${path} repos[${index}]`);
  }
  const rec = item as Record<string, unknown>;
  const id = typeof rec.id === "string" ? rec.id : "";
  const name = typeof rec.name === "string" ? rec.name : "";
  const pathNs = typeof rec.pathNs === "string" ? rec.pathNs : "";
  if (id === "" || name === "" || pathNs === "") {
    throw new Error(`invalid repo index: ${path} repos[${index}]`);
  }
  return { id, name, pathNs };
}

function shortPath(raw: string, orgId: string): string {
  const parts = raw.split("/").filter((seg) => seg !== "");
  if (parts[0] === orgId) {
    parts.shift();
  }
  if (parts.length === 0) {
    throw new Error(`indexed repository missing path: ${raw}`);
  }
  return parts.join("/");
}
