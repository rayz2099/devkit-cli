import { expect, test } from "bun:test";
import { maskHooks } from "../src/output";
import type { Webhook } from "../src/types";

const hook: Webhook = {
  id: "1",
  url: "https://example.com/hook",
  secretToken: "s3cret",
  pushEvents: true,
  mergeRequestsEvents: true,
  tagPushEvents: false,
  noteEvents: false,
  createdAt: "2026-01-01T00:00:00+08:00",
  updatedAt: "2026-01-02T00:00:00+08:00",
};

test("默认打码 secretToken", () => {
  expect(maskHooks([hook], false)[0]?.secretToken).toBe("***");
  expect(maskHooks([hook], true)[0]?.secretToken).toBe("s3cret");
});

test("默认打码 url query token", () => {
  const masked = maskHooks([{
    ...hook,
    url: "https://example.com/invoke?token=s3cret",
  }], false);
  expect(masked[0]?.url).toBe("https://example.com/invoke?token=***");
});
