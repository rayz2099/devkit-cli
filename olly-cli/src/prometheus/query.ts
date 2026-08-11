import type { PrometheusConfig, PrometheusMetadataInput, PrometheusQueryInput, PrometheusQueryRangeInput, PrometheusRequestSpec, PrometheusStatusRequestSpec } from "./types";

/** 为什么：PromQL 经常很长并包含特殊字符，统一 POST form 避免 URL 长度和转义问题。 */
export function buildQueryRequest(config: PrometheusConfig, input: PrometheusQueryInput): PrometheusRequestSpec {
  requireNonEmpty(input.query, "query");
  const body = new URLSearchParams();
  body.set("query", input.query);
  appendOptional(body, "time", input.time);
  appendOptional(body, "timeout", input.timeout ?? config.defaultTimeout);
  appendOptionalNumber(body, "limit", input.limit);
  return post(config, "/api/v1/query", body);
}

export function buildQueryRangeRequest(config: PrometheusConfig, input: PrometheusQueryRangeInput): PrometheusRequestSpec {
  requireNonEmpty(input.query, "query");
  requireNonEmpty(input.start, "--start");
  requireNonEmpty(input.end, "--end");
  const step = input.step ?? config.defaultStep;
  requireNonEmpty(step, "--step");

  const body = new URLSearchParams();
  body.set("query", input.query);
  body.set("start", input.start);
  body.set("end", input.end);
  body.set("step", step);
  appendOptional(body, "timeout", input.timeout ?? config.defaultTimeout);
  appendOptionalNumber(body, "limit", input.limit);
  return post(config, "/api/v1/query_range", body);
}

export function buildLabelsRequest(config: PrometheusConfig, input: PrometheusMetadataInput): PrometheusRequestSpec {
  const body = metadataBody(input);
  return post(config, "/api/v1/labels", body);
}

export function buildLabelValuesRequest(config: PrometheusConfig, labelName: string, input: PrometheusMetadataInput): PrometheusRequestSpec {
  requireNonEmpty(labelName, "label name");
  const body = metadataBody(input);
  return post(config, `/api/v1/label/${encodeURIComponent(labelName)}/values`, body);
}

export function buildSeriesRequest(config: PrometheusConfig, input: PrometheusMetadataInput): PrometheusRequestSpec {
  if (!input.matches || input.matches.length === 0) {
    throw new Error("prometheus query series requires at least one --match");
  }
  const body = metadataBody(input);
  return post(config, "/api/v1/series", body);
}

export function buildReadyRequest(config: PrometheusConfig): PrometheusStatusRequestSpec {
  return get(config, "/-/ready");
}

export function buildHealthyRequest(config: PrometheusConfig): PrometheusStatusRequestSpec {
  return get(config, "/-/healthy");
}

export function buildBuildInfoRequest(config: PrometheusConfig): PrometheusStatusRequestSpec {
  return get(config, "/api/v1/status/buildinfo");
}

export function buildRuntimeInfoRequest(config: PrometheusConfig): PrometheusStatusRequestSpec {
  return get(config, "/api/v1/status/runtimeinfo");
}

function metadataBody(input: PrometheusMetadataInput): URLSearchParams {
  const body = new URLSearchParams();
  for (const match of input.matches ?? []) {
    body.append("match[]", match);
  }
  appendOptional(body, "start", input.start);
  appendOptional(body, "end", input.end);
  appendOptionalNumber(body, "limit", input.limit);
  return body;
}

function post(config: PrometheusConfig, path: string, body: URLSearchParams): PrometheusRequestSpec {
  return {
    method: "POST",
    url: `${config.baseUrl}${path}`,
    body,
  };
}

function get(config: PrometheusConfig, path: string): PrometheusStatusRequestSpec {
  return {
    method: "GET",
    url: `${config.baseUrl}${path}`,
  };
}

function appendOptional(body: URLSearchParams, key: string, value: string | undefined): void {
  if (value !== undefined) {
    body.set(key, value);
  }
}

function appendOptionalNumber(body: URLSearchParams, key: string, value: number | undefined): void {
  if (value !== undefined) {
    body.set(key, String(value));
  }
}

function requireNonEmpty(value: string | undefined, name: string): asserts value is string {
  if (!value || value.length === 0) {
    throw new Error(`prometheus ${name} is required`);
  }
}
