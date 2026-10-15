import { describe, expect, test } from "bun:test";
import {
  CLIENT_SIDEBAR_RESIZE,
  SIDEBAR_W_DEFAULT,
  SIDEBAR_W_KEY,
  SIDEBAR_W_MIN,
  clampSidebarW,
} from "./serve-ui-sidebar";

describe("clampSidebarW", () => {
  test("夹在最小宽度与视口比例上限之间", () => {
    expect(clampSidebarW(100, 1280)).toBe(SIDEBAR_W_MIN);
    expect(clampSidebarW(400, 1280)).toBe(400);
    expect(clampSidebarW(2000, 1280)).toBe(Math.floor(1280 * 0.6));
    expect(clampSidebarW(SIDEBAR_W_DEFAULT, 1280)).toBe(SIDEBAR_W_DEFAULT);
  });
});

describe("CLIENT_SIDEBAR_RESIZE", () => {
  test("浏览器脚本复用同一套夹取规则", () => {
    const fn = new Function(
      `${CLIENT_SIDEBAR_RESIZE}; return clampSidebarW;`,
    )() as (px: number, viewW: number) => number;
    expect(fn(100, 1280)).toBe(clampSidebarW(100, 1280));
    expect(fn(2000, 1280)).toBe(clampSidebarW(2000, 1280));
    expect(CLIENT_SIDEBAR_RESIZE).toContain(SIDEBAR_W_KEY);
    expect(CLIENT_SIDEBAR_RESIZE).toContain("bindSidebarResize");
  });
});
