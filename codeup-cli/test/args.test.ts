import { expect, test } from "bun:test";
import { parseArgs } from "../src/args";

test("默认 human, agent 是前缀", () => {
  const human = parseArgs(["repos"]);
  expect(human).toMatchObject({ kind: "repos", audience: "human", page: 1, perPage: 50 });
  const agent = parseArgs(["agent", "cr", "list"]);
  expect(agent).toMatchObject({ kind: "cr-list", audience: "agent" });
});

test("-p 可插在前面", () => {
  expect(parseArgs(["-p", "work", "webhook", "list", "--show-secrets"])).toMatchObject({
    kind: "webhook-list",
    profile: "work",
    showSecrets: true,
  });
});

test("push 默认 origin, 位置参数只当 branch", () => {
  expect(parseArgs(["push"])).toMatchObject({ kind: "push", remote: "origin" });
  expect(parseArgs(["push", "feature/x"])).toMatchObject({
    kind: "push",
    remote: "origin",
    branch: "feature/x",
  });
  expect(parseArgs(["push", "--remote", "pub", "feature/x"])).toMatchObject({
    kind: "push",
    remote: "pub",
    branch: "feature/x",
  });
});

test("cr create 缺 source/title 失败", () => {
  expect(() => parseArgs(["cr", "create", "--title", "x"])).toThrow("missing --source");
  expect(() => parseArgs(["cr", "create", "--source", "x"])).toThrow("missing --title");
});

test("未知命令失败", () => {
  expect(() => parseArgs(["merge"])).toThrow("unknown command");
});

test("init 是根命令", () => {
  expect(parseArgs(["init"])).toMatchObject({ kind: "init", audience: "human" });
  expect(parseArgs(["agent", "init"])).toMatchObject({ kind: "init", audience: "agent" });
});

test("cr list 默认 opened", () => {
  expect(parseArgs(["cr", "list"])).toMatchObject({ kind: "cr-list", state: "opened" });
  expect(parseArgs(["cr", "list", "--state", "all"])).toMatchObject({ kind: "cr-list", state: "all" });
  expect(parseArgs(["cr", "list", "--state", "merged"])).toMatchObject({ kind: "cr-list", state: "merged" });
});
