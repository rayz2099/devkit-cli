import { describe, expect, test } from "bun:test";
import {
  fuzzyScore,
  parseSearchQuery,
  rankPaths,
  scorePath,
} from "./serve-search";

describe("parseSearchQuery", () => {
  test("拆出扩展名过滤与模糊词", () => {
    expect(parseSearchQuery("readme .md")).toEqual({
      terms: ["readme"],
      exts: [".md"],
    });
    expect(parseSearchQuery("type:kt OssApi")).toEqual({
      terms: ["OssApi"],
      exts: [".kt", ".kts"],
    });
    expect(parseSearchQuery("ext:md")).toEqual({
      terms: [],
      exts: [".md", ".markdown"],
    });
  });
});

describe("fuzzyScore", () => {
  test("精确/子串优先于松散子序列", () => {
    expect(fuzzyScore("readme.md", "readme.md")).toBeGreaterThan(
      fuzzyScore("dt-metadata.md", "readme.md"),
    );
    expect(fuzzyScore("kt-file-tools/README.md", "readme")).toBeGreaterThan(0);
    // 过散的子序列应被丢弃, 防止 readme 命中 metadata
    expect(fuzzyScore("dt-metadata.md", "readme.md")).toBe(-1);
  });
});

describe("scorePath / rankPaths", () => {
  const files = [
    "saturn/README.md",
    "kt-file/README.md",
    "kt-file/libs/kt-file-tools/README.md",
    "tasks/tools-cdn-replace-cdn-host-util/dt-metadata.md",
    "kt-file/libs/kt-file-tools/src/main/kotlin/OssApi.kt",
    "dt-base/README.md",
  ];

  test("readme 能命中 kt-file 下的 README", () => {
    const ranked = rankPaths(files, "readme.md");
    const paths = ranked.map((x) => x.path);
    expect(paths).toContain("kt-file/libs/kt-file-tools/README.md");
    expect(paths).toContain("kt-file/README.md");
    expect(paths).not.toContain(
      "tasks/tools-cdn-replace-cdn-host-util/dt-metadata.md",
    );
  });

  test("扩展名过滤", () => {
    const ranked = rankPaths(files, "oss .kt");
    expect(ranked.map((x) => x.path)).toEqual([
      "kt-file/libs/kt-file-tools/src/main/kotlin/OssApi.kt",
    ]);
  });

  test("空 query 优先 recent", () => {
    const ranked = rankPaths(
      files,
      "",
      ["kt-file/libs/kt-file-tools/README.md", "missing.md"],
      3,
    );
    expect(ranked[0]?.path).toBe("kt-file/libs/kt-file-tools/README.md");
    expect(ranked).toHaveLength(3);
  });

  test("文件名命中分高于路径深层误伤", () => {
    const a = scorePath("kt-file/libs/kt-file-tools/README.md", parseSearchQuery("readme"));
    const b = scorePath("tasks/tools-cdn-replace-cdn-host-util/dt-metadata.md", parseSearchQuery("readme"));
    expect(a).toBeGreaterThan(b);
  });
});
