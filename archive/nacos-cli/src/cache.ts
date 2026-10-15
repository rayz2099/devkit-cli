import { mkdirSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import type { ConfigItem } from "./types";

export const LIST_TTL_MS = 10_000;

type CacheEnt = {
  expire: number;
  items: ConfigItem[];
};

type CacheFile = {
  entries: Record<string, CacheEnt>;
};

/** 为什么: fish 每次 Tab 都是新进程, 内存 cache 无效, 必须落到 XDG cache 才能对齐 Go 的 10s 复用. */
export function listCachePath(home = homedir()): string {
  return join(home, ".config", "nacos-cli", "cache", "config-list.json");
}

export function cacheKey(
  serverAddr: string,
  namespace: string,
  username: string,
): string {
  return `${serverAddr}|${namespace}|${username}`;
}

export async function readListCache(
  key: string,
  now = Date.now(),
  path = listCachePath(),
): Promise<ConfigItem[] | undefined> {
  const file = Bun.file(path);
  if (!(await file.exists())) {
    return undefined;
  }

  let parsed: CacheFile;
  try {
    parsed = (await file.json()) as CacheFile;
  } catch {
    return undefined;
  }
  if (!parsed || typeof parsed !== "object" || parsed.entries === undefined) {
    return undefined;
  }

  const entry = parsed.entries[key];
  if (entry === undefined || now >= entry.expire || !Array.isArray(entry.items)) {
    return undefined;
  }
  return entry.items.map((item) => ({ dataId: item.dataId, group: item.group }));
}

export async function writeListCache(
  key: string,
  items: ConfigItem[],
  now = Date.now(),
  path = listCachePath(),
): Promise<void> {
  mkdirSync(join(path, ".."), { recursive: true });

  let parsed: CacheFile = { entries: {} };
  const file = Bun.file(path);
  if (await file.exists()) {
    try {
      const current = (await file.json()) as CacheFile;
      if (current && typeof current === "object" && current.entries) {
        parsed = current;
      }
    } catch {
      parsed = { entries: {} };
    }
  }

  const next: CacheFile = { entries: {} };
  for (const [entryKey, entry] of Object.entries(parsed.entries)) {
    if (entry.expire > now) {
      next.entries[entryKey] = entry;
    }
  }
  next.entries[key] = {
    expire: now + LIST_TTL_MS,
    items: items.map((item) => ({ dataId: item.dataId, group: item.group })),
  };
  await Bun.write(path, `${JSON.stringify(next)}\n`);
}
