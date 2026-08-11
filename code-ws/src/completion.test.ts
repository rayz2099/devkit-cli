import { describe, expect, test } from "bun:test";
import { renderFishCompletion } from "./completion";

describe("renderFishCompletion", () => {
  test("生成 project 动态候选", () => {
    const txt = renderFishCompletion();

    expect(txt).toContain("function __code_ws_projects");
    expect(txt).toContain("function __code_ws_needs_init_project");
    expect(txt).toContain("code-ws projects");
    expect(txt).toContain("__code_ws_needs_init_project");
    expect(txt).toContain("__code_ws_projects");
    expect(txt).toContain("fork");
  });
});
