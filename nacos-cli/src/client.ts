import { appendFileSync, mkdirSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { cacheKey, readListCache, writeListCache } from "./cache";
import type {
  ConfigItem,
  ConfigPage,
  NamingInst,
  Runtime,
  SearchMode,
  ServerEp,
} from "./types";

export type HttpFn = (input: URL | RequestInfo, init?: RequestInit) => Promise<Response>;

/** 为什么: CLI 直打 OpenAPI, 去掉 Go SDK 的 snapshot/cache, 结果只反映当前 server. */
export class NacosClient {
  private token: string | undefined;
  private readonly baseUrl: string;

  constructor(
    private readonly runtime: Runtime,
    private readonly http: HttpFn = fetch,
  ) {
    const endpoint = parseServerAddr(runtime.serverAddr)[0];
    if (endpoint === undefined) {
      throw new Error("server-addr is required");
    }
    this.baseUrl = `${endpoint.scheme}://${endpoint.host}:${endpoint.port}`;
  }

  async getConfig(dataId: string, group: string): Promise<string> {
    const body = await this.request(
      "GET",
      "/nacos/v1/cs/configs",
      {
        dataId,
        group,
        tenant: this.runtime.namespace,
      },
    );
    return body;
  }

  async putConfig(
    dataId: string,
    group: string,
    content: string,
  ): Promise<void> {
    const body = await this.request(
      "POST",
      "/nacos/v1/cs/configs",
      {},
      {
        dataId,
        group,
        content,
        tenant: this.runtime.namespace,
      },
    );
    if (body.trim().toLowerCase() !== "true") {
      throw new Error("publish config failed");
    }
  }

  async deleteConfig(dataId: string, group: string): Promise<void> {
    const body = await this.request(
      "DELETE",
      "/nacos/v1/cs/configs",
      {
        dataId,
        group,
        tenant: this.runtime.namespace,
      },
    );
    if (body.trim().toLowerCase() !== "true") {
      throw new Error("delete config failed");
    }
  }

  async listConfigs(input: {
    search: SearchMode;
    dataId: string;
    group: string;
    pageNo: number;
    pageSize: number;
  }): Promise<ConfigPage> {
    const body = await this.request(
      "GET",
      "/nacos/v1/cs/configs",
      {
        search: input.search,
        dataId: input.dataId,
        group: input.group,
        pageNo: String(input.pageNo),
        pageSize: String(input.pageSize),
        tenant: this.runtime.namespace,
      },
    );
    return parseConfigPage(body, input.pageNo);
  }

  async registerInst(input: {
    service: string;
    ip: string;
    port: number;
    group: string;
    cluster: string;
    weight: number;
    ephemeral: boolean;
  }): Promise<void> {
    const body = await this.request(
      "POST",
      "/nacos/v1/ns/instance",
      {
        serviceName: input.service,
        ip: input.ip,
        port: String(input.port),
        groupName: input.group,
        clusterName: input.cluster,
        weight: String(input.weight),
        enable: "true",
        healthy: "true",
        ephemeral: String(input.ephemeral),
        namespaceId: this.runtime.namespace,
      },
    );
    if (body.trim().toLowerCase() !== "ok") {
      throw new Error("register instance failed");
    }
  }

  async deregisterInst(input: {
    service: string;
    ip: string;
    port: number;
    group: string;
    cluster: string;
    ephemeral: boolean;
  }): Promise<void> {
    const body = await this.request(
      "DELETE",
      "/nacos/v1/ns/instance",
      {
        serviceName: input.service,
        ip: input.ip,
        port: String(input.port),
        groupName: input.group,
        clusterName: input.cluster,
        ephemeral: String(input.ephemeral),
        namespaceId: this.runtime.namespace,
      },
    );
    if (body.trim().toLowerCase() !== "ok") {
      throw new Error("deregister instance failed");
    }
  }

  async listInsts(input: {
    service: string;
    group: string;
    clusters: string[];
    healthyOnly: boolean;
  }): Promise<NamingInst[]> {
    const query: Record<string, string> = {
      serviceName: input.service,
      groupName: input.group,
      healthyOnly: String(input.healthyOnly),
      namespaceId: this.runtime.namespace,
    };
    if (input.clusters.length > 0) {
      query.clusters = input.clusters.join(",");
    }
    const body = await this.request("GET", "/nacos/v1/ns/instance/list", query);
    return parseInsts(body);
  }

  async listItemsCached(): Promise<ConfigItem[]> {
    const key = cacheKey(this.runtime.serverAddr, this.runtime.namespace, this.runtime.username);
    const hit = await readListCache(key);
    if (hit !== undefined) {
      return hit;
    }

    const page = await this.listConfigs({
      search: "blur",
      dataId: "",
      group: "",
      pageNo: 1,
      pageSize: 1000,
    });
    await writeListCache(key, page.pageItems);
    return page.pageItems.slice();
  }

  private async request(
    method: string,
    path: string,
    query: Record<string, string>,
    form?: Record<string, string>,
  ): Promise<string> {
    await this.ensureToken();
    const url = new URL(path, this.baseUrl);
    for (const [key, value] of Object.entries(query)) {
      url.searchParams.set(key, value);
    }
    if (this.token !== undefined) {
      url.searchParams.set("accessToken", this.token);
    }

    const headers: Record<string, string> = {};
    let body: string | undefined;
    if (form !== undefined) {
      const params = new URLSearchParams(form);
      if (this.token !== undefined) {
        params.set("accessToken", this.token);
      }
      body = params.toString();
      headers["content-type"] = "application/x-www-form-urlencoded";
    }

    const started = Date.now();
    const response = await this.http(url, { method, headers, body });
    const text = await response.text();
    this.writeDevLog(`${method} ${url.pathname} ${response.status} ${Date.now() - started}ms`);
    if (!response.ok) {
      throw new Error(`nacos ${method} ${path} failed: ${response.status} ${text}`.trim());
    }
    return text;
  }

  private async ensureToken(): Promise<void> {
    if (this.token !== undefined || this.runtime.username.trim() === "") {
      return;
    }

    const url = new URL("/nacos/v1/auth/login", this.baseUrl);
    const body = new URLSearchParams({
      username: this.runtime.username,
      password: this.runtime.password,
    }).toString();
    const response = await this.http(url, {
      method: "POST",
      headers: { "content-type": "application/x-www-form-urlencoded" },
      body,
    });
    const text = await response.text();
    this.writeDevLog(`POST /nacos/v1/auth/login ${response.status}`);
    if (!response.ok) {
      throw new Error(`nacos login failed: ${response.status} ${text}`.trim());
    }

    const parsed = JSON.parse(text) as { accessToken?: unknown };
    if (typeof parsed.accessToken !== "string" || parsed.accessToken === "") {
      throw new Error("nacos login failed: accessToken missing");
    }
    this.token = parsed.accessToken;
  }

  private writeDevLog(line: string): void {
    if (!this.runtime.dev) {
      return;
    }
    const dir = join(homedir(), ".config", "nacos-cli", "log");
    mkdirSync(dir, { recursive: true });
    appendFileSync(join(dir, "nacos-cli.log"), `${new Date().toISOString()} ${line}\n`);
  }
}

/** 为什么: server-addr 要兼容 host:port 和带 scheme 的 URL, 和旧 CLI 输入一致. */
export function parseServerAddr(raw: string): ServerEp[] {
  const result: ServerEp[] = [];
  for (const part of raw.split(",")) {
    const item = part.trim();
    if (item === "") {
      continue;
    }
    result.push(parseAddress(item));
  }
  if (result.length === 0) {
    throw new Error("server-addr is required");
  }
  return result;
}

export function parseAddress(addr: string): ServerEp {
  const value = addr.trim();
  if (value === "") {
    throw new Error(`invalid server address: ${addr}`);
  }

  if (value.includes("://")) {
    let parsed: URL;
    try {
      parsed = new URL(value);
    } catch {
      throw new Error(`invalid server address: ${addr}`);
    }
    const host = parsed.hostname.trim();
    if (host === "") {
      throw new Error(`invalid server address: ${addr}`);
    }
    const scheme = parsed.protocol === "https:" ? "https" : "http";
    if (parsed.port === "") {
      return { scheme, host, port: 8848 };
    }
    const port = Number(parsed.port);
    if (!Number.isInteger(port) || port <= 0) {
      throw new Error(`invalid server port in address ${addr}`);
    }
    return { scheme, host, port };
  }

  const lastColon = value.lastIndexOf(":");
  if (lastColon === -1) {
    return { scheme: "http", host: value, port: 8848 };
  }

  const host = value.slice(0, lastColon).trim();
  const portRaw = value.slice(lastColon + 1).trim();
  if (host === "" || portRaw === "") {
    throw new Error(`invalid server address: ${addr}`);
  }
  const port = Number(portRaw);
  if (!Number.isInteger(port) || port <= 0) {
    throw new Error(`invalid server port in address ${addr}`);
  }
  return { scheme: "http", host, port };
}

function parseConfigPage(body: string, pageNo: number): ConfigPage {
  if (body.trim() === "") {
    return {
      totalCount: 0,
      pageNumber: pageNo,
      pagesAvailable: 0,
      pageItems: [],
    };
  }

  const parsed = JSON.parse(body) as {
    totalCount?: unknown;
    pageNumber?: unknown;
    pagesAvailable?: unknown;
    pageItems?: unknown;
  };
  const items = Array.isArray(parsed.pageItems) ? parsed.pageItems : [];
  return {
    totalCount: asNumber(parsed.totalCount, 0),
    pageNumber: asNumber(parsed.pageNumber, pageNo),
    pagesAvailable: asNumber(parsed.pagesAvailable, 0),
    pageItems: items.flatMap((item) => {
      if (!item || typeof item !== "object") {
        return [];
      }
      const row = item as { dataId?: unknown; group?: unknown; content?: unknown };
      if (typeof row.dataId !== "string" || typeof row.group !== "string") {
        return [];
      }
      return [{
        dataId: row.dataId,
        group: row.group,
        content: typeof row.content === "string" ? row.content : undefined,
      }];
    }),
  };
}

function parseInsts(body: string): NamingInst[] {
  if (body.trim() === "") {
    return [];
  }
  const parsed = JSON.parse(body) as { hosts?: unknown };
  if (!Array.isArray(parsed.hosts)) {
    return [];
  }
  return parsed.hosts.flatMap((item) => {
    if (!item || typeof item !== "object") {
      return [];
    }
    const row = item as {
      ip?: unknown;
      port?: unknown;
      weight?: unknown;
      healthy?: unknown;
      enabled?: unknown;
      ephemeral?: unknown;
      clusterName?: unknown;
      serviceName?: unknown;
      metadata?: unknown;
    };
    if (typeof row.ip !== "string" || typeof row.port !== "number") {
      return [];
    }
    return [{
      ip: row.ip,
      port: row.port,
      weight: typeof row.weight === "number" ? row.weight : undefined,
      healthy: typeof row.healthy === "boolean" ? row.healthy : undefined,
      enabled: typeof row.enabled === "boolean" ? row.enabled : undefined,
      ephemeral: typeof row.ephemeral === "boolean" ? row.ephemeral : undefined,
      clusterName: typeof row.clusterName === "string" ? row.clusterName : undefined,
      serviceName: typeof row.serviceName === "string" ? row.serviceName : undefined,
      metadata:
        row.metadata && typeof row.metadata === "object" && !Array.isArray(row.metadata)
          ? Object.fromEntries(
              Object.entries(row.metadata as Record<string, unknown>).filter(
                (entry): entry is [string, string] => typeof entry[1] === "string",
              ),
            )
          : undefined,
    }];
  });
}

function asNumber(value: unknown, defaultValue: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : defaultValue;
}
