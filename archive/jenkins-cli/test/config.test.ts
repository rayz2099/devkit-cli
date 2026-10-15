import { expect, test } from "bun:test";
import { parseFileCfg, resolveSecret, resolveRuntime } from "../src/config";

const sample = `{
  "defaultProfile": "prod",
  "profiles": [
    {
      "name": "prod",
      "url": "http://10.0.0.1:8080/",
      "username": "admin",
      "password": "pass",
      "apiToken": "token",
    }
  ]
}`;

test("parseFileCfg 接受 trailing comma", () => {
  const cfg = parseFileCfg(sample, "/tmp/config.json");
  expect(cfg.defaultProfile).toBe("prod");
  expect(cfg.profiles[0]?.url).toBe("http://10.0.0.1:8080");
  expect(cfg.profiles[0]?.apiToken).toBe("token");
});

test("resolveSecret 优先 apiToken", () => {
  expect(resolveSecret({
    name: "prod",
    url: "http://x",
    username: "u",
    password: "pass",
    apiToken: "token",
  })).toBe("token");
  expect(resolveSecret({
    name: "prod",
    url: "http://x",
    username: "u",
    password: "pass",
    apiToken: "",
  })).toBe("pass");
});

test("两空 Secret 失败", () => {
  expect(() => resolveSecret({
    name: "prod",
    url: "http://x",
    username: "u",
    password: "",
    apiToken: "",
  })).toThrow("empty apiToken and password");
});

test("resolveRuntime 按 defaultProfile", () => {
  const runtime = resolveRuntime(parseFileCfg(sample, "/tmp/config.json"), "human");
  expect(runtime.profile.name).toBe("prod");
  expect(runtime.secret).toBe("token");
});
