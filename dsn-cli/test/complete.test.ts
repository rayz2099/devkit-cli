import { expect, test } from "bun:test";
import { completeValues } from "../src/complete";

test("根命令和 agent 前缀包含 doctor", async () => {
  const root = await completeValues([], "");
  expect(root).toContain("doctor");
  expect(root).toContain("query");
  const afterAgent = await completeValues(["agent"], "");
  expect(afterAgent).toEqual(["query", "doctor"]);
});
