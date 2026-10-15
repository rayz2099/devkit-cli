import { afterEach, describe, expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { parseArgs } from "../src/args";
import type { NacosClient } from "../src/client";
import { runCmd } from "../src/run";

const homes: string[] = [];
const prevHome = process.env.HOME;

afterEach(() => {
  for (const dir of homes) {
    rmSync(dir, { recursive: true, force: true });
  }
  homes.length = 0;
  if (prevHome !== undefined) {
    process.env.HOME = prevHome;
  }
});

function withHome(config: unknown): void {
  const home = mkdtempSync(join(tmpdir(), "nacos-cli-complete-"));
  homes.push(home);
  mkdirSync(join(home, ".config", "nacos-cli"), { recursive: true });
  writeFileSync(join(home, ".config", "nacos-cli", "config.json"), JSON.stringify(config));
  process.env.HOME = home;
}

function mockClient(): NacosClient {
  return {
    listItemsCached: async () => [
      { dataId: "app-rpc", group: "COMMON" },
      { dataId: "app-infra", group: "COMMON" },
      { dataId: "biz-kv", group: "COMMON" },
    ],
  } as unknown as NacosClient;
}

describe("complete config get", () => {
  test("未选 namespace 时先补配置里的空间", async () => {
    withHome({
      namespaces: [
        "prepare_hwc",
        "online_hwc",
        "job_hwc",
        "loadtest",
        "runtime",
        "it_hwc",
        "public",
      ],
    });
    const out = await runCmd(
      parseArgs(["__complete", "config", "get", "--to-complete", ""]),
      mockClient,
    );
    expect(out).toBe("-ns\n:4\n");
  });

  test("选定 namespace 后补该空间的 dataId", async () => {
    withHome({ namespaces: ["prepare_hwc", "online_hwc"] });
    let namespace = "";
    const out = await runCmd(
      parseArgs([
        "__complete",
        "config",
        "get",
        "-ns",
        "online_hwc",
        "--to-complete",
        "app",
      ]),
      (runtime) => {
        namespace = runtime.namespace;
        return mockClient();
      },
    );
    expect(namespace).toBe("online_hwc");
    expect(out).toBe("app-rpc\napp-infra\n:4\n");
  });

  test("选定 namespace 后按前缀补 dataId", async () => {
    withHome({ namespaces: ["prepare"] });
    const out = await runCmd(
      parseArgs([
        "__complete",
        "config",
        "get",
        "-ns",
        "prepare",
        "--to-complete",
        "app",
      ]),
      mockClient,
    );
    expect(out).toBe("app-rpc\napp-infra\n:4\n");
  });

  test("cobra 风格 __complete 识别 namespace", async () => {
    withHome({});
    const out = await runCmd(
      parseArgs(["__complete", "config", "get", "-ns", "public", "app"]),
      mockClient,
    );
    expect(out).toBe("app-rpc\napp-infra\n:4\n");
  });

  test("config get app-rpc CO 补 group", async () => {
    withHome({});
    const out = await runCmd(
      parseArgs(["__complete", "config", "get", "app-rpc", "--to-complete", "CO"]),
      mockClient,
    );
    expect(out).toBe("COMMON\n:4\n");
  });

  test("--namespace 补配置里的空间", async () => {
    withHome({ namespaces: ["prepare", "online"] });
    const out = await runCmd(
      parseArgs(["__complete", "--namespace", "--to-complete", "pre"]),
      mockClient,
    );
    expect(out).toBe("prepare\n:4\n");
  });

  test("-ns 补配置里的空间", async () => {
    withHome({ namespaces: ["prepare", "runtime"] });
    const out = await runCmd(
      parseArgs(["__complete", "config", "get", "-ns", "--to-complete", "run"]),
      mockClient,
    );
    expect(out).toBe("runtime\n:4\n");
  });
});
