import { afterEach, describe, expect, test } from "bun:test";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { ImagesStore, splitSource } from "../src/images";

const dirs: string[] = [];

afterEach(() => {
  for (const dir of dirs.splice(0)) {
    rmSync(dir, { recursive: true, force: true });
  }
});

describe("ImagesStore", () => {
  test("新版本移动到首位并保留其他版本", () => {
    const dir = mkdtempSync(join(tmpdir(), "gh-image-cli-"));
    dirs.push(dir);
    const path = join(dir, "images.yaml");
    writeFileSync(path, `# inventory\nimages:\n  cpa:\n    type: mirror\n    source: eceasy/cli-proxy-api\n    versions:\n      latest:\n        status: done\n`);

    const store = new ImagesStore(path);
    store.addMirror("eceasy/cli-proxy-api", "cpa", "v7.2.130");
    store.save();

    const text = readFileSync(path, "utf8");
    expect(text).toStartWith("# inventory");
    expect(text.indexOf("v7.2.130")).toBeLessThan(text.indexOf("latest"));
    expect(new ImagesStore(path).versions("cpa")).toEqual(["v7.2.130", "latest"]);
  });

  test("同 alias 不允许更换 source", () => {
    const dir = mkdtempSync(join(tmpdir(), "gh-image-cli-"));
    dirs.push(dir);
    const path = join(dir, "images.yaml");
    writeFileSync(path, "images: {}\n");
    const store = new ImagesStore(path);
    store.addMirror("postgres", "postgres", "latest");
    expect(() => store.addMirror("other/postgres", "postgres", "v2")).toThrow();
  });
});

test("source tag 只从最后一个 path segment 解析", () => {
  expect(splitSource("localhost:5000/team/app:v1")).toEqual({
    source: "localhost:5000/team/app",
    version: "v1",
  });
});
