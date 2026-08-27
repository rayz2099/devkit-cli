import { expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { decodeBytes, pickNewest, type KafkaRec } from "../src/kafka";
import { readTopicCache, writeTopicCache } from "../src/kafka-cache";
import { kafkaComplete, parseKafkaStmt } from "../src/kafka-stmt";

test("parseKafkaStmt 小写头和默认 n", () => {
  expect(parseKafkaStmt("topics")).toEqual({ head: "topics" });
  expect(parseKafkaStmt("peek t")).toEqual({ head: "peek", topic: "t", n: 50, partition: undefined });
  expect(parseKafkaStmt("peek t 3 partition 1")).toEqual({
    head: "peek",
    topic: "t",
    n: 3,
    partition: 1,
  });
  expect(parseKafkaStmt("listen t partition 0")).toEqual({
    head: "listen",
    topic: "t",
    partition: 0,
  });
  expect(() => parseKafkaStmt("PEEK t")).toThrow("PEEK");
  expect(() => parseKafkaStmt("peek t 501")).toThrow("1..500");
});

test("kafkaComplete 在 peek/listen 后补 topic", () => {
  expect(kafkaComplete([], ["a", "b"])).toEqual(["topics", "peek", "listen"]);
  expect(kafkaComplete(["peek"], ["a", "b"])).toEqual(["a", "b"]);
  expect(kafkaComplete(["listen"], ["a"])).toEqual(["a"]);
  expect(kafkaComplete(["topics"], ["a"])).toEqual([]);
  expect(kafkaComplete(["peek", "a"], ["a", "b"])).toEqual([]);
});

test("pickNewest 按时间取全局最后 n 条, 保持时间顺序", () => {
  const rows: KafkaRec[] = [
    rec(0, "1", 10),
    rec(1, "9", 50),
    rec(0, "2", 20),
    rec(1, "8", 40),
  ];
  expect(pickNewest(rows, 2).map((item) => item.offset)).toEqual(["8", "9"]);
});

test("decodeBytes json / utf8 / 二进制", () => {
  expect(decodeBytes(null)).toBeNull();
  expect(decodeBytes(Buffer.from('{"a":1}'))).toEqual({ a: 1 });
  expect(decodeBytes(Buffer.from("hello"))).toBe("hello");
  expect(decodeBytes(Buffer.from([0xff, 0xfe, 0x00]))).toBe(Buffer.from([0xff, 0xfe, 0x00]).toString("base64"));
});

test("topic cache 按 profile 读写", async () => {
  const home = await mkdtemp(join(tmpdir(), "dsn-cache-"));
  try {
    await writeTopicCache("kf-biz", ["album_audit_log", "blog_core_1"], home);
    expect(await readTopicCache("kf-biz", home)).toEqual(["album_audit_log", "blog_core_1"]);
    expect(await readTopicCache("other", home)).toEqual([]);
  } finally {
    await rm(home, { recursive: true, force: true });
  }
});

function rec(partition: number, offset: string, timestamp: number): KafkaRec {
  return { partition, offset, timestamp, key: null, value: null, headers: {} };
}
