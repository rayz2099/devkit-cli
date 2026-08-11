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
import { bootstrapModConf } from "./main";

describe("bootstrapModConf", () => {
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

  test("缺失配置时从 example 复制到 XDG", () => {
    const rootDir = mkdtempSync(join(tmpdir(), "devkit-cli-main-"));
    const home = join(rootDir, "home");
    const modRoot = join(rootDir, "demo-cli");
    dirs.push(rootDir);

    mkdirSync(modRoot, { recursive: true });
    writeFileSync(join(modRoot, "config.example.json"), '{\n  "profiles": []\n}\n');
    writeFileSync(join(modRoot, "project.example.yml"), 'home: "$HOME/projects"\n');

    bootstrapModConf(
      {
        name: "demo-cli",
        root: modRoot,
        justfile: join(modRoot, "Justfile"),
      },
      home,
    );

    const cfg = join(home, ".config", "demo-cli", "config.json");
    const project = join(home, ".config", "demo-cli", "project.yml");
    expect(existsSync(cfg)).toBe(true);
    expect(existsSync(project)).toBe(true);
    expect(readFileSync(cfg, "utf8")).toContain("profiles");
    expect(readFileSync(project, "utf8")).toContain("$HOME/projects");
  });

  test("已有配置时不覆盖", () => {
    const rootDir = mkdtempSync(join(tmpdir(), "devkit-cli-main-"));
    const home = join(rootDir, "home");
    const modRoot = join(rootDir, "demo-cli");
    dirs.push(rootDir);

    mkdirSync(modRoot, { recursive: true });
    mkdirSync(join(home, ".config", "demo-cli"), {
      recursive: true,
    });
    writeFileSync(join(modRoot, "config.example.json"), '{"from":"example"}\n');
    writeFileSync(join(home, ".config", "demo-cli", "config.json"), '{"from":"local"}\n');

    bootstrapModConf(
      {
        name: "demo-cli",
        root: modRoot,
        justfile: join(modRoot, "Justfile"),
      },
      home,
    );

    expect(readFileSync(join(home, ".config", "demo-cli", "config.json"), "utf8")).toContain("local");
  });

  test("没有 example 时静默跳过", () => {
    const rootDir = mkdtempSync(join(tmpdir(), "devkit-cli-main-"));
    const home = join(rootDir, "home");
    const modRoot = join(rootDir, "demo-cli");
    dirs.push(rootDir);
    mkdirSync(modRoot, {
      recursive: true,
    });

    bootstrapModConf(
      {
        name: "demo-cli",
        root: modRoot,
        justfile: join(modRoot, "Justfile"),
      },
      home,
    );

    expect(existsSync(join(home, ".config", "demo-cli"))).toBe(false);
  });

  test("存在 templates 时写入 module-root 标记", () => {
    const rootDir = mkdtempSync(join(tmpdir(), "devkit-cli-main-"));
    const home = join(rootDir, "home");
    const modRoot = join(rootDir, "demo-cli");
    dirs.push(rootDir);

    mkdirSync(join(modRoot, "templates"), { recursive: true });
    writeFileSync(join(modRoot, "config.example.json"), '{"ok":true}\n');

    bootstrapModConf(
      {
        name: "demo-cli",
        root: modRoot,
        justfile: join(modRoot, "Justfile"),
      },
      home,
    );

    const marker = join(home, ".config", "demo-cli", "module-root");
    expect(readFileSync(marker, "utf8").trim()).toBe(modRoot);
  });
});
