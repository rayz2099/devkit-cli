import { expect, test } from "bun:test";
import { parseArgs } from "../src/args";

test("默认 human, agent 是前缀", () => {
  expect(parseArgs(["ls"])).toMatchObject({
    kind: "ls",
    audience: "human",
    path: "/",
    page: 1,
    size: 100,
    refresh: false,
  });
  expect(parseArgs(["agent", "ls", "/disk"])).toMatchObject({
    kind: "ls",
    audience: "agent",
    path: "/disk",
  });
});

test("-p 可插在前面", () => {
  expect(parseArgs(["-p", "home", "mkdir", "/a/b"])).toMatchObject({
    kind: "mkdir",
    profile: "home",
    path: "/a/b",
  });
});

test("put 位置参数和 flags 都能取 src/dst", () => {
  expect(parseArgs(["put", "/tmp/a", "/remote/a"])).toMatchObject({
    kind: "put",
    src: "/tmp/a",
    dst: "/remote/a",
    asTask: false,
  });
  expect(parseArgs(["put", "--src", "/tmp/a", "--dst", "/remote/a", "--as-task"])).toMatchObject({
    kind: "put",
    src: "/tmp/a",
    dst: "/remote/a",
    asTask: true,
  });
});

test("put 缺 src/dst 失败", () => {
  expect(() => parseArgs(["put", "/tmp/a"])).toThrow("put: missing <dst>");
  expect(() => parseArgs(["put"])).toThrow("put: missing <src>");
});

test("sync 缺 src/dst 失败", () => {
  expect(() => parseArgs(["sync", "/tmp/a"])).toThrow("sync: missing <dst>");
});

test("未知命令失败", () => {
  expect(() => parseArgs(["rm"])).toThrow("unknown command");
});

test("ls --refresh --page --size", () => {
  expect(parseArgs(["ls", "/x", "--page", "2", "--size", "50", "--refresh"])).toMatchObject({
    kind: "ls",
    path: "/x",
    page: 2,
    size: 50,
    refresh: true,
  });
});
