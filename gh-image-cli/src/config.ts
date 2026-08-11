import { existsSync, readFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import type { AppConfig } from "./types";

const defaults: AppConfig = {
  registry: "registry.cn-shanghai.aliyuncs.com",
  namespace: "linran-pub",
  timeoutSeconds: 600,
};

/**
 * 用户级配置只覆盖稳定的基础设施参数, 项目状态统一归 images.yaml 所有.
 */
export function loadConfig(): AppConfig {
  const path = join(homedir(), ".config", "gh-image-cli", "config.json");
  if (!existsSync(path)) {
    return defaults;
  }

  const raw = JSON.parse(readFileSync(path, "utf8")) as Partial<AppConfig>;
  const cfg = {
    ...defaults,
    ...raw,
  };
  if (cfg.timeoutSeconds <= 0 || !Number.isInteger(cfg.timeoutSeconds)) {
    throw new Error("timeoutSeconds must be a positive integer");
  }
  return cfg;
}
