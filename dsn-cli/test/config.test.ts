import { expect, test } from "bun:test";
import { dsOut, parseFileCfg } from "../src/config";

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
        { "name": "pg-it", "kind": "oracle", "url": "oracle://127.0.0.1/db" },
        { "name": "kf-biz", "kind": "kafka", "url": "kafka://127.0.0.1:9092" }
      ]
    }`,
    "/tmp/x.json",
  );
  expect(cfg.profiles).toHaveLength(1);
  expect(cfg.profiles[0]?.name).toBe("kf-biz");
});

test("postgres url 接受 postgres:// 和 postgresql://", () => {
  const cfg = parseFileCfg(
    `{ "profiles": [
      { "name": "pg", "kind": "postgres", "url": "postgres://readonly@127.0.0.1:5432/app" },
      { "name": "pg-iana", "kind": "postgres", "url": "postgresql://readonly@127.0.0.1:5432/app" }
    ] }`,
    "/tmp/pg.json",
  );
  expect(cfg.profiles).toHaveLength(2);
  expect(cfg.profiles[0]?.kind).toBe("postgres");
  const skipped = parseFileCfg(
    `{ "profiles": [{ "name": "x", "kind": "postgres", "url": "mysql://127.0.0.1:5432/db" }] }`,
    "/tmp/pg.json",
  );
  expect(skipped.profiles).toHaveLength(0);
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

test("description 可选, 空字符串当没有", () => {
  const cfg = parseFileCfg(
    `{ "profiles": [
      { "name": "buy", "kind": "mysql", "url": "mysql://readonly@127.0.0.1:3306/buy", "description": "交易库只读" },
      { "name": "dw", "kind": "doris", "url": "mysql://readonly@127.0.0.1:9030/dw", "description": "  " }
    ] }`,
    "/tmp/desc.json",
  );
  expect(cfg.profiles[0]?.description).toBe("交易库只读");
  expect(cfg.profiles[1]?.description).toBeUndefined();
});

test("description 非字符串则跳过该 profile", () => {
  const cfg = parseFileCfg(
    `{ "profiles": [
      { "name": "buy", "kind": "mysql", "url": "mysql://readonly@127.0.0.1:3306/buy", "description": 1 },
      { "name": "kf-biz", "kind": "kafka", "url": "kafka://127.0.0.1:9092" }
    ] }`,
    "/tmp/desc.json",
  );
  expect(cfg.profiles).toHaveLength(1);
  expect(cfg.profiles[0]?.name).toBe("kf-biz");
});

test("mongodb 非 mongodb:// 仍跳过", () => {
  const cfg = parseFileCfg(
    `{ "profiles": [{ "name": "x", "kind": "mongodb", "url": "redis://127.0.0.1:6379/0" }] }`,
    "/tmp/mongo.json",
  );
  expect(cfg.profiles).toHaveLength(0);
});

test("mongodb 副本集 seed list 是合法 url", () => {
  const cfg = parseFileCfg(
    `{ "profiles": [
      { "name": "rs-biz", "kind": "mongodb",
        "url": "mongodb://biz:secret@10.0.16.60:27017,10.0.16.61:27017,10.0.16.62:27017?readPreference=secondary" },
      { "name": "rs-meta", "kind": "mongodb",
        "url": "mongodb://biz_read:secret@10.0.16.70:27017,10.0.16.71:27017,10.0.16.72:27017?authSource=admin&readPreference=secondary" }
    ] }`,
    "/tmp/mongo.json",
  );
  expect(cfg.profiles).toHaveLength(2);
  expect(cfg.profiles[0]?.name).toBe("rs-biz");
  expect(cfg.profiles[1]?.name).toBe("rs-meta");
});

test("dsOut 不包含 url", () => {
  const cfg = parseFileCfg(
    `{ "profiles": [{ "name": "buy", "kind": "mysql", "url": "mysql://secret@127.0.0.1:3306/buy", "description": "交易库只读" }] }`,
    "/tmp/desc.json",
  );
  expect(dsOut(cfg.profiles)).toEqual({
    columns: ["name", "kind", "access", "description"],
    rows: [{ name: "buy", kind: "mysql", access: "read", description: "交易库只读" }],
  });
});
