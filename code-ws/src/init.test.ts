import { describe, expect, test } from "bun:test";
import { parseCliArgs, resolveInitTarget } from "./main";
import type { CodeWsCfg } from "./types";

const repo = { name: "brush", path: "/repos/brush", group: "ui", branch: "main" };
const other = { name: "api", path: "/repos/api", group: "backend", branch: "master" };
const cfg: CodeWsCfg = {
  workspaceRoot: "/workspaces",
  baseBranch: "master",
  remote: "origin",
  initAgentsTemplate: "/templates/default",
  projects: [repo, other],
  profiles: {
    work: { name: "work", agentsTemplate: "/templates/work", repos: [other] },
  },
};

describe("init project selection", () => {
  for (const flag of ["-p", "--project"]) {
    test(`${flag} 与模板组合时仅选择指定仓库`, () => {
      const args = parseCliArgs(["init", "feature/brush", "-t", "work", flag, "brush"]);
      if (args.cmd !== "init") throw new Error("expected init args");

      const target = resolveInitTarget(cfg, args);
      expect(target.agentsTemplate).toBe("/templates/work");
      expect(target.repos).toEqual([repo]);
    });
  }

  test("单独指定 project 使用默认模板", () => {
    const args = parseCliArgs(["init", "feature/brush", "--project", "brush"]);
    if (args.cmd !== "init") throw new Error("expected init args");

    const target = resolveInitTarget(cfg, args);
    expect(target.agentsTemplate).toBe("/templates/default");
    expect(target.repos).toEqual([repo]);
  });

  test("省略 project 保留 profile 仓库集合", () => {
    const args = parseCliArgs(["init", "feature/brush", "-t", "work"]);
    if (args.cmd !== "init") throw new Error("expected init args");

    const target = resolveInitTarget(cfg, args);
    expect(target.repos).toEqual([other]);
  });

  test("未知项目报错而不使用 profile 仓库", () => {
    const args = parseCliArgs(["init", "feature/brush", "-t", "work", "-p", "missing"]);
    if (args.cmd !== "init") throw new Error("expected init args");

    expect(() => resolveInitTarget(cfg, args)).toThrow("repo not found in project.yml: missing");
  });

  test("未知 profile 报错而不使用默认模板", () => {
    const args = parseCliArgs(["init", "feature/brush", "-t", "missing", "-p", "brush"]);
    if (args.cmd !== "init") throw new Error("expected init args");

    expect(() => resolveInitTarget(cfg, args)).toThrow("profile not found: missing");
  });

  test("缺值、重复项目和混用位置参数均在写入前拒绝", () => {
    expect(() => parseCliArgs(["init", "feature/brush", "--project"]))
      .toThrow("--project requires a value");
    expect(() => parseCliArgs(["init", "feature/brush", "-p", "brush", "-p", "api"]))
      .toThrow("--project may only be specified once");
    expect(() => parseCliArgs(["init", "feature/brush", "brush", "-p", "api"]))
      .toThrow("usage: code-ws init");
    expect(() => parseCliArgs(["projects", "-p", "brush"]))
      .toThrow("--project is only supported by init");
  });
});
