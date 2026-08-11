import { appendQuery, buildGroupsRequest, buildGroupStatsRequest, buildSpansRequest, buildTraceRequest } from "./query";
import type { GroupsQueryInput, SpansQueryInput, UptraceConfig, UptraceGroup, UptraceSpan } from "./types";

export interface UptraceClient {
  listGroups(input: GroupsQueryInput): Promise<unknown>;
  listGroupStats(input: GroupsQueryInput): Promise<unknown>;
  listSpans(input: SpansQueryInput): Promise<UptraceSpan[]>;
  getTrace(traceId: string): Promise<UptraceSpan[]>;
}

/** 为什么：把 HTTP 访问隔离起来，CLI 命令只编排能力，不关心协议细节。 */
export function createUptraceClient(config: UptraceConfig, fetcher: typeof fetch = fetch): UptraceClient {
  return {
    async listGroups(input) {
      const spec = buildGroupsRequest(config, input);
      return requestJson(fetcher, appendQuery(spec), spec.headers);
    },
    async listGroupStats(input) {
      const spec = buildGroupStatsRequest(config, input);
      return requestJson(fetcher, appendQuery(spec), spec.headers);
    },
    async listSpans(input) {
      const spec = buildSpansRequest(config, input);
      const body = await requestJson(fetcher, appendQuery(spec), spec.headers);
      return extractSpans(body);
    },
    async getTrace(traceId) {
      const spec = buildTraceRequest(config, traceId);
      const body = await requestJson(fetcher, appendQuery(spec), spec.headers);
      return extractTraceSpans(body);
    },
  };
}

export function extractGroups(body: unknown): UptraceGroup[] {
  if (isRecord(body) && Array.isArray(body.groups)) {
    return body.groups as UptraceGroup[];
  }
  if (Array.isArray(body)) {
    return body as UptraceGroup[];
  }
  return [];
}

export function extractSpans(body: unknown): UptraceSpan[] {
  if (isRecord(body) && Array.isArray(body.spans)) {
    return body.spans as UptraceSpan[];
  }
  if (Array.isArray(body)) {
    return body as UptraceSpan[];
  }
  return [];
}

export function extractTraceSpans(body: unknown): UptraceSpan[] {
  if (isRecord(body) && isRecord(body.root)) {
    return flattenTraceNode(body.root as unknown as UptraceSpan);
  }
  return extractSpans(body);
}

function flattenTraceNode(span: UptraceSpan): UptraceSpan[] {
  const children = Array.isArray(span.children) ? span.children as UptraceSpan[] : [];
  const { children: _children, ...spanWithoutChildren } = span;
  return [spanWithoutChildren, ...children.flatMap((child) => flattenTraceNode(child))];
}

async function requestJson(fetcher: typeof fetch, url: string, headers: Record<string, string>): Promise<unknown> {
  const response = await fetcher(url, { headers });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`Uptrace API failed: ${response.status} ${response.statusText} ${text}`.trim());
  }
  return response.json();
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
