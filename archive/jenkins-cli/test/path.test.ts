import { expect, test } from "bun:test";
import { isMainline, jobUrl, runResult } from "../src/path";

test("jobUrl 编码每一段", () => {
  expect(jobUrl("dt-vrec/feature/search_wm")).toBe(
    "/job/dt-vrec/job/feature/job/search_wm",
  );
});

test("isMainline 只看最后一段", () => {
  expect(isMainline("dt-vrec/master")).toBe(true);
  expect(isMainline("dt-vrec/main")).toBe(true);
  expect(isMainline("dt-vrec/feature/search_wm")).toBe(false);
  expect(isMainline("X_dt-vrec")).toBe(false);
});

test("runResult building 优先", () => {
  expect(runResult(true, "SUCCESS")).toBe("IN_PROGRESS");
  expect(runResult(false, null)).toBe("UNKNOWN");
});
