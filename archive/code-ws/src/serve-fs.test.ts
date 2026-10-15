import { describe, expect, test } from "bun:test";
import { mkdirSync, symlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import {
  isDeniedRelPath,
  listFileIndex,
  openRawFile,
  pickReadmePath,
  readBlob,
  resolveServeRoot,
  resolveUnderRoot,
} from "./serve-fs";

describe("serve-fs deny-list", () => {
  test("拦截 .git 与密钥类路径", () => {
    expect(isDeniedRelPath(".git/config")).toBe(true);
    expect(isDeniedRelPath("app/.env")).toBe(true);
    expect(isDeniedRelPath("certs/server.pem")).toBe(true);
    expect(isDeniedRelPath("id_rsa")).toBe(true);
    expect(isDeniedRelPath("src/main.ts")).toBe(false);
    expect(isDeniedRelPath("docs/note.md")).toBe(false);
  });
});

describe("serve-fs path resolve", () => {
  test("拒绝 path traversal", () => {
    const root = resolveServeRoot(".", process.cwd());
    expect(() => resolveUnderRoot(root, "../etc/passwd")).toThrow();
  });

  test("读取目录与文本文件", () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-test");
    mkdirSync(join(dir, "docs"), { recursive: true });
    writeFileSync(join(dir, "docs", "a.md"), "# hi\n");
    writeFileSync(join(dir, ".env"), "SECRET=1\n");
    mkdirSync(join(dir, ".git"), { recursive: true });
    writeFileSync(join(dir, ".git", "config"), "x\n");

    const root = resolveServeRoot(dir, process.cwd());
    const blob = readBlob(root, "");
    expect(blob.type).toBe("dir");
    const names = (blob.entries ?? []).map((e) => e.name);
    expect(names).toContain("docs");
    expect(names).not.toContain(".env");
    expect(names).not.toContain(".git");

    const md = readBlob(root, "docs/a.md");
    expect(md.content).toContain("# hi");
    expect(md.language).toBe("markdown");

    const files = listFileIndex(root);
    expect(files).toContain("docs/a.md");
    expect(files.some((f) => f.includes(".env"))).toBe(false);
  });
});

describe("pickReadmePath", () => {
  test("优先 README.md", () => {
    expect(
      pickReadmePath([
        { name: "src", path: "src", type: "dir" },
        { name: "README", path: "README", type: "file" },
        { name: "README.md", path: "README.md", type: "file" },
        { name: "readme.txt", path: "readme.txt", type: "file" },
      ]),
    ).toBe("README.md");
  });
});

describe("listFileIndex priority", () => {
  test("跳过 target/.class 并保留 README", () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-index");
    mkdirSync(join(dir, "app", "src"), { recursive: true });
    mkdirSync(join(dir, "app", "target", "classes"), { recursive: true });
    writeFileSync(join(dir, "app", "src", "Main.kt"), "class Main\n");
    writeFileSync(join(dir, "app", "README.md"), "# app\n");
    writeFileSync(join(dir, "app", "target", "classes", "Main.class"), "dead");
    writeFileSync(join(dir, "app", "target", "app.jar"), "dead");

    const files = listFileIndex(dir);
    expect(files).toContain("app/README.md");
    expect(files).toContain("app/src/Main.kt");
    expect(files.some((f) => f.endsWith(".class"))).toBe(false);
    expect(files.some((f) => f.includes("/target/"))).toBe(false);
  });
});

describe("preview kind", () => {
  test("pdf 可检索, 表格和媒体带预览类型", () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-preview");
    mkdirSync(dir, { recursive: true });
    writeFileSync(join(dir, "a.pdf"), "%PDF-1.4\n");
    writeFileSync(join(dir, "a.csv"), "name,qty\n\"a,b\",2\n");
    writeFileSync(join(dir, "a.xlsx"), Buffer.from([0x50, 0x4b, 0x03, 0x04, 0]));
    writeFileSync(join(dir, "a.mp4"), Buffer.from("0123456789"));

    const files = listFileIndex(dir);
    expect(files).toContain("a.pdf");

    const csv = readBlob(dir, "a.csv");
    expect(csv.preview).toBe("csv");
    expect(csv.content).toContain("name,qty");

    const xlsx = readBlob(dir, "a.xlsx");
    expect(xlsx.preview).toBe("xlsx");
    expect(xlsx.binary).toBe(true);
    expect(xlsx.content).toBeUndefined();

    const raw = openRawFile(dir, "a.mp4");
    expect(raw.contentType).toBe("video/mp4");
    expect(raw.size).toBe(10);
  });
});

describe("listFileIndex symlink safety", () => {
  test("不跟随 symlink, 环状链接不会拖死 index", () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-symlink");
    mkdirSync(join(dir, "a"), { recursive: true });
    writeFileSync(join(dir, "a", "README.md"), "# a\n");
    // a/loop -> a
    try {
      symlinkSync(join(dir, "a"), join(dir, "a", "loop"));
    } catch {
      symlinkSync("..", join(dir, "a", "loop"));
    }

    const started = Date.now();
    const files = listFileIndex(dir);
    expect(Date.now() - started).toBeLessThan(2000);
    expect(files).toContain("a/README.md");
    expect(files.every((f) => !f.includes("/loop/"))).toBe(true);
  });
});
