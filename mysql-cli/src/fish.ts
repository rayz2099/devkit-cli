import type { MysqlCliConfig } from "./types";

// fish 补全只输出 profile name, 由 shell 自己处理展示和过滤.
export function profileCompletion(config: MysqlCliConfig): string {
  return `${config.profiles.map((profile) => profile.name).join("\n")}\n`;
}
