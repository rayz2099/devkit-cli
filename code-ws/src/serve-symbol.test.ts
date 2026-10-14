import { describe, expect, test } from "bun:test";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { listFileIndex } from "./serve-fs";
import { searchSymbol } from "./serve-symbol";

describe("searchSymbol", () => {
  test("找出定义和引用, 过短的名字不扫描", () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-symbol");
    mkdirSync(join(dir, "src"), { recursive: true });
    writeFileSync(join(dir, "src", "Foo.kt"), "package demo\nclass Foo {\n  fun bar() {}\n}\n");
    writeFileSync(join(dir, "src", "Use.kt"), "import demo.Foo\nfun main() { Foo() }\n");

    const files = listFileIndex(dir);
    const found = searchSymbol(dir, files, "Foo");
    const def = found.hits.find((hit) => hit.kind === "def");
    expect(def?.path).toBe("src/Foo.kt");
    expect(def?.line).toBe(2);
    expect(found.hits.some((hit) => hit.kind === "ref" && hit.path === "src/Use.kt")).toBe(true);
    expect(searchSymbol(dir, files, "F")).toEqual({
      query: "F",
      hits: [],
      truncated: false,
    });
  });
});
