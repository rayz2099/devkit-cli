import { lstatSync } from "node:fs";
import { encodePath } from "./path";
import { AlistErr } from "./types";
import type { FsEntry, ListQuery, PutTask, Runtime } from "./types";

export type HttpFn = (input: URL | RequestInfo, init?: RequestInit) => Promise<Response>;

/** 为什么: CLI 只谈 Alist OpenAPI, 命令层不该知道 login/envelope. */
export type AlistApi = {
  listDir: (query: ListQuery) => Promise<FsEntry[]>;
  putFile: (
    src: string,
    dst: string,
    asTask: boolean,
    password: string,
  ) => Promise<PutTask | undefined>;
  mkdir: (path: string, password: string) => Promise<void>;
};

export class AlistClient implements AlistApi {
  private token: string | undefined;

  constructor(
    private readonly runtime: Runtime,
    private readonly http: HttpFn = fetch,
  ) {}

  async listDir(query: ListQuery): Promise<FsEntry[]> {
    const data = await this.postJson("/api/fs/list", {
      path: query.path,
      password: query.password,
      page: query.page,
      per_page: query.size,
      refresh: query.refresh,
    });
    return parseEntries(data);
  }

  async putFile(
    src: string,
    dst: string,
    asTask: boolean,
    password: string,
  ): Promise<PutTask | undefined> {
    let st: ReturnType<typeof lstatSync>;
    try {
      st = lstatSync(src);
    } catch {
      throw new Error(`file not found: ${src}`);
    }
    if (!st.isFile()) {
      throw new Error(`not a file: ${src}`);
    }
    const file = Bun.file(src);
    const headers: Record<string, string> = {
      "File-Path": encodePath(dst),
      "As-Task": asTask ? "true" : "false",
      "Content-Type": "application/octet-stream",
      "Content-Length": String(st.size),
    };
    if (password !== "") {
      headers.Password = password;
    }
    const data = await this.request("PUT", "/api/fs/put", headers, file);
    if (!asTask) {
      if (data === null || data === undefined) {
        return undefined;
      }
      throw new AlistErr("invalid fs put response", 2);
    }
    return parseTask(data);
  }

  async mkdir(path: string, password: string): Promise<void> {
    await this.postJson("/api/fs/mkdir", {
      path,
      password,
    });
  }

  private async postJson(
    path: string,
    body: Record<string, unknown>,
  ): Promise<unknown> {
    return this.request(
      "POST",
      path,
      { "Content-Type": "application/json" },
      JSON.stringify(body),
    );
  }

  private async request(
    method: string,
    path: string,
    headers: Record<string, string>,
    body?: BodyInit,
  ): Promise<unknown> {
    await this.ensureToken();
    const response = await this.http(new URL(path, `${this.runtime.profile.url}/`), {
      method,
      headers: {
        ...headers,
        Authorization: this.token ?? "",
      },
      body,
    });
    const text = await response.text();
    return readEnvelope(text, path, response.status);
  }

  private async ensureToken(): Promise<void> {
    if (this.token !== undefined) {
      return;
    }
    const path = "/api/auth/login";
    const response = await this.http(new URL(path, `${this.runtime.profile.url}/`), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        username: this.runtime.profile.username,
        password: this.runtime.profile.password,
      }),
    });
    const text = await response.text();
    const data = readEnvelope(text, path, response.status);
    this.token = readToken(data);
  }
}

function readEnvelope(text: string, path: string, status: number): unknown {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new AlistErr(
      status >= 400 ? `alist ${path} failed: ${status}` : `alist ${path} invalid json`,
      2,
    );
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new AlistErr(`alist ${path} invalid json`, 2);
  }
  const env = parsed as Record<string, unknown>;
  if (env.code !== 200) {
    const message = typeof env.message === "string" && env.message !== ""
      ? env.message
      : `alist ${path} failed`;
    throw new AlistErr(message, 2);
  }
  return env.data;
}

function readToken(data: unknown): string {
  if (!data || typeof data !== "object" || Array.isArray(data)) {
    throw new AlistErr("alist login failed: token missing", 2);
  }
  const token = (data as Record<string, unknown>).token;
  if (typeof token !== "string" || token === "") {
    throw new AlistErr("alist login failed: token missing", 2);
  }
  return token;
}

function parseEntries(data: unknown): FsEntry[] {
  if (data === null || data === undefined) {
    return [];
  }
  if (!data || typeof data !== "object" || Array.isArray(data)) {
    throw new AlistErr("invalid fs list response", 2);
  }
  const content = (data as Record<string, unknown>).content;
  if (content === null || content === undefined) {
    return [];
  }
  if (!Array.isArray(content)) {
    throw new AlistErr("invalid fs list response", 2);
  }
  return content.map((item) => parseEntry(item));
}

function parseEntry(item: unknown): FsEntry {
  if (!item || typeof item !== "object" || Array.isArray(item)) {
    throw new AlistErr("invalid fs list entry", 2);
  }
  const raw = item as Record<string, unknown>;
  if (typeof raw.name !== "string" || raw.name === "") {
    throw new AlistErr("invalid fs list entry", 2);
  }
  if (typeof raw.size !== "number" || !Number.isFinite(raw.size)) {
    throw new AlistErr("invalid fs list entry", 2);
  }
  if (typeof raw.is_dir !== "boolean") {
    throw new AlistErr("invalid fs list entry", 2);
  }
  if (typeof raw.modified !== "string") {
    throw new AlistErr("invalid fs list entry", 2);
  }
  if (typeof raw.sign !== "string") {
    throw new AlistErr("invalid fs list entry", 2);
  }
  if (typeof raw.thumb !== "string") {
    throw new AlistErr("invalid fs list entry", 2);
  }
  if (typeof raw.type !== "number" || !Number.isFinite(raw.type)) {
    throw new AlistErr("invalid fs list entry", 2);
  }
  return {
    name: raw.name,
    size: raw.size,
    isDir: raw.is_dir,
    modified: raw.modified,
    sign: raw.sign,
    thumb: raw.thumb,
    type: raw.type,
  };
}

function parseTask(data: unknown): PutTask {
  if (data === null || data === undefined || typeof data !== "object" || Array.isArray(data)) {
    throw new AlistErr("invalid fs put response", 2);
  }
  const raw = data as Record<string, unknown>;
  if (typeof raw.id !== "string") {
    throw new AlistErr("invalid fs put response", 2);
  }
  if (typeof raw.name !== "string") {
    throw new AlistErr("invalid fs put response", 2);
  }
  if (typeof raw.progress !== "number" || !Number.isFinite(raw.progress)) {
    throw new AlistErr("invalid fs put response", 2);
  }
  if (typeof raw.state !== "number" || !Number.isFinite(raw.state)) {
    throw new AlistErr("invalid fs put response", 2);
  }
  if (typeof raw.status !== "string") {
    throw new AlistErr("invalid fs put response", 2);
  }
  if (typeof raw.error !== "string") {
    throw new AlistErr("invalid fs put response", 2);
  }
  return {
    id: raw.id,
    name: raw.name,
    progress: raw.progress,
    state: raw.state,
    status: raw.status,
    error: raw.error,
  };
}
