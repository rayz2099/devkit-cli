import { afterEach, describe, expect, test } from "bun:test";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { copyAgentsTemplate, copyForkWorkspaceEntries } from "./agents";

describe("copyAgentsTemplate", () => {
  const dirs: string[] = [];

  afterEach(() => {
    for (const dir of dirs) {
      rmSync(dir, {
        recursive: true,
        force: true,
      });
    }
    dirs.length = 0;
  });

  test("复制模板目录内容到 workspace 根目录", () => {
    const root = mkdtempSync(join(tmpdir(), "code-ws-"));
    dirs.push(root);

    const tpl = join(root, "tpl");
    const ws = join(root, "ws");
    mkdirSync(join(tpl, ".agents"), {
      recursive: true,
    });
    mkdirSync(join(tpl, "spec"));
    writeFileSync(join(tpl, "AGENTS.md"), "agents");
    writeFileSync(join(tpl, "README.md"), "agents");
    writeFileSync(join(tpl, ".agents", "skill.md"), "skill");
    writeFileSync(join(tpl, "spec", "context.md"), "context");

    copyAgentsTemplate(tpl, ws);

    expect(existsSync(join(ws, "AGENTS.md"))).toBe(true);
    expect(existsSync(join(ws, "README.md"))).toBe(true);
    expect(existsSync(join(ws, ".agents", ".agents"))).toBe(false);
    expect(existsSync(join(ws, ".agents", "skill.md"))).toBe(true);
    expect(existsSync(join(ws, "spec", "context.md"))).toBe(true);
  });

  test("目标文件已存在时严格失败", () => {
    const root = mkdtempSync(join(tmpdir(), "code-ws-"));
    dirs.push(root);

    const tpl = join(root, "tpl");
    const ws = join(root, "ws");
    mkdirSync(tpl);
    mkdirSync(ws, {
      recursive: true,
    });
    writeFileSync(join(tpl, "README.md"), "agents");
    writeFileSync(join(ws, "README.md"), "old");

    expect(() => copyAgentsTemplate(tpl, ws)).toThrow(
      "workspace template target already exists",
    );
  });
});

describe("copyForkWorkspaceEntries", () => {
  const dirs: string[] = [];

  afterEach(() => {
    for (const dir of dirs) {
      rmSync(dir, {
        recursive: true,
        force: true,
      });
    }
    dirs.length = 0;
  });

  test("只复制 fork 固定枚举条目", () => {
    const root = mkdtempSync(join(tmpdir(), "code-ws-"));
    dirs.push(root);

    const src = join(root, "src");
    const dst = join(root, "dst");
    mkdirSync(join(src, "docs"), {
      recursive: true,
    });
    mkdirSync(join(src, "spec"));
    mkdirSync(join(src, "tasks"));
    mkdirSync(join(src, ".agents"));
    mkdirSync(join(src, "app-api"));
    writeFileSync(join(src, "project.yml"), "branch: feature/a\n");
    writeFileSync(join(src, "docs", "prd.md"), "prd");
    writeFileSync(join(src, "spec", "context.md"), "context");
    writeFileSync(join(src, "tasks", "task.md"), "task");
    writeFileSync(join(src, ".agents", "skill.md"), "skill");
    writeFileSync(join(src, "AGENTS.md"), "agents");
    writeFileSync(join(src, "README.md"), "readme");
    writeFileSync(join(src, "diamond-card.code-workspace"), "{}");
    writeFileSync(join(src, "app-api", "App.kt"), "code");

    const copied = copyForkWorkspaceEntries(
      src,
      dst,
    );

    expect(copied).toEqual([
      "project.yml",
      "docs",
      "spec",
      "tasks",
      "AGENTS.md",
      "README.md",
      ".agents",
    ]);
    expect(readFileSync(join(dst, "docs", "prd.md"), "utf8")).toBe("prd");
    expect(existsSync(join(dst, "diamond-card.code-workspace"))).toBe(false);
    expect(existsSync(join(dst, "app-api"))).toBe(false);
  });

  test("枚举条目不存在时跳过", () => {
    const root = mkdtempSync(join(tmpdir(), "code-ws-"));
    dirs.push(root);

    const src = join(root, "src");
    const dst = join(root, "dst");
    mkdirSync(src);
    writeFileSync(join(src, "project.yml"), "branch: feature/a\n");

    const copied = copyForkWorkspaceEntries(
      src,
      dst,
    );

    expect(copied).toEqual(["project.yml"]);
    expect(existsSync(join(dst, "project.yml"))).toBe(true);
  });
});
