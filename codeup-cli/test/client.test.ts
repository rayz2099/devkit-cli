import { expect, test } from "bun:test";
import { crCreateBody, toProjectId } from "../src/client";

test("create body 用数字 project id, 不带 createFrom", () => {
  const body = crCreateBody({
    projectId: 12,
    source: "feature/x",
    target: "master",
    title: "t",
  });
  expect(body).toEqual({
    title: "t",
    sourceBranch: "feature/x",
    targetBranch: "master",
    sourceProjectId: 12,
    targetProjectId: 12,
  });
  expect("createFrom" in body).toBe(false);
});

test("toProjectId 拒绝非数字", () => {
  expect(toProjectId("12")).toBe(12);
  expect(() => toProjectId("12a")).toThrow("invalid repository project id");
});
