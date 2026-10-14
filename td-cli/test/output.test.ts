import { expect, test } from "bun:test";
import { emit, errBody, eventRows, queryText } from "../src/output";

test("agent 成功和失败都是 JSON", () => {
  expect(JSON.parse(emit("agent", { n: 1 }, "ignored"))).toEqual({ ok: true, data: { n: 1 } });
  expect(JSON.parse(errBody("unauthorized"))).toEqual({
    ok: false,
    error: { type: "auth", message: "unauthorized" },
  });
});

test("human 用文本, 事件名从目录树里抽", () => {
  expect(emit("human", {}, "ID  NAME\n")).toBe("ID  NAME\n");
  const rows = eventRows({
    eventCatalog: [{ events: [{ eventName: "login", eventDesc: "登录" }] }],
  });
  expect(rows).toEqual([["login", "登录"]]);
});

test("x/y 结果打成表", () => {
  const text = queryText({
    data: {
      x: ["10-13"],
      y: [{ eventName: "login", data: [3] }],
    },
  });
  expect(text).toContain("login");
  expect(text).toContain("3");
});
