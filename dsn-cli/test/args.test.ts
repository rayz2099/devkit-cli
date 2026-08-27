import { expect, test } from "bun:test";
import { parseArgs } from "../src/args";

test("无参数是 help", () => {
  expect(parseArgs([]).kind).toBe("help");
});

test("-p 无 query 是 Console", () => {
  expect(parseArgs(["-p", "buy"])).toEqual({
    kind: "console",
    audience: "human",
    profile: "buy",
  });
});

test("query 需要 -p 和单参数语句", () => {
  expect(parseArgs(["-p", "buy", "query", "SELECT 1"])).toEqual({
    kind: "query",
    audience: "human",
    profile: "buy",
    stmt: "SELECT 1",
    output: "table",
    limit: undefined,
    connectSec: 3,
    execSec: 30,
  });
  expect(() => parseArgs(["query", "SELECT 1"])).toThrow("-p is required");
  expect(() => parseArgs(["-p", "buy", "query", "SELECT", "1"])).toThrow(
    "single argument",
  );
});

test("agent 前缀和 --output --limit", () => {
  expect(parseArgs(["agent", "-p", "buy", "query", "PING", "--limit", "10"])).toEqual({
    kind: "query",
    audience: "agent",
    profile: "buy",
    stmt: "PING",
    output: "table",
    limit: 10,
    connectSec: 3,
    execSec: 30,
  });
  expect(parseArgs(["-p", "buy", "query", "SELECT 1", "--output", "json"]).kind).toBe(
    "query",
  );
});

test("--timeout 覆盖执行秒数", () => {
  const cmd = parseArgs(["-p", "buy", "query", "SELECT 1", "--timeout", "120", "--connect-timeout", "2"]);
  expect(cmd).toMatchObject({ kind: "query", execSec: 120, connectSec: 2 });
});

test("doctor 不需要 -p, 默认连接 3s", () => {
  expect(parseArgs(["doctor"])).toEqual({
    kind: "doctor",
    audience: "human",
    profile: undefined,
    output: "table",
    connectSec: 3,
    execSec: 5,
  });
  expect(parseArgs(["agent", "doctor", "-p", "buy", "--timeout", "8"])).toEqual({
    kind: "doctor",
    audience: "agent",
    profile: "buy",
    output: "table",
    connectSec: 3,
    execSec: 8,
  });
  expect(() => parseArgs(["doctor", "extra"])).toThrow("unexpected argument");
});
