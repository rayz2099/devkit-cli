import type { GrafanaConfig, RawGrafanaConfig } from "./types";

/** 为什么：Grafana 走内网 basic，缺字段必须直接失败，避免请求打到公网 SSO。 */
export function normalizeGrafanaConfig(raw: RawGrafanaConfig): GrafanaConfig {
  if (!raw.base_url) {
    throw new Error("config missing grafana.base_url");
  }
  if (!raw.username) {
    throw new Error("config missing grafana.username");
  }
  if (!raw.password) {
    throw new Error("config missing grafana.password");
  }

  return {
    baseUrl: trimTrailingSlash(withScheme(raw.base_url)),
    webBaseUrl: raw.web_base_url ? trimTrailingSlash(withScheme(raw.web_base_url)) : undefined,
    username: raw.username,
    password: raw.password,
  };
}

function withScheme(value: string): string {
  if (value.startsWith("http://") || value.startsWith("https://")) {
    return value;
  }
  return `http://${value}`;
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
