import type { OutputMode } from "../uptrace/types";

export type PrometheusOutputMode = OutputMode;

export interface PrometheusConfig {
  baseUrl: string;
  defaultStep?: string | undefined;
  defaultTimeout?: string | undefined;
}

export interface RawPrometheusConfig {
  base_url?: string;
  default_step?: string;
  default_timeout?: string;
}

export interface PrometheusRequestSpec {
  method: "POST";
  url: string;
  body: URLSearchParams;
}

export interface PrometheusStatusRequestSpec {
  method: "GET";
  url: string;
}

export interface PrometheusEnvelope<T> {
  status: "success" | "error";
  data?: T;
  errorType?: string;
  error?: string;
  warnings?: string[];
  infos?: string[];
}

export type PrometheusScalarValue = [number, string];

export interface PrometheusSeries {
  metric: Record<string, string>;
  value?: PrometheusScalarValue;
  values?: PrometheusScalarValue[];
}

export interface PrometheusQueryData {
  resultType: "matrix" | "vector" | "scalar" | "string";
  result: PrometheusSeries[] | PrometheusScalarValue | string;
}

export interface PrometheusMetadataInput {
  matches?: string[] | undefined;
  start?: string | undefined;
  end?: string | undefined;
  limit?: number | undefined;
}

export interface PrometheusQueryInput {
  query: string;
  time?: string | undefined;
  timeout?: string | undefined;
  limit?: number | undefined;
}

export interface PrometheusQueryRangeInput {
  query: string;
  start?: string | undefined;
  end?: string | undefined;
  step?: string | undefined;
  timeout?: string | undefined;
  limit?: number | undefined;
}

export interface PrometheusFormatContext {
  command: string;
  query?: string | undefined;
  includeValues?: boolean | undefined;
}
