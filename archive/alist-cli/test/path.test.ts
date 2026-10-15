import { expect, test } from "bun:test";
import { encodePath, joinRemote } from "../src/path";

test("joinRemote 去空段并强制绝对路径", () => {
  expect(joinRemote("/baidu", "a/b.txt")).toBe("/baidu/a/b.txt");
  expect(joinRemote("baidu", "/a/", "b.txt")).toBe("/baidu/a/b.txt");
  expect(joinRemote("/")).toBe("/");
});

test("joinRemote 拒绝 ..", () => {
  expect(() => joinRemote("/a", "../b")).toThrow("invalid remote path");
});

test("encodePath 整段 PathEscape", () => {
  expect(encodePath("/baidu/a b.txt")).toBe("%2Fbaidu%2Fa%20b.txt");
});
