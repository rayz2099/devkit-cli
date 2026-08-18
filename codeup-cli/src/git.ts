import { loadIndex, pickCachedRepo } from "./cache";
import { CODEUP_HOST, parseGitRemote, repoFromRemotePath } from "./org";
import { CodeupErr } from "./types";

/** 为什么: push 和 cwd repo 都走同一条 git 子进程, 避免两套解析. */
export async function runGit(args: string[], cwd = process.cwd()): Promise<string> {
  const proc = Bun.spawn(["git", ...args], {
    cwd,
    stdout: "pipe",
    stderr: "pipe",
  });
  const stdout = await new Response(proc.stdout).text();
  const stderr = await new Response(proc.stderr).text();
  const code = await proc.exited;
  if (code !== 0) {
    const detail = stderr.trim() === "" ? stdout.trim() : stderr.trim();
    throw new CodeupErr(detail === "" ? `git ${args.join(" ")} failed` : detail, 1);
  }
  return stdout.trim();
}

export async function resolveRepoFromCwd(orgId: string, cwd = process.cwd()): Promise<string> {
  const url = await runGit(["remote", "get-url", "origin"], cwd);
  if (url === "") {
    throw new CodeupErr("no git origin remote in cwd; pass --repo", 1);
  }
  const remote = parseGitRemote(url);
  if (remote.host !== CODEUP_HOST) {
    throw new CodeupErr(
      `origin host is not ${CODEUP_HOST} (${remote.host}); pass --repo`,
      1,
    );
  }
  return repoFromRemotePath(remote.path, orgId);
}

export async function currentBranch(cwd = process.cwd()): Promise<string> {
  const branch = await runGit(["rev-parse", "--abbrev-ref", "HEAD"], cwd);
  if (branch === "" || branch === "HEAD") {
    throw new CodeupErr("detached HEAD; pass an explicit branch to push", 1);
  }
  return branch;
}

export async function pushBranch(
  remote: string,
  branch: string,
  cwd = process.cwd(),
): Promise<void> {
  const url = await runGit(["remote", "get-url", remote], cwd);
  const parsed = parseGitRemote(url);
  if (parsed.host !== CODEUP_HOST) {
    throw new CodeupErr(
      `remote ${remote} is not ${CODEUP_HOST} (${parsed.host})`,
      1,
    );
  }
  await runGit(["push", remote, branch], cwd);
}

/**
 * 为什么: --repo / positional / cwd 的优先级是公开合同, 短名只走 Index.
 */
export async function resolveRepo(
  orgId: string,
  profile: string,
  flagValue: string | undefined,
  posValue: string | undefined,
  cwd = process.cwd(),
): Promise<string> {
  if (flagValue !== undefined && flagValue.trim() !== "") {
    return expandRepo(orgId, profile, flagValue.trim());
  }
  if (posValue !== undefined && posValue.trim() !== "") {
    return expandRepo(orgId, profile, posValue.trim());
  }
  return resolveRepoFromCwd(orgId, cwd);
}

export async function expandRepo(
  orgId: string,
  profile: string,
  raw: string,
): Promise<string> {
  if (/^\d+$/.test(raw)) {
    return raw;
  }
  const parts = raw.split("/").filter((seg) => seg !== "");
  if (parts[0] === orgId) {
    parts.shift();
  }
  if (parts.length >= 2) {
    return parts.join("/");
  }
  const idx = await loadIndex(profile, orgId);
  return pickCachedRepo(raw, idx.repos).pathNs;
}
