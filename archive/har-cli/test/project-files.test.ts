import { describe, expect, test } from "bun:test";

describe("project files", () => {
  test("包含 justfile 常用配方", async () => {
    const content = await Bun.file("Justfile").text();

    expect(content).toContain("clean:");
    expect(content).toContain("build:");
    expect(content).toContain("install:");
  });
});
