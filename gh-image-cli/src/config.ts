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
export function loadConfig(path?: string): AppConfig {
  const resolved = path ?? join(homedir(), ".config", "gh-image-cli", "config.json");
  if (!existsSync(resolved)) {
    if (path !== undefined) {
      throw new Error(`config file not found: ${resolved}`);
    }
    return defaults;
  }

  const raw = JSON.parse(readFileSync(resolved, "utf8")) as Partial<AppConfig>;
  const cfg = {
    ...defaults,
    ...raw,
  };
  if (cfg.timeoutSeconds <= 0 || !Number.isInteger(cfg.timeoutSeconds)) {
    throw new Error("timeoutSeconds must be a positive integer");
  }
  return cfg;
}
