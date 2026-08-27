import { expect, test } from "bun:test";
import { agentLimit, capRows, renderQuery } from "../src/output";

const data = {
  columns: ["id", "name"],
  rows: [
    { id: 1, name: "a" },
    { id: 2, name: "b" },
  ],
};

test("agent 默认 1000, 0 表示不截", () => {
  expect(agentLimit(undefined)).toBe(1000);
  expect(agentLimit(0)).toBe(0);
  expect(capRows(data.rows, 1)).toEqual({
    rows: [{ id: 1, name: "a" }],
    truncated: true,
  });
});

test("人 json 是 NDJSON, agent 是对象", () => {
  const human = renderQuery("human", "json", data, false);
  expect(human).toBe(`{"id":1,"name":"a"}\n{"id":2,"name":"b"}\n`);
  const agent = renderQuery("agent", "csv", data, true);
  expect(agent).toContain(`"truncated": true`);
  expect(agent).toContain(`"rows"`);
});

test("人 json --pretty 是缩进数组", () => {
  const pretty = renderQuery("human", "json", data, false, true);
  expect(pretty).toBe(`${JSON.stringify(data.rows, null, 2)}\n`);
});
