import { readFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import type { MysqlCliConfig, MysqlConnection } from "./types";

export const CONFIG_PATH = join(homedir(), ".config", "mysql-cli", "config.json");

// 只接受明确的配置结构, 避免把错误配置静默转成错误连接.
export function parseConfigJson(content: string): MysqlCliConfig {
  const parsed = JSON.parse(content) as unknown;

  if (!parsed || typeof parsed !== "object" || !Array.isArray((parsed as { profiles?: unknown }).profiles)) {
    throw new Error("Invalid config: profiles must be an array");
  }

  const profiles = (parsed as { profiles: unknown[] }).profiles.map((item) => {
    if (!item || typeof item !== "object") {
      throw new Error("Invalid config: profile must be an object");
    }

    const profile = item as { name?: unknown; jdbcUrl?: unknown };
    if (typeof profile.name !== "string" || profile.name.length === 0) {
      throw new Error("Invalid config: profile.name is required");
    }
    if (typeof profile.jdbcUrl !== "string" || profile.jdbcUrl.length === 0) {
      throw new Error(`Invalid config: jdbcUrl is required for profile ${profile.name}`);
    }

    return {
      name: profile.name,
      jdbcUrl: profile.jdbcUrl,
    };
  });

  return { profiles };
}

// JDBC URL 只作为连接资料来源, 其他驱动参数不参与 mysql 客户端调用.
export function parseJdbcUrl(profileName: string, jdbcUrl: string): MysqlConnection {
  const prefix = "jdbc:mysql://";
  if (!jdbcUrl.startsWith(prefix)) {
    throw new Error(`Invalid jdbcUrl for profile ${profileName}`);
  }

  let url: URL;
  try {
    url = new URL(`mysql://${jdbcUrl.slice(prefix.length)}`);
  } catch {
    throw new Error(`Invalid jdbcUrl for profile ${profileName}`);
  }

  if (!url.hostname) {
    throw new Error(`Invalid jdbcUrl for profile ${profileName}: host is required`);
  }

  const database = decodeURIComponent(url.pathname.replace(/^\//, ""));

  return {
    host: url.hostname,
    port: url.port || undefined,
    user: url.searchParams.get("user") ?? undefined,
    password: url.searchParams.get("password") ?? undefined,
    database: database.length > 0 ? database : undefined,
  };
}

// CLI 每次执行都读取最新配置, 让 fish 补全和运行行为保持一致.
export function loadConfig(path = CONFIG_PATH): MysqlCliConfig {
  return parseConfigJson(readFileSync(path, "utf8"));
}

export function findProfile(config: MysqlCliConfig, name: string) {
  const profile = config.profiles.find((item) => item.name === name);
  if (!profile) {
    const names = config.profiles.map((item) => item.name).join(", ");
    throw new Error(`Profile not found: ${name}${names ? `. Available profiles: ${names}` : ""}`);
  }
  return profile;
}
