import { afterEach, describe, expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { parseArgs } from "../src/args";
import type { NacosClient } from "../src/client";
import { runCmd } from "../src/run";

const homes: string[] = [];

afterEach(() => {
  const prev = process.env.HOME;
  for (const dir of homes) {
    rmSync(dir, { recursive: true, force: true });
  }
  homes.length = 0;
  if (prev !== undefined) {
    process.env.HOME = prev;
  }
});

function withHome(config: unknown): void {
  const home = mkdtempSync(join(tmpdir(), "nacos-cli-"));
  homes.push(home);
  mkdirSync(join(home, ".config", "nacos-cli"), { recursive: true });
  writeFileSync(join(home, ".config", "nacos-cli", "config.json"), JSON.stringify(config));
  process.env.HOME = home;
}

describe("runCmd", () => {
  test("config get text 输出内容", async () => {
    withHome({});
    const out = await runCmd(parseArgs(["config", "get", "app-rpc"]), () => ({
      getConfig: async (dataId: string, group: string) => {
        expect(dataId).toBe("app-rpc");
        expect(group).toBe("COMMON");
        return "ok";
      },
    }) as unknown as NacosClient);
    expect(out).toBe("ok\n");
  });

  test("config list text 表格", async () => {
    withHome({});
    const out = await runCmd(parseArgs(["config", "list"]), () => ({
      listConfigs: async () => ({
        totalCount: 2,
        pageNumber: 1,
        pagesAvailable: 1,
        pageItems: [
          { dataId: "d1", group: "g1" },
          { dataId: "d2", group: "g2" },
        ],
      }),
    }) as unknown as NacosClient);
    expect(out).toContain("Total: 2  Page: 1/1");
    expect(out).toContain("DATA_ID");
    expect(out).toContain("d1");
  });

  test("naming instances text", async () => {
    withHome({});
    const out = await runCmd(parseArgs(["naming", "instances", "--service", "svc1"]), () => ({
      listInsts: async () => [{ ip: "1.1.1.1", port: 8080 }],
    }) as unknown as NacosClient);
    expect(out).toContain("count: 1");
    expect(out).toContain("1.1.1.1:8080");
  });
});
