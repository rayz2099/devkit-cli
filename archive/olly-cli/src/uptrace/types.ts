export type OutputMode = "human" | "agent" | "plain";

export interface UptraceConfig {
  baseUrl: string;
  webBaseUrl?: string | undefined;
  projectId: number;
  authToken?: string | undefined;
  jwtToken?: string | undefined;
  defaultEnv?: string | undefined;
  defaultTimeDurSeconds?: number | undefined;
}

export interface RequestSpec {
  url: string;
  query: URLSearchParams;
  headers: Record<string, string>;
}

export interface TimeRangeInput {
  timeStart?: string | undefined;
  timeEnd?: string | undefined;
  timeGte?: string | undefined;
  timeLt?: string | undefined;
  timeDurSeconds?: number | undefined;
}

export interface GroupsQueryInput extends TimeRangeInput {
  service?: string | undefined;
  env?: string | undefined;
  query?: string | undefined;
  search?: string | undefined;
  limit?: number | undefined;
  durationGte?: number | undefined;
  durationLt?: number | undefined;
}

export interface SpansQueryInput extends TimeRangeInput {
  traceId?: string | undefined;
  spanId?: number | undefined;
  parentId?: number | undefined;
  limit?: number | undefined;
  durationGte?: number | undefined;
  durationLt?: number | undefined;
  query?: string | undefined;
  sortBy?: string | undefined;
  sortDesc?: boolean | undefined;
  page?: number | undefined;
}

export interface UptraceSpan {
  id: number | string;
  traceId?: string;
  trace_id?: string;
  parentId?: number | string;
  parent_id?: number | string;
  name?: string;
  kind?: string;
  time?: string;
  duration?: number;
  statusCode?: string;
  status_code?: string;
  attrs?: Record<string, unknown>;
  attributes?: Record<string, unknown>;
  [key: string]: unknown;
}

export interface UptraceGroup {
  group?: string;
  _group_id?: string;
  service_name?: string;
  [key: string]: unknown;
}

export interface TraceNode {
  span: UptraceSpan;
  children: TraceNode[];
  depth: number;
  durationShare: number;
  isError: boolean;
  isSlow: boolean;
}

export interface TraceTree {
  roots: TraceNode[];
  orphans: TraceNode[];
  allNodes: TraceNode[];
  topSlowSpans: TraceNode[];
  errorSpans: TraceNode[];
  totalDuration: number;
}

export interface ContextInput {
  config: UptraceConfig;
  traceInput: string;
  spans: UptraceSpan[];
  query?: {
    service?: string | undefined;
    env?: string | undefined;
    uri?: string | undefined;
  };
}

export interface AgentContext {
  projectId: number;
  traceId: string;
  traceUrl: string;
  query: {
    service?: string | undefined;
    env?: string | undefined;
    uri?: string | undefined;
  };
  tree: TraceTree;
  sourceHints: string[];
}
