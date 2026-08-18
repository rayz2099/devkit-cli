import { describe, expect, test } from "bun:test";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { cacheKey, LIST_TTL_MS, readListCache, writeListCache } from "../src/cache";

describe("list cache", () => {
  test("未过期时命中", async () => {
    const path = join(mkdtempSync(join(tmpdir(), "nacos-cache-")), "config-list.json");
    const key = cacheKey("127.0.0.1:8848", "public", "");
    await writeListCache(key, [{ dataId: "app-rpc", group: "COMMON" }], 1_000, path);
    await expect(readListCache(key, 1_000 + LIST_TTL_MS - 1, path)).resolves.toEqual([
      { dataId: "app-rpc", group: "COMMON" },
    ]);
  });

  test("过期后不命中", async () => {
    const path = join(mkdtempSync(join(tmpdir(), "nacos-cache-")), "config-list.json");
    const key = cacheKey("127.0.0.1:8848", "public", "");
    await writeListCache(key, [{ dataId: "app-rpc", group: "COMMON" }], 1_000, path);
    await expect(readListCache(key, 1_000 + LIST_TTL_MS, path)).resolves.toBeUndefined();
  });
});
