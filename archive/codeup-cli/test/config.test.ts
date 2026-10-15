import { expect, test } from "bun:test";
import { parseFileCfg, resolveRuntime } from "../src/config";

const sample = `{
  "defaultProfile": "work",
  "profiles": [
    {
      "name": "work",
      "url": "https://codeup.aliyun.com/org-example/",
      "token": "tok",
    }
  ]
}`;

test("parseFileCfg 接受 trailing comma 并去掉 url 尾斜杠", () => {
  const cfg = parseFileCfg(sample, "/tmp/config.json");
  expect(cfg.defaultProfile).toBe("work");
  expect(cfg.profiles[0]?.url).toBe("https://codeup.aliyun.com/org-example");
  expect(cfg.profiles[0]?.token).toBe("tok");
});

test("缺 token 失败", () => {
  expect(() => parseFileCfg(`{
    "defaultProfile": "work",
    "profiles": [{ "name": "work", "url": "https://codeup.aliyun.com/org-example", "token": "" }]
  }`, "/tmp/config.json")).toThrow("token is required");
});

test("resolveRuntime 解析 Organization", () => {
  const runtime = resolveRuntime(parseFileCfg(sample, "/tmp/config.json"), "agent");
  expect(runtime.audience).toBe("agent");
  expect(runtime.orgId).toBe("org-example");
  expect(runtime.apiBase).toBe("https://openapi-rdc.aliyuncs.com");
});
