import { expect, test } from "bun:test";
import { parseFileCfg } from "../src/config";

const sample = `{
  "profiles": [
    {
      "name": "buy",
      "kind": "mysql",
      "url": "mysql://readonly@127.0.0.1:3306/buy",
    },
    {
      "name": "dw",
      "kind": "doris",
      "url": "mysql://readonly@127.0.0.1:9030/dw",
      "access": "write",
    }
  ]
}`;

test("parseFileCfg 接受 jsonc, access 默认 read", () => {
  const cfg = parseFileCfg(sample, "/tmp/config.json");
  expect(cfg.profiles).toHaveLength(2);
  expect(cfg.profiles[0]?.access).toBe("read");
  expect(cfg.profiles[1]?.access).toBe("write");
  expect(cfg.profiles[1]?.kind).toBe("doris");
});

test("kind 与 url scheme 必须匹配", () => {
  expect(() =>
    parseFileCfg(
      `{ "profiles": [{ "name": "x", "kind": "mysql", "url": "redis://127.0.0.1:6379/0" }] }`,
      "/tmp/x.json",
    ),
  ).toThrow("mysql://");
});

test("未知 kind 失败", () => {
  expect(() =>
    parseFileCfg(
      `{ "profiles": [{ "name": "x", "kind": "postgres", "url": "mysql://127.0.0.1/db" }] }`,
      "/tmp/x.json",
    ),
  ).toThrow("unknown kind");
});
