import { afterEach, describe, expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { discoverMods, selectMods } from "./modules";

describe("discoverMods", () => {
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

  test("只发现带 justfile 的一级子项目", () => {
    const root = mkdtempSync(join(tmpdir(), "devkit-cli-"));
    dirs.push(root);

    mkdirSync(join(root, "code-ws"), {
      recursive: true,
    });
    writeFileSync(join(root, "code-ws", "Justfile"), "");

    mkdirSync(join(root, "docs"), {
      recursive: true,
    });

    expect(discoverMods(root).map((mod) => mod.name)).toEqual(["code-ws"]);
  });
});

describe("selectMods", () => {
  test("未指定模块时选择全部模块", () => {
    const mods = [
      {
        name: "code-ws",
        root: "/tmp/code-ws",
        justfile: "/tmp/code-ws/Justfile",
      },
    ];

    expect(selectMods(mods, [])).toEqual(mods);
  });

  test("指定不存在模块时报错", () => {
    const mods = [
      {
        name: "code-ws",
        root: "/tmp/code-ws",
        justfile: "/tmp/code-ws/Justfile",
      },
    ];

    expect(() => selectMods(mods, ["missing"])).toThrow("module not found: missing");
  });
});
