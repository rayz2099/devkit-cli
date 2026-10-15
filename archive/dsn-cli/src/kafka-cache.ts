import { mkdir } from "node:fs/promises";
import { dirname, join } from "node:path";

/** 为什么: 补全不能每次打 metadata, 缓存必须按 Profile 分开, 两个 kafka Profile 不是一个集群. */
export function topicCachePath(profile: string, home = process.env.HOME): string {
  if (home === undefined || home === "") {
    throw new Error("HOME is required");
  }
  const safe = profile.replace(/[^A-Za-z0-9._-]/g, "_");
  return join(home, ".cache", "dsn-cli", safe, "topics.json");
}

export async function readTopicCache(
  profile: string,
  home = process.env.HOME,
): Promise<string[]> {
  const path = topicCachePath(profile, home);
  const file = Bun.file(path);
  if (!(await file.exists())) {
    return [];
  }
  const parsed = JSON.parse(await file.text()) as { topics?: unknown };
  if (!Array.isArray(parsed.topics)) {
    throw new Error(`invalid topic cache ${path}`);
  }
  return parsed.topics.filter((item): item is string => typeof item === "string");
}

export async function writeTopicCache(
  profile: string,
  topics: string[],
  home = process.env.HOME,
): Promise<void> {
  const path = topicCachePath(profile, home);
  await mkdir(dirname(path), { recursive: true });
  const body = `${JSON.stringify({ topics }, null, 2)}\n`;
  await Bun.write(path, body);
}
