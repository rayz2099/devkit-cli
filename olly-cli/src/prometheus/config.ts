import type { PrometheusConfig, RawPrometheusConfig } from "./types";

export const DEFAULT_PROMETHEUS_BASE_URL = "http://127.0.0.1:9090";

/** 为什么：Prometheus 查询应可零配置运行，默认 endpoint 来自原 prom-cli 约定。 */
export function normalizePrometheusConfig(raw: RawPrometheusConfig | undefined): PrometheusConfig {
  return {
    baseUrl: trimTrailingSlash(raw?.base_url ?? DEFAULT_PROMETHEUS_BASE_URL),
    defaultStep: raw?.default_step,
    defaultTimeout: raw?.default_timeout,
  };
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
