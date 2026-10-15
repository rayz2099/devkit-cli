import type { GroupsQueryInput, RequestSpec, SpansQueryInput, TimeRangeInput, UptraceConfig } from "./types";

const DEFAULT_TIME_DUR_SECONDS = 10800;
const DEFAULT_GROUP_STATS_COLUMNS = [
  "per_min(sum(_count))",
  "_error_rate",
  "p50(_duration)",
  "p90(_duration)",
  "p99(_duration)",
];

export interface TimeRange {
  timeGte: string;
  timeLt: string;
}

/** 为什么：页面上的 time_gte/time_dur 需要转成官方 JSON API 的 time_start/time_end。 */
export function parseTimeRange(input: TimeRangeInput, now: Date = new Date()): TimeRange {
  const explicitEnd = input.timeEnd ?? input.timeLt;
  if (input.timeStart && explicitEnd) {
    return {
      timeGte: normalizeApiTime(input.timeStart),
      timeLt: normalizeApiTime(explicitEnd),
    };
  }

  const durationSeconds = input.timeDurSeconds ?? DEFAULT_TIME_DUR_SECONDS;
  if (input.timeGte) {
    const start = parseLooseTime(input.timeGte);
    return {
      timeGte: start.toISOString(),
      timeLt: new Date(start.getTime() + durationSeconds * 1000).toISOString(),
    };
  }

  return {
    timeGte: new Date(now.getTime() - durationSeconds * 1000).toISOString(),
    timeLt: now.toISOString(),
  };
}

/** 为什么：服务聚合是从“哪个接口慢”进入排查链路的第一步。 */
export function buildGroupsRequest(config: UptraceConfig, input: GroupsQueryInput): RequestSpec {
  return buildGroupsLikeRequest(config, "groups", input);
}

/** 为什么：Uptrace 页面选中 system 后用 group-stats 取固定指标列。 */
export function buildGroupStatsRequest(config: UptraceConfig, input: GroupsQueryInput): RequestSpec {
  const spec = buildGroupsLikeRequest(config, "group-stats", input);
  for (const column of DEFAULT_GROUP_STATS_COLUMNS) {
    spec.query.append("column[]", column);
  }
  return spec;
}

function buildGroupsLikeRequest(config: UptraceConfig, kind: "groups" | "group-stats", input: GroupsQueryInput): RequestSpec {
  const query = new URLSearchParams();
  const timeRange = parseTimeRange({
    ...input,
    timeDurSeconds: input.timeDurSeconds ?? config.defaultTimeDurSeconds,
  });
  addInternalTimeQuery(query, timeRange);
  query.set("query", input.query ?? buildDefaultGroupsUql(input.service, input.env ?? config.defaultEnv));
  addNumber(query, "limit", input.limit);
  addString(query, "search", input.search);
  addNumber(query, "duration_gte", input.durationGte);
  addNumber(query, "duration_lt", input.durationLt);

  return buildRequest(config, kind, query);
}

/** 为什么：span 明细承接聚合结果，用于定位具体慢 trace 或直接拉完整调用链。 */
export function buildSpansRequest(config: UptraceConfig, input: SpansQueryInput): RequestSpec {
  const query = new URLSearchParams();
  const timeRange = parseTimeRange({
    ...input,
    timeDurSeconds: input.timeDurSeconds ?? config.defaultTimeDurSeconds,
  });
  addInternalTimeQuery(query, timeRange);
  addString(query, "query", input.query);
  addString(query, "trace_id", input.traceId);
  addNumber(query, "id", input.spanId);
  addNumber(query, "parent_id", input.parentId);
  addNumber(query, "limit", input.limit);
  addNumber(query, "duration_gte", input.durationGte);
  addNumber(query, "duration_lt", input.durationLt);
  addString(query, "sort_by", input.sortBy);
  addBoolean(query, "sort_desc", input.sortDesc);
  addNumber(query, "page", input.page);

  return buildRequest(config, "spans", query);
}

/** 为什么：浏览器实际使用内部 trace endpoint，返回完整嵌套调用链。 */
export function buildTraceRequest(config: UptraceConfig, traceId: string): RequestSpec {
  return {
    url: `${trimRightSlash(config.baseUrl)}/internal/v1/tracing/${config.projectId}/traces/${traceId}`,
    query: new URLSearchParams(),
    headers: buildAuthHeaders(config),
  };
}

/** 为什么：默认 UQL 固化压测常用视角，仍允许用户用 --query 覆盖。 */
export function buildDefaultGroupsUql(service?: string, env?: string): string {
  const clauses = [
    "group by _group_id",
    "per_min(sum(_count))",
    "_error_rate",
    "{p50,p90,p99}(_duration)",
  ];
  if (env) {
    clauses.push(`where deployment_environment = "${escapeUqlString(env)}"`);
  }
  if (service) {
    clauses.push(`where service_name = "${escapeUqlString(service)}"`);
  }
  return clauses.join(" | ");
}

export function appendQuery(spec: RequestSpec): string {
  const text = spec.query.toString();
  return text ? `${spec.url}?${text}` : spec.url;
}

function buildRequest(config: UptraceConfig, kind: "groups" | "group-stats" | "spans", query: URLSearchParams): RequestSpec {
  return {
    url: `${trimRightSlash(config.baseUrl)}/internal/v1/tracing/${config.projectId}/${kind}`,
    query,
    headers: buildAuthHeaders(config),
  };
}

function addInternalTimeQuery(query: URLSearchParams, timeRange: TimeRange): void {
  query.set("time_gte", timeRange.timeGte);
  query.set("time_lt", timeRange.timeLt);
  query.set("system[]", "all");
}

/** 为什么：Uptrace API token 需要额外维护，内部环境可直接复用登录态 JWT cookie。 */
function buildAuthHeaders(config: UptraceConfig): Record<string, string> {
  const headers: Record<string, string> = {
    Accept: "application/json",
  };
  if (config.jwtToken) {
    headers.Cookie = `token=${config.jwtToken}`;
    return headers;
  }
  if (config.authToken) {
    headers.Authorization = `Bearer ${config.authToken}`;
    return headers;
  }
  throw new Error("config missing uptrace auth: expected auth_token or jwt_token");
}

function normalizeApiTime(value: string): string {
  return parseLooseTime(value).toISOString();
}

function parseLooseTime(value: string): Date {
  const compact = value.match(/^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})$/);
  if (compact) {
    const [, year, month, day, hour, minute, second] = compact;
    return new Date(Date.UTC(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute), Number(second)));
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    throw new Error(`invalid time value: ${value}`);
  }
  return date;
}

function addString(query: URLSearchParams, key: string, value?: string): void {
  if (value !== undefined && value !== "") {
    query.set(key, value);
  }
}

function addNumber(query: URLSearchParams, key: string, value?: number): void {
  if (value !== undefined) {
    query.set(key, String(value));
  }
}

function addBoolean(query: URLSearchParams, key: string, value?: boolean): void {
  if (value !== undefined) {
    query.set(key, String(value));
  }
}

function escapeUqlString(value: string): string {
  return value.replaceAll("\\", "\\\\").replaceAll('"', '\\"');
}

function trimRightSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
