import { describe, expect, test } from "bun:test";
import { emptyFileCfg, nsCands, parseFileCfg, resolveRuntime } from "../src/config";

describe("resolveRuntime", () => {
  test("优先级 flags > env > file > default", () => {
    const fileCfg = parseFileCfg(
      JSON.stringify({
        nacos_server_addr: "file:8848",
        nacos_namespace: "file-ns",
        namespaces: ["prepare", "online"],
        nacos_output: "text",
      }),
      "/tmp/config.json",
    );

    const runtime = resolveRuntime(
      { serverAddr: "flag:8848", dev: false },
      fileCfg,
      { nacos_namespace: "env-ns", NACOS_OUTPUT: "json" },
    );

    expect(runtime.serverAddr).toBe("flag:8848");
    expect(runtime.namespace).toBe("env-ns");
    expect(runtime.output).toBe("json");
  });

  test("namespace 可回落到 namespaces 第一项", () => {
    const fileCfg = emptyFileCfg();
    fileCfg.namespaces = ["prepare", "online"];
    const runtime = resolveRuntime({ dev: false }, fileCfg, {});
    expect(runtime.namespace).toBe("prepare");
    expect(runtime.serverAddr).toBe("127.0.0.1:8848");
  });

  test("nsCands 去重并补 public", () => {
    expect(nsCands(parseFileCfg(
      JSON.stringify({ namespaces: ["prepare", "public"] }),
      "/tmp/config.json",
    ))).toEqual(["prepare", "public"]);
  });
});
