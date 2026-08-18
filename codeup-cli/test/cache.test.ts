import { expect, test } from "bun:test";
import { hintRepos, indexPath, pickCachedRepo, scoreRepo, type CachedRepo } from "../src/cache";

const repos: CachedRepo[] = [
  { id: "1", name: "demo-api", pathNs: "backend/demo-api" },
  { id: "2", name: "demo-ops", pathNs: "backend/demo-ops" },
  { id: "3", name: "demo-ui", pathNs: "fe/demo-ui" },
];

test("短名唯一命中 path 尾段", () => {
  expect(pickCachedRepo("demo-api", repos).pathNs).toBe("backend/demo-api");
  expect(scoreRepo("demo-api", repos[0]!)).toBe(100);
});

test("模糊命中唯一子串", () => {
  expect(pickCachedRepo("demo-ui", repos).pathNs).toBe("fe/demo-ui");
});

test("歧义失败", () => {
  expect(() => pickCachedRepo("demo-", repos)).toThrow("ambiguous repository");
});

test("未命中失败", () => {
  expect(() => pickCachedRepo("nope", repos)).toThrow("no repository matched");
});

test("fish 提示同时给 path 和 name", () => {
  expect(hintRepos(repos)).toContain("backend/demo-api");
  expect(hintRepos(repos)).toContain("demo-api");
});

test("indexPath 落在配置目录", () => {
  expect(indexPath("prod", "/tmp/home")).toBe("/tmp/home/.config/codeup-cli/repos-prod.json");
});
