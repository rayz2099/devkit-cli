import { normalizePrometheusConfig } from "../prometheus/config";
import { normalizeGraylogConfig } from "../graylog/config";
import type { GraylogConfig, RawGraylogConfig } from "../graylog/types";
import type { PrometheusConfig, RawPrometheusConfig } from "../prometheus/types";
import type { UptraceConfig } from "./types";
import { homedir } from "node:os";

export interface AppConfig {
  uptrace?: UptraceConfig | undefined;
  prometheus: PrometheusConfig;
  graylog?: GraylogConfig | undefined;
}

interface RawConfig {
  uptrace?: {
    base_url?: string;
    web_base_url?: string;
    project_id?: number;
    auth_token?: string;
    jwt_token?: string;
    default_env?: string;
    default_time_dur_seconds?: number;
  };
  prometheus?: RawPrometheusConfig;
  graylog?: RawGraylogConfig;
}

/** 为什么：CLI 配置必须显式校验，鉴权缺失时应直接失败。 */
export async function loadConfig(path?: string): Promise<AppConfig> {
  const configPath = path ?? defaultConfigPath();
  const file = Bun.file(configPath);
  if (!(await file.exists())) {
    throw new Error(`config file not found: ${configPath}`);
  }

  return normalizeRawConfig((await file.json()) as RawConfig);
}

/** 为什么：Prometheus 有内置默认 endpoint，默认配置文件缺失不应阻塞零配置查询。 */
export async function loadConfigIfExists(path?: string): Promise<AppConfig | undefined> {
  const configPath = path ?? defaultConfigPath();
  const file = Bun.file(configPath);
  if (!(await file.exists())) {
    if (path) {
      throw new Error(`config file not found: ${configPath}`);
    }
    return undefined;
  }

  return normalizeRawConfig((await file.json()) as RawConfig);
}

/** 为什么：配置校验独立出来，方便测试 jwt cookie 鉴权和后续扩展配置来源。 */
export function normalizeRawConfig(raw: RawConfig): AppConfig {
  return {
    uptrace: raw.uptrace ? normalizeUptraceConfig(raw.uptrace) : undefined,
    prometheus: normalizePrometheusConfig(raw.prometheus),
    graylog: raw.graylog ? normalizeGraylogConfig(raw.graylog) : undefined,
  };
}

export function requireUptraceConfig(config: AppConfig): UptraceConfig {
  if (!config.uptrace) {
    throw new Error("config missing uptrace section");
  }
  return config.uptrace;
}

export function requireGraylogConfig(config: AppConfig): GraylogConfig {
  if (!config.graylog) {
    throw new Error("config missing graylog section");
  }
  return config.graylog;
}

function normalizeUptraceConfig(raw: NonNullable<RawConfig["uptrace"]>): UptraceConfig {
  if (!raw.base_url) {
    throw new Error("config missing uptrace.base_url");
  }
  if (!raw.project_id) {
    throw new Error("config missing uptrace.project_id");
  }
  if (!raw.auth_token && !raw.jwt_token) {
    throw new Error("config missing uptrace auth: expected auth_token or jwt_token");
  }

  return {
    baseUrl: raw.base_url,
    webBaseUrl: raw.web_base_url,
    projectId: raw.project_id,
    authToken: raw.auth_token,
    jwtToken: raw.jwt_token,
    defaultEnv: raw.default_env,
    defaultTimeDurSeconds: raw.default_time_dur_seconds,
  };
}

function defaultConfigPath(): string {
  return `${homedir()}/.config/olly-cli/config.json`;
}
