import { describe, expect, test } from "bun:test";
import { NacosClient, parseAddress } from "../src/client";
import type { Runtime } from "../src/types";

const runtime: Runtime = {
  serverAddr: "http://127.0.0.1:8848",
  username: "",
  password: "",
  namespace: "public",
  output: "text",
  dev: false,
};

describe("parseAddress", () => {
  test("解析带 http 前缀的地址", () => {
    expect(parseAddress("http://127.0.0.1:8848")).toEqual({
      scheme: "http",
      host: "127.0.0.1",
      port: 8848,
    });
  });

  test("https 无端口默认 8848", () => {
    expect(parseAddress("https://127.0.0.1")).toEqual({
      scheme: "https",
      host: "127.0.0.1",
      port: 8848,
    });
  });
});

describe("NacosClient", () => {
  test("getConfig 走 v1 OpenAPI 并带 tenant", async () => {
    const seen: string[] = [];
    const client = new NacosClient(runtime, async (input) => {
      const url = new URL(String(input));
      seen.push(`${url.pathname}?${url.searchParams.toString()}`);
      return new Response("content", { status: 200 });
    });
    await expect(client.getConfig("app-rpc", "COMMON")).resolves.toBe("content");
    expect(seen[0]).toContain("/nacos/v1/cs/configs");
    expect(seen[0]).toContain("dataId=app-rpc");
    expect(seen[0]).toContain("group=COMMON");
    expect(seen[0]).toContain("tenant=public");
  });

  test("有用户名时先 login 再带 accessToken", async () => {
    const paths: string[] = [];
    const client = new NacosClient(
      { ...runtime, username: "nacos", password: "nacos" },
      async (input) => {
        const url = new URL(String(input));
        paths.push(url.pathname);
        if (url.pathname.endsWith("/login")) {
          return new Response(JSON.stringify({ accessToken: "tok" }), { status: 200 });
        }
        if (url.searchParams.get("accessToken") !== "tok") {
          return new Response("missing token", { status: 403 });
        }
        return new Response("true", { status: 200 });
      },
    );
    await client.putConfig("d1", "g1", "v1");
    expect(paths).toEqual(["/nacos/v1/auth/login", "/nacos/v1/cs/configs"]);
  });

  test("listInsts 解析 hosts", async () => {
    const client = new NacosClient(runtime, async () =>
      new Response(JSON.stringify({
        hosts: [{ ip: "1.1.1.1", port: 8080 }],
      }), { status: 200 }),
    );
    await expect(client.listInsts({
      service: "svc1",
      group: "DEFAULT_GROUP",
      clusters: [],
      healthyOnly: true,
    })).resolves.toEqual([{
      ip: "1.1.1.1",
      port: 8080,
      weight: undefined,
      healthy: undefined,
      enabled: undefined,
      ephemeral: undefined,
      clusterName: undefined,
      serviceName: undefined,
      metadata: undefined,
    }]);
  });
});
