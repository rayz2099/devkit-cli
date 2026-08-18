import { encodeRepoId } from "./org";
import {
  CodeupErr,
  type ChangeRequest,
  type Repo,
  type Runtime,
  type Webhook,
} from "./types";

type Query = Record<string, string | number | undefined>;

/** 为什么: 云效 OpenAPI 的 token/org/编码散落会写出第二套客户端. */
export class CodeupClient {
  constructor(private readonly runtime: Runtime) {}

  async listRepos(opts: {
    search?: string;
    page: number;
    perPage: number;
  }): Promise<Repo[]> {
    const page = await this.listRepoPage(opts);
    return page.repos;
  }

  /** 为什么: Init 必须拉全量, 分页中断会留下残缺 Index. */
  async listAllRepos(): Promise<Repo[]> {
    const perPage = 100;
    const first = await this.listRepoPage({ page: 1, perPage });
    if (first.totalPages > 200) {
      throw new CodeupErr("repository list exceeded 200 pages", 3);
    }
    if (first.totalPages === 1) {
      return first.repos;
    }
    const rest = await Promise.all(
      rangeClosed(2, first.totalPages).map((page) => this.listRepoPage({ page, perPage })),
    );
    return [first, ...rest].flatMap((item) => item.repos);
  }

  private async listRepoPage(opts: {
    search?: string;
    page: number;
    perPage: number;
  }): Promise<RepoPage> {
    const raw = await this.requestRaw(
      "GET",
      `/oapi/v1/codeup/organizations/${this.runtime.orgId}/repositories`,
      {
        search: opts.search,
        page: opts.page,
        perPage: opts.perPage,
        orderBy: "name",
        sort: "asc",
      },
    );
    const meta = parsePageMeta(raw.headers);
    if (meta.page !== opts.page) {
      throw new CodeupErr(`repository page mismatch: want ${opts.page} got ${meta.page}`, 3);
    }
    return {
      ...meta,
      repos: asArray(raw.json, "repository list").map((item) => this.toRepo(item)),
    };
  }

  async getRepo(repo: string): Promise<Repo> {
    const raw = await this.request<unknown>("GET", this.repoPath(repo, ""));
    return this.toRepo(raw);
  }

  async listCrs(opts: {
    repo: string;
    state?: string;
    source?: string;
    target?: string;
    search?: string;
    page: number;
    perPage: number;
  }): Promise<ChangeRequest[]> {
    const repoId = await this.numericId(opts.repo);
    const raw = await this.request<unknown>(
      "GET",
      `/oapi/v1/codeup/organizations/${this.runtime.orgId}/changeRequests`,
      {
        projectIds: repoId,
        state: opts.state === "all" ? undefined : opts.state,
        search: opts.search,
        page: opts.page,
        perPage: opts.perPage,
        orderBy: "updated_at",
        sort: "desc",
      },
    );
    let list = asArray(raw, "change request list").map((item) => this.toCr(item));
    if (opts.source !== undefined) {
      list = list.filter((cr) => cr.srcBranch === opts.source);
    }
    if (opts.target !== undefined) {
      list = list.filter((cr) => cr.tgtBranch === opts.target);
    }
    return list;
  }

  async getCr(repo: string, localId: string): Promise<ChangeRequest> {
    const raw = await this.request<unknown>(
      "GET",
      this.repoPath(repo, `/changeRequests/${localId}`),
    );
    return this.toCr(raw);
  }

  async createCr(opts: {
    repo: string;
    source: string;
    target: string;
    title: string;
    description?: string;
  }): Promise<ChangeRequest> {
    const projectId = toProjectId(await this.numericId(opts.repo));
    await this.requireRemoteBranch(opts.repo, opts.source);
    const raw = await this.request<unknown>(
      "POST",
      this.repoPath(opts.repo, "/changeRequests"),
      undefined,
      crCreateBody({
        projectId,
        source: opts.source,
        target: opts.target,
        title: opts.title,
        description: opts.description,
      }),
    );
    return this.toCr(raw);
  }

  async listHooks(
    repo: string,
    page: number,
    perPage: number,
  ): Promise<Webhook[]> {
    const repoId = await this.numericId(repo);
    const raw = await this.request<unknown>(
      "GET",
      `/oapi/v1/codeup/organizations/${this.runtime.orgId}/repositories/${repoId}/webhooks`,
      { page, perPage },
    );
    return asArray(raw, "webhook list").map((item) => this.toHook(item));
  }

  /** 为什么: COMMAND_LINE 要 sourceCommit, 先确认远端分支比吃 400 便宜. */
  private async requireRemoteBranch(repo: string, branch: string): Promise<void> {
    const encoded = encodeURIComponent(branch);
    try {
      await this.request<unknown>("GET", this.repoPath(repo, `/branches/${encoded}`));
    } catch (err) {
      if (err instanceof CodeupErr && err.code === 2) {
        throw new CodeupErr(`source branch not on remote: ${branch}; push first`, 1);
      }
      throw err;
    }
  }

  private async numericId(repo: string): Promise<string> {
    const got = await this.getRepo(repo);
    if (got.id === "") {
      throw new CodeupErr(`could not resolve numeric id for ${repo}`, 3);
    }
    return got.id;
  }

  private repoPath(repo: string, tail: string): string {
    const encoded = encodeRepoId(repo, this.runtime.orgId);
    return `/oapi/v1/codeup/organizations/${this.runtime.orgId}/repositories/${encoded}${tail}`;
  }

  /** 为什么: 错误文本不能把 Organization 带进 shell 历史. */
  private redact(text: string): string {
    return text.split(this.runtime.orgId).join("<organizationId>");
  }

  private async request<T>(
    method: string,
    path: string,
    query?: Query,
    body?: unknown,
  ): Promise<T> {
    const raw = await this.requestRaw(method, path, query, body);
    return raw.json as T;
  }

  private async requestRaw(
    method: string,
    path: string,
    query?: Query,
    body?: unknown,
  ): Promise<{ json: unknown; headers: Headers }> {
    const url = new URL(path, this.runtime.apiBase);
    if (query !== undefined) {
      for (const [key, value] of Object.entries(query)) {
        if (value !== undefined && value !== "") {
          url.searchParams.set(key, String(value));
        }
      }
    }
    const resp = await fetch(url.toString(), {
      method,
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
        "x-yunxiao-token": this.runtime.token,
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const text = await resp.text();
    if (!resp.ok) {
      throw new CodeupErr(
        `Codeup API ${resp.status} ${resp.statusText}: ${this.redact(url.pathname)}${text === "" ? "" : `: ${this.redact(clipBody(text))}`}`,
        resp.status === 404 ? 2 : 3,
      );
    }
    if (text === "") {
      throw new CodeupErr(`Codeup API empty body: ${this.redact(url.pathname)}`, 3);
    }
    try {
      return { json: JSON.parse(text), headers: resp.headers };
    } catch {
      throw new CodeupErr(`Codeup API invalid JSON: ${this.redact(url.pathname)}`, 3);
    }
  }

  private toRepo(raw: unknown): Repo {
    const rec = asRec(raw, "repository");
    const id = rec.id;
    if (typeof id !== "number" && typeof id !== "string") {
      throw new CodeupErr("repository missing id", 3);
    }
    const pathNs = asStr(rec.pathWithNamespace);
    const path = asStr(rec.path);
    const defBranch = asStr(rec.defaultBranch);
    return {
      id: String(id),
      name: asStr(rec.name),
      path,
      pathNs: pathNs === "" ? path : pathNs,
      webUrl: asStr(rec.webUrl),
      defBranch,
    };
  }

  private toCr(raw: unknown): ChangeRequest {
    const rec = asRec(raw, "change request");
    const localId = rec.localId;
    if (typeof localId !== "number" && typeof localId !== "string") {
      throw new CodeupErr("change request missing localId", 3);
    }
    const webUrl = asStr(rec.webUrl);
    const idText = String(localId);
    return {
      localId: idText,
      title: asStr(rec.title),
      state: asStr(rec.state),
      srcBranch: asStr(rec.sourceBranch),
      tgtBranch: asStr(rec.targetBranch),
      author: asStr(rec.authorUserName),
      createdAt: asStr(rec.createdAt),
      updatedAt: asStr(rec.updatedAt),
      webUrl,
      crUrl: webUrl === "" ? "" : `${webUrl}/change/${idText}`,
      description: asStr(rec.description),
    };
  }

  private toHook(raw: unknown): Webhook {
    const rec = asRec(raw, "webhook");
    const id = rec.id;
    if (typeof id !== "number" && typeof id !== "string") {
      throw new CodeupErr("webhook missing id", 3);
    }
    return {
      id: String(id),
      url: asStr(rec.url),
      secretToken: asStr(rec.token),
      pushEvents: asBool(rec.pushEvents),
      mergeRequestsEvents: asBool(rec.mergeRequestEvents),
      tagPushEvents: asBool(rec.tagPushEvents),
      noteEvents: asBool(rec.noteEvents),
      createdAt: asStr(rec.createdAt),
      updatedAt: asStr(rec.updatedAt),
    };
  }
}

function asArray(raw: unknown, label: string): unknown[] {
  if (!Array.isArray(raw)) {
    throw new CodeupErr(`${label}: expected array`, 3);
  }
  return raw;
}

function asRec(raw: unknown, label: string): Record<string, unknown> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    throw new CodeupErr(`${label}: expected object`, 3);
  }
  return raw as Record<string, unknown>;
}

function asStr(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function asBool(value: unknown): boolean {
  if (typeof value !== "boolean") {
    throw new CodeupErr("expected boolean field", 3);
  }
  return value;
}

/** 为什么: COMMAND_LINE 会强制 sourceCommit, OpenAPI 创建体里根本没有这个字段. */
export function crCreateBody(opts: {
  projectId: number;
  source: string;
  target: string;
  title: string;
  description?: string;
}): Record<string, unknown> {
  const body: Record<string, unknown> = {
    title: opts.title,
    sourceBranch: opts.source,
    targetBranch: opts.target,
    sourceProjectId: opts.projectId,
    targetProjectId: opts.projectId,
  };
  if (opts.description !== undefined) {
    body.description = opts.description;
  }
  return body;
}

export function toProjectId(repoId: string): number {
  if (!/^\d+$/.test(repoId)) {
    throw new CodeupErr(`invalid repository project id: ${repoId}`, 3);
  }
  return Number(repoId);
}

function clipBody(text: string): string {
  return text.length <= 300 ? text : text.slice(0, 300);
}

type RepoPage = {
  page: number;
  totalPages: number;
  total: number;
  repos: Repo[];
};

/** 为什么: 没有 x-total-pages 就不能并行, 禁止猜页数. */
export function parsePageMeta(headers: Headers): {
  page: number;
  totalPages: number;
  total: number;
} {
  return {
    page: headerInt(headers, "x-page"),
    totalPages: headerInt(headers, "x-total-pages"),
    total: headerInt(headers, "x-total"),
  };
}

function headerInt(headers: Headers, name: string): number {
  const raw = headers.get(name);
  if (raw === null || raw.trim() === "") {
    throw new CodeupErr(`missing ${name} header`, 3);
  }
  const value = Number.parseInt(raw, 10);
  if (!Number.isFinite(value) || value < 0) {
    throw new CodeupErr(`invalid ${name} header: ${raw}`, 3);
  }
  return value;
}

function rangeClosed(from: number, to: number): number[] {
  const out: number[] = [];
  for (let page = from; page <= to; page += 1) {
    out.push(page);
  }
  return out;
}
