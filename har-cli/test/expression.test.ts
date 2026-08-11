import { describe, expect, test } from "bun:test";
import { evaluateWhere } from "../src/core/expression";

describe("evaluateWhere", () => {
  test("支持 and 组合 http.status 与 body.status", () => {
    const matched = evaluateWhere("body.status == 1 and http.status == 403", {
      body: { status: 1 },
      http: { status: 403 },
    });

    expect(matched).toBe(true);
  });

  test("body 缺失时返回 false 而不是抛错", () => {
    const matched = evaluateWhere("body.status == 1", {
      http: { status: 403 },
    });

    expect(matched).toBe(false);
  });
});
