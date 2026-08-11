import type { GraylogConfig, RawGraylogConfig } from "./types";

/** 为什么：Graylog 配置来自内网地址时常省略 scheme，HTTP client 需要标准 URL。 */
export function normalizeGraylogConfig(raw: RawGraylogConfig): GraylogConfig {
  if (!raw.base_url) {
    throw new Error("config missing graylog.base_url");
  }
  if (!raw.username) {
    throw new Error("config missing graylog.username");
  }
  if (!raw.password) {
    throw new Error("config missing graylog.password");
  }

  return {
    baseUrl: trimTrailingSlash(withScheme(raw.base_url)),
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
