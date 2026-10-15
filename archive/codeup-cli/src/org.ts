export const API_BASE = "https://openapi-rdc.aliyuncs.com";
export const CODEUP_HOST = "codeup.aliyun.com";

/**
 * 为什么: Profile.url 被锁成 Organization 根, 多一段就会把 repo 名当成 org id.
 */
export function parseOrgId(rawUrl: string): string {
  let parsed: URL;
  try {
    parsed = new URL(rawUrl);
  } catch {
    throw new Error(`invalid organization url: ${rawUrl}`);
  }
  if (parsed.protocol !== "https:" && parsed.protocol !== "http:") {
    throw new Error(`invalid organization url: ${rawUrl}`);
  }
  if (parsed.hostname !== CODEUP_HOST) {
    throw new Error(`organization url host must be ${CODEUP_HOST}`);
  }
  const segs = parsed.pathname.split("/").filter((seg) => seg !== "");
  if (segs.length !== 1) {
    throw new Error("organization url must be https://codeup.aliyun.com/<organizationId>");
  }
  const raw = segs[0];
  if (raw === undefined) {
    throw new Error("organization id is missing from profile url");
  }
  const orgId = decodeURIComponent(raw);
  if (orgId === "" || orgId === "<organizationId>") {
    throw new Error("organization id is missing from profile url");
  }
  return orgId;
}

/**
 * 为什么: 云效 repositoryId 既吃数字也吃 org/group/repo 的 %2F 编码, 编码规则必须唯一.
 */
export function encodeRepoId(raw: string, orgId: string): string {
  const trimmed = raw.trim();
  if (trimmed === "") {
    throw new Error("repository is required");
  }
  if (/^\d+$/.test(trimmed)) {
    return trimmed;
  }
  if (trimmed.includes("%2F") || trimmed.includes("%2f")) {
    return trimmed;
  }
  const parts = trimmed.split("/").filter((seg) => seg !== "");
  if (parts[0] === orgId) {
    parts.shift();
  }
  if (parts.length === 0) {
    throw new Error("repository must be group/project or numeric id");
  }
  const withOrg = [orgId, ...parts];
  return withOrg
    .map((seg) => encodeURIComponent(seg).replace(/\+/g, "%20"))
    .join("%2F");
}

export function parseGitRemote(url: string): { host: string; path: string } {
  const ssh = url.match(/^[^@]+@([^:]+):(.+?)(?:\.git)?$/);
  if (ssh) {
    const host = ssh[1];
    const path = ssh[2];
    if (host === undefined || path === undefined) {
      throw new Error(`cannot parse git remote: ${url}`);
    }
    return { host, path };
  }
  const https = url.match(/^https?:\/\/(?:[^@]+@)?([^/]+)\/(.+?)(?:\.git)?$/);
  if (https) {
    const host = https[1];
    const path = https[2];
    if (host === undefined || path === undefined) {
      throw new Error(`cannot parse git remote: ${url}`);
    }
    return { host, path };
  }
  throw new Error(`cannot parse git remote: ${url}`);
}

/**
 * 为什么: cwd origin 只能贡献 group/project, org 必须来自 Profile, 避免 remote 里的租户信息进命令行.
 */
export function repoFromRemotePath(path: string, orgId: string): string {
  const parts = path.split("/").filter((seg) => seg !== "");
  if (parts[0] === orgId) {
    parts.shift();
  }
  if (parts.length < 2) {
    throw new Error(`git remote path is not group/project: ${path}`);
  }
  return `${parts[0]}/${parts[1]}`;
}
