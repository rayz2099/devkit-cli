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

test("kind 与 url 对不上的 profile 跳过", () => {
  const cfg = parseFileCfg(
    `{ "profiles": [{ "name": "x", "kind": "mysql", "url": "redis://127.0.0.1:6379/0" }] }`,
    "/tmp/x.json",
  );
  expect(cfg.profiles).toHaveLength(0);
});

test("未知 kind 跳过, 不影响其它 profile", () => {
  const cfg = parseFileCfg(
    `{
      "profiles": [
        { "name": "pg-it", "kind": "postgres", "url": "postgres://127.0.0.1/db" },
        { "name": "kf-biz", "kind": "kafka", "url": "kafka://127.0.0.1:9092" }
      ]
    }`,
    "/tmp/x.json",
  );
  expect(cfg.profiles).toHaveLength(1);
  expect(cfg.profiles[0]?.name).toBe("kf-biz");
});

test("kafka url 必须是 kafka:// 或 kafkas://", () => {
  const cfg = parseFileCfg(
    `{ "profiles": [{ "name": "kf-biz", "kind": "kafka", "url": "kafka://127.0.0.1:9092" }] }`,
    "/tmp/k.json",
  );
  expect(cfg.profiles[0]?.kind).toBe("kafka");
  const skipped = parseFileCfg(
    `{ "profiles": [{ "name": "x", "kind": "kafka", "url": "http://127.0.0.1:9092" }] }`,
    "/tmp/k.json",
  );
  expect(skipped.profiles).toHaveLength(0);
});
