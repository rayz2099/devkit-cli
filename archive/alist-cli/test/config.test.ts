import { expect, test } from "bun:test";
import { emptyFileCfg, parseFileCfg, resolveRuntime } from "../src/config";

const sample = `{
  "defaultProfile": "home",
  "profiles": [
    {
      "name": "home",
      "url": "http://alist.example/",
      "username": "admin",
      "password": "secret",
    }
  ]
}`;

test("parseFileCfg 接受 trailing comma 并去掉 url 尾斜杠", () => {
  const cfg = parseFileCfg(sample, "/tmp/config.json");
  expect(cfg.defaultProfile).toBe("home");
  expect(cfg.profiles[0]?.url).toBe("http://alist.example");
  expect(cfg.profiles[0]?.username).toBe("admin");
});

test("file 允许空 password, 无 env 时才失败", () => {
  const cfg = parseFileCfg(`{
    "defaultProfile": "home",
    "profiles": [{ "name": "home", "url": "http://alist.example", "username": "admin", "password": "" }]
  }`, "/tmp/config.json");
  expect(cfg.profiles[0]?.password).toBe("");
  expect(() => resolveRuntime(cfg, "human", undefined, {})).toThrow("ALIST_PASSWORD is required");
});

test("resolveRuntime 选出 default profile", () => {
  const runtime = resolveRuntime(
    parseFileCfg(sample, "/tmp/config.json"),
    "agent",
    undefined,
    {},
  );
  expect(runtime.audience).toBe("agent");
  expect(runtime.profile.name).toBe("home");
  expect(runtime.profile.url).toBe("http://alist.example");
});

test("profile 不存在失败", () => {
  const cfg = parseFileCfg(sample, "/tmp/config.json");
  expect(() => resolveRuntime(cfg, "human", "prod", {})).toThrow("profile not found: prod");
});

test("ALIST_* 覆盖 file", () => {
  const runtime = resolveRuntime(
    parseFileCfg(sample, "/tmp/config.json"),
    "human",
    undefined,
    {
      ALIST_ADDRESS: "http://from-env/",
      ALIST_USERNAME: "env-user",
      ALIST_PASSWORD: "env-pass",
    },
  );
  expect(runtime.profile).toEqual({
    name: "home",
    url: "http://from-env",
    username: "env-user",
    password: "env-pass",
  });
});

test("无 file 时只读 ALIST_*", () => {
  const runtime = resolveRuntime(
    emptyFileCfg(),
    "agent",
    undefined,
    {
      ALIST_ADDRESS: "http://from-env",
      ALIST_USERNAME: "env-user",
      ALIST_PASSWORD: "env-pass",
    },
  );
  expect(runtime.profile).toEqual({
    name: "env",
    url: "http://from-env",
    username: "env-user",
    password: "env-pass",
  });
});

test("无 file 且缺 ALIST_ADDRESS 失败", () => {
  expect(() => resolveRuntime(
    emptyFileCfg(),
    "human",
    undefined,
    {
      ALIST_USERNAME: "env-user",
      ALIST_PASSWORD: "env-pass",
    },
  )).toThrow("ALIST_ADDRESS is required");
});
