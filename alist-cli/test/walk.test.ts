import { afterEach, expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { planLocal } from "../src/walk";

const dirs: string[] = [];

afterEach(() => {
  for (const dir of dirs) {
    rmSync(dir, { recursive: true, force: true });
  }
  dirs.length = 0;
});

function tmp(): string {
  const dir = mkdtempSync(join(tmpdir(), "alist-walk-"));
  dirs.push(dir);
  return dir;
}

test("单文件", () => {
  const dir = tmp();
  const file = join(dir, "a.txt");
  writeFileSync(file, "x");
  expect(planLocal(file)).toEqual({ kind: "file", srcPath: file });
});

test("递归摊平相对路径", () => {
  const dir = tmp();
  mkdirSync(join(dir, "sub"));
  writeFileSync(join(dir, "a.txt"), "a");
  writeFileSync(join(dir, "sub", "b.txt"), "b");
  const plan = planLocal(dir);
  expect(plan.kind).toBe("dir");
  if (plan.kind !== "dir") {
    throw new Error("unreachable");
  }
  const rels = plan.files.map((item) => item.relPath).sort();
  expect(rels).toEqual(["a.txt", "sub/b.txt"]);
});

test("空目录", () => {
  const dir = tmp();
  expect(planLocal(dir)).toEqual({ kind: "dir", files: [] });
});

test("symlink 直接失败", () => {
  const dir = tmp();
  const file = join(dir, "a.txt");
  writeFileSync(file, "x");
  const link = join(dir, "link");
  symlinkSync(file, link);
  expect(() => planLocal(link)).toThrow("symlink not supported");
});

test("路径不存在", () => {
  expect(() => planLocal(join(tmp(), "missing"))).toThrow("path not found");
});
