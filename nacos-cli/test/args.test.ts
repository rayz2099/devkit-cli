import { describe, expect, test } from "bun:test";
import { parseArgs } from "../src/args";

describe("parseArgs", () => {
  test("config get 位置参数默认 group=COMMON", () => {
    expect(parseArgs(["config", "get", "app-rpc"])).toEqual({
      kind: "config-get",
      dataId: "app-rpc",
      group: "COMMON",
      global: { dev: false },
    });
  });

  test("config get 支持 data-id group 位置参数", () => {
    expect(parseArgs(["config", "get", "app-rpc", "COMMON"])).toEqual({
      kind: "config-get",
      dataId: "app-rpc",
      group: "COMMON",
      global: { dev: false },
    });
  });

  test("全局 flags 可插在子命令前后", () => {
    const cmd = parseArgs(["-o", "json", "config", "list", "--namespace", "prepare"]);
    expect(cmd.kind).toBe("config-list");
    if (cmd.kind !== "config-list") {
      throw new Error("unreachable");
    }
    expect(cmd.global.output).toBe("json");
    expect(cmd.global.namespace).toBe("prepare");
    expect(cmd.search).toBe("blur");
    expect(cmd.pageNo).toBe(1);
    expect(cmd.pageSize).toBe(10);
  });

  test("naming instances 默认 healthy-only=true", () => {
    const cmd = parseArgs(["naming", "instances", "--service", "svc1"]);
    expect(cmd).toMatchObject({
      kind: "naming-instances",
      service: "svc1",
      group: "DEFAULT_GROUP",
      clusters: [],
      healthyOnly: true,
    });
  });

  test("naming register 缺 port 报错", () => {
    expect(() => parseArgs(["naming", "register", "--service", "svc", "--ip", "1.1.1.1"])).toThrow(
      "port is required",
    );
  });

  test("search 非法值报错", () => {
    expect(() => parseArgs(["config", "list", "--search", "fuzzy"])).toThrow(
      "search must be one of: accurate, blur",
    );
  });
});
