import { buildAggregateRequest, buildSearchRequest } from "./query";
import type { GraylogAggregateInput, GraylogAggregateResult, GraylogConfig, GraylogPostSpec, GraylogRequestSpec, GraylogSearchInput, GraylogSearchResponse } from "./types";

export interface GraylogClient {
  search(input: GraylogSearchInput): Promise<GraylogSearchResponse>;
  aggregate(input: GraylogAggregateInput): Promise<GraylogAggregateResult>;
  raw(spec: GraylogRequestSpec): Promise<GraylogSearchResponse>;
}

type FetchLike = (input: string, init?: RequestInit) => Promise<Response>;

/** 为什么：Graylog client 只封装协议细节，方便 Agent 命令和测试共用同一请求契约。 */
export function createGraylogClient(config: GraylogConfig, fetcher: FetchLike = fetch): GraylogClient {
  return {
    async search(input) {
      return await request(config, fetcher, buildSearchRequest(config, input));
    },
    async aggregate(input) {
      return readAggregateResponse(await post(config, fetcher, buildAggregateRequest(config, input)), input);
    },
    async raw(spec) {
      return await request(config, fetcher, spec);
    },
  };
}

async function post(config: GraylogConfig, fetcher: FetchLike, spec: GraylogPostSpec): Promise<unknown> {
  const query = spec.params ? `?${spec.params.toString()}` : "";
  const response = await fetcher(`${spec.url}${query}`, {
    method: spec.method,
    headers: {
      Accept: "application/json",
      Authorization: `Basic ${btoa(`${config.username}:${config.password}`)}`,
      "Content-Type": "application/json",
      "X-Requested-By": "olly-cli",
    },
    body: JSON.stringify(spec.body),
  });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`graylog http ${response.status}: ${text}`);
  }
  return parseUnknownJson(text);
}

async function request(config: GraylogConfig, fetcher: FetchLike, spec: GraylogRequestSpec): Promise<GraylogSearchResponse> {
  const response = await fetcher(`${spec.url}?${spec.params.toString()}`, {
    method: spec.method,
    headers: {
      Accept: "application/json",
      Authorization: `Basic ${btoa(`${config.username}:${config.password}`)}`,
      "X-Requested-By": "olly-cli",
    },
  });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`graylog http ${response.status}: ${text}`);
  }
  return parseJson(text);
}

function parseJson(text: string): GraylogSearchResponse {
  try {
    return JSON.parse(text) as GraylogSearchResponse;
  } catch {
    throw new Error(`graylog response is not json: ${text}`);
  }
}

function parseUnknownJson(text: string): unknown {
  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw new Error(`graylog response is not json: ${text}`);
  }
}

function readAggregateResponse(body: unknown, input: GraylogAggregateInput): GraylogAggregateResult {
  const searchType = (((body as Record<string, unknown>).results as Record<string, unknown> | undefined)?.["query-1"] as Record<string, unknown> | undefined)?.search_types as Record<string, unknown> | undefined;
  const pivot = searchType?.["pivot-1"] as { rows?: unknown[]; total?: number; effective_timerange?: unknown } | undefined;
  const rows = (pivot?.rows ?? []).flatMap((row) => toAggregateRow(row));
  return {
    query: input.query,
    field: input.field,
    total: pivot?.total ?? 0,
    rows,
    effective_timerange: pivot?.effective_timerange,
  };
}

function toAggregateRow(row: unknown): GraylogAggregateResult["rows"] {
  const item = row as { key?: unknown[]; values?: unknown[]; source?: string } | undefined;
  if (!item || item.source !== "leaf") {
    return [];
  }
  const key = item.key?.[0];
  const value = (item.values?.[0] as { value?: unknown } | undefined)?.value;
  if (typeof key !== "string" || typeof value !== "number") {
    return [];
  }
  return [{ key, count: value }];
}
