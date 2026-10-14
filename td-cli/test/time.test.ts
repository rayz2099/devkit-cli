import { expect, test } from "bun:test";
import { resolveRecent, resolveSpan } from "../src/time";

const now = new Date(2026, 9, 14, 15, 4, 5);

test("1-7 是过去 7 天, 不含今天", () => {
  expect(resolveRecent("1-7", now)).toEqual({
    recentDay: "1-7",
    startTime: "2026-10-07 00:00:00",
    endTime: "2026-10-13 23:59:59",
  });
});

test("0-7 含今天, 往前共 7 天", () => {
  expect(resolveRecent("0-7", now)).toEqual({
    recentDay: "0-7",
    startTime: "2026-10-08 00:00:00",
    endTime: "2026-10-14 23:59:59",
  });
});

test("事件分析缺省用 1-7, 属性分析不补时间", () => {
  expect(resolveSpan(undefined, undefined, undefined, true, now)?.recentDay).toBe("1-7");
  expect(resolveSpan(undefined, undefined, undefined, false, now)).toBeUndefined();
});

test("from 和 to 必须成对", () => {
  expect(() => resolveSpan(undefined, "2026-10-01 00:00:00", undefined, false, now))
    .toThrow("--from and --to are both required");
});
