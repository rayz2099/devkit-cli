import {
  buildBuildInfoRequest,
  buildHealthyRequest,
  buildLabelValuesRequest,
  buildLabelsRequest,
  buildQueryRangeRequest,
  buildQueryRequest,
  buildReadyRequest,
  buildRuntimeInfoRequest,
  buildSeriesRequest,
} from "./query";
import type {
  PrometheusConfig,
  PrometheusEnvelope,
  PrometheusMetadataInput,
  PrometheusQueryData,
  PrometheusQueryInput,
  PrometheusQueryRangeInput,
  PrometheusRequestSpec,
  PrometheusStatusRequestSpec,
} from "./types";

export interface PrometheusClient {
  query(input: PrometheusQueryInput): Promise<PrometheusQueryData>;
  queryRange(input: PrometheusQueryRangeInput): Promise<PrometheusQueryData>;
  labels(input: PrometheusMetadataInput): Promise<string[]>;
  labelValues(labelName: string, input: PrometheusMetadataInput): Promise<string[]>;
  series(input: PrometheusMetadataInput): Promise<Record<string, string>[]>;
  raw(spec: PrometheusRequestSpec | PrometheusStatusRequestSpec): Promise<PrometheusEnvelope<unknown>>;
  status(spec: PrometheusStatusRequestSpec): Promise<string>;
  buildInfo(): Promise<unknown>;
  runtimeInfo(): Promise<unknown>;
  ready(): Promise<string>;
  healthy(): Promise<string>;
}

type FetchLike = (input: string, init?: RequestInit) => Promise<Response>;

/** 为什么：Prometheus client 只依赖 fetch，便于测试替换和单文件打包。 */
export function createPrometheusClient(config: PrometheusConfig, fetcher: FetchLike = fetch): PrometheusClient {
  return {
    async query(input) {
      return readPrometheusEnvelope(await request(fetcher, buildQueryRequest(config, input))) as PrometheusQueryData;
    },
    async queryRange(input) {
      return readPrometheusEnvelope(await request(fetcher, buildQueryRangeRequest(config, input))) as PrometheusQueryData;
    },
    async labels(input) {
      return readPrometheusEnvelope(await request(fetcher, buildLabelsRequest(config, input))) as string[];
    },
    async labelValues(labelName, input) {
      return readPrometheusEnvelope(await request(fetcher, buildLabelValuesRequest(config, labelName, input))) as string[];
    },
    async series(input) {
      return readPrometheusEnvelope(await request(fetcher, buildSeriesRequest(config, input))) as Record<string, string>[];
    },
    async raw(spec) {
      return await request(fetcher, spec);
    },
    async status(spec) {
      return await requestText(fetcher, spec);
    },
    async buildInfo() {
      return readPrometheusEnvelope(await request(fetcher, buildBuildInfoRequest(config))) as unknown;
    },
    async runtimeInfo() {
      return readPrometheusEnvelope(await request(fetcher, buildRuntimeInfoRequest(config))) as unknown;
    },
    async ready() {
      return await requestText(fetcher, buildReadyRequest(config));
    },
    async healthy() {
      return await requestText(fetcher, buildHealthyRequest(config));
    },
  };
}

export function readPrometheusEnvelope<T>(envelope: PrometheusEnvelope<T>): T {
  if (envelope.status === "success") {
    if (envelope.data === undefined) {
      throw new Error("prometheus response missing data");
    }
    return envelope.data;
  }

  const type = envelope.errorType ?? "error";
  const message = envelope.error ?? "unknown prometheus error";
  throw new Error(`${type}: ${message}`);
}

async function request(fetcher: FetchLike, spec: PrometheusRequestSpec | PrometheusStatusRequestSpec): Promise<PrometheusEnvelope<unknown>> {
  const init: RequestInit = spec.method === "POST"
    ? {
        method: spec.method,
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: spec.body.toString(),
      }
    : { method: spec.method };
  const response = await fetcher(spec.url, init);
  const text = await response.text();
  const body = parseJson(text);

  if (isPrometheusEnvelope(body)) {
    if (!response.ok && body.status !== "error") {
      throw new Error(`prometheus http ${response.status}: ${text}`);
    }
    return body;
  }

  if (!response.ok) {
    throw new Error(`prometheus http ${response.status}: ${text}`);
  }
  throw new Error("prometheus response is not a valid envelope");
}

async function requestText(fetcher: FetchLike, spec: PrometheusStatusRequestSpec): Promise<string> {
  const response = await fetcher(spec.url, { method: spec.method });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`prometheus http ${response.status}: ${text}`);
  }
  return text.endsWith("\n") ? text : `${text}\n`;
}

function parseJson(text: string): unknown {
  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw new Error(`prometheus response is not json: ${text}`);
  }
}

function isPrometheusEnvelope(value: unknown): value is PrometheusEnvelope<unknown> {
  if (!value || typeof value !== "object") {
    return false;
  }
  const status = (value as { status?: unknown }).status;
  return status === "success" || status === "error";
}
