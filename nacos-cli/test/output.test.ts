import { describe, expect, test } from "bun:test";
import { render, renderTable } from "../src/output";

describe("render", () => {
  test("空 text 不补空行", () => {
    expect(render("text", "", null)).toBe("");
  });

  test("json 缩进输出", () => {
    expect(render("json", "ignored", { success: true })).toBe('{\n  "success": true\n}\n');
  });

  test("table 带 summary", () => {
    const text = renderTable("Total: 1  Page: 1/1", ["DATA_ID", "GROUP"], [["d1", "g1"]]);
    expect(text).toContain("Total: 1  Page: 1/1");
    expect(text).toContain("DATA_ID");
    expect(text).toContain("d1");
    expect(text).toContain("g1");
  });
});
