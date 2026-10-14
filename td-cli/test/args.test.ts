import { expect, test } from "bun:test";
import { parseArgs } from "../src/args";

test("默认 human, agent 是前缀", () => {
  const human = parseArgs(["project", "ls"]);
  expect(human).toMatchObject({ kind: "project-ls", audience: "human" });
  const agent = parseArgs(["agent", "project", "ls"]);
  expect(agent).toMatchObject({ kind: "project-ls", audience: "agent" });
});

test("写入命令直接拒绝", () => {
  expect(() => parseArgs(["report", "save"])).toThrow("read-only");
  expect(() => parseArgs(["query", "download"])).toThrow("read-only");
});

test("事件分析默认口径跟页面一致", () => {
  expect(parseArgs(["query", "event", "--project", "3"])).toMatchObject({
    kind: "query-event",
    project: "3",
    eventName: "anyEvent",
    analysis: "A100",
    particle: "T1",
    timeoutMs: 120_000,
  });
});

test("--qp 不能再带其它查询条件", () => {
  expect(() => parseArgs(["query", "event", "--project", "1", "--qp", "a.json", "--event", "login"]))
    .toThrow("--qp replaces other query flags");
});

test("-p 可插在前面", () => {
  expect(parseArgs(["-p", "local", "auth", "status"])).toMatchObject({
    kind: "auth-status",
    profile: "local",
  });
});
