import type { OutputMode } from "../uptrace/types";

export type GraylogOutputMode = OutputMode;

export interface GraylogConfig {
  baseUrl: string;
  username: string;
  password: string;
}

export interface RawGraylogConfig {
  base_url?: string | undefined;
  username?: string | undefined;
  password?: string | undefined;
}

export interface GraylogSearchInput {
  query: string;
  rangeType: "relative" | "absolute" | "keyword";
  relative?: number | undefined;
  from?: string | undefined;
  to?: string | undefined;
  keyword?: string | undefined;
  limit?: number | undefined;
  sort?: string | undefined;
  fields?: string | undefined;
  filter?: string | undefined;
  offset?: number | undefined;
  decorate?: boolean | undefined;
}

export interface GraylogSearchSource {
  source?: string | undefined;
  relative?: number | undefined;
  from?: string | undefined;
  to?: string | undefined;
  keyword?: string | undefined;
  limit?: number | undefined;
  sort?: string | undefined;
  fields?: string | undefined;
  filter?: string | undefined;
  offset?: number | undefined;
  decorate?: boolean | undefined;
}

export interface GraylogRequestSpec {
  method: "GET";
  url: string;
  params: URLSearchParams;
}

export interface GraylogPostSpec {
  method: "POST";
  url: string;
  params?: URLSearchParams | undefined;
  body: unknown;
}

export interface GraylogSearchResponse {
  total_results?: number | undefined;
  messages?: GraylogMessageEnvelope[] | undefined;
  fields?: string[] | undefined;
}

export interface GraylogMessageEnvelope {
  message?: GraylogMessage | undefined;
}

export interface GraylogMessage {
  timestamp?: string | undefined;
  message?: string | undefined;
  source?: string | undefined;
  logger_name?: string | undefined;
  loggerName?: string | undefined;
  trace_id?: string | undefined;
  traceId?: string | undefined;
  level?: number | string | undefined;
  [key: string]: unknown;
}

export interface GraylogFormatContext {
  query: string;
  rangeType: "relative" | "absolute" | "keyword";
  relative?: number | undefined;
  from?: string | undefined;
  to?: string | undefined;
  keyword?: string | undefined;
  groupBy?: string | undefined;
}

export interface GraylogAggregateInput {
  query: string;
  rangeType: "relative" | "absolute" | "keyword";
  field: string;
  relative?: number | undefined;
  from?: string | undefined;
  to?: string | undefined;
  keyword?: string | undefined;
  limit?: number | undefined;
}

export interface GraylogAggregateRow {
  key: string;
  count: number;
}

export interface GraylogAggregateResult {
  query: string;
  field: string;
  total: number;
  rows: GraylogAggregateRow[];
  effective_timerange?: unknown;
}
