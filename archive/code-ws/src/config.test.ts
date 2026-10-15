import { describe, expect, test } from "bun:test";
import { parseConfig, resolveConfig } from "./config";

describe("parseConfig", () => {
  test("解析只包含 repo name 和 template name 的有效配置", () => {
    const cfg = parseConfig({
      workspaceRoot: "$HOME/projects/workspaces",
      baseBranch: "master",
      remote: "origin",
      initAgentsTemplate: "work",
      profiles: {
        "work-01": {
          name: "daily",
          agentsTemplate: "backend",
          repos: [
            "app-gw",
          ],
        },
      },
    });

    expect(cfg.workspaceRoot).toBe("$HOME/projects/workspaces");
    expect(cfg.initAgentsTemplate).toBe("work");
    expect(cfg.profiles["work-01"]?.agentsTemplate).toBe("backend");
    expect(cfg.profiles["work-01"]?.repos[0]).toBe("app-gw");
  });

  test("缺少 workspaceRoot 时报错", () => {
    expect(() =>
      parseConfig({
        baseBranch: "master",
        remote: "origin",
        profiles: {},
      }),
    ).toThrow("workspaceRoot must be a non-empty string");
  });
});

describe("resolveConfig", () => {
  test("通过 project.yml repo name 解析真实路径并展开 HOME", () => {
    const cfg = parseConfig({
        workspaceRoot: "$HOME/projects/workspaces",
        baseBranch: "master",
        remote: "origin",
        initAgentsTemplate: "work",
        profiles: {
          daily: {
            name: "profile name can change",
          agentsTemplate: "work-01",
          repos: [
            "app-gw",
            "app-audit",
          ],
        },
      },
    });

    const resolved = resolveConfig(
      cfg,
      [
        "home: \"$HOME/projects\"",
        "projects:",
        "  - repo-name: app-gw",
        "    group: backend",
        "    description: app gateway",
        "  - repo-name: app-audit",
        "    group: frontend",
      ].join("\n"),
      {
        home: "/home/dev",
        templateRoot: "/home/dev/.config/code-ws/templates/agents",
      },
    );

    expect(resolved.workspaceRoot).toBe("/home/dev/projects/workspaces");
    expect(resolved.initAgentsTemplate).toBe(
      "/home/dev/.config/code-ws/templates/agents/work",
    );
    expect(resolved.profiles.daily?.agentsTemplate).toBe(
      "/home/dev/.config/code-ws/templates/agents/work-01",
    );
    expect(resolved.profiles.daily?.repos).toEqual([
      {
        name: "app-gw",
        path: "/home/dev/projects/backend/app-gw",
        group: "backend",
        branch: "master",
        description: "app gateway",
      },
      {
        name: "app-audit",
        path: "/home/dev/projects/frontend/app-audit",
        group: "frontend",
        branch: "master",
      },
    ]);
  });

  test("repo 未声明 branch 时默认使用 master, 声明时使用项目主分支", () => {
    const cfg = parseConfig({
        workspaceRoot: "$HOME/projects/workspaces",
        baseBranch: "master",
        remote: "origin",
        initAgentsTemplate: "work",
        profiles: {
          daily: {
            name: "profile name can change",
          agentsTemplate: "work-01",
          repos: [
            "app-gw",
            "editor-tool",
          ],
        },
      },
    });

    const resolved = resolveConfig(
      cfg,
      [
        "home: \"$HOME/projects\"",
        "projects:",
        "  - repo-name: app-gw",
        "    group: backend",
        "  - repo-name: editor-tool",
        "    group: product",
        "    branch: main",
      ].join("\n"),
      {
        home: "/home/dev",
        templateRoot: "/home/dev/.config/code-ws/templates/agents",
      },
    );

    expect(resolved.profiles.daily?.repos).toEqual([
      {
        name: "app-gw",
        path: "/home/dev/projects/backend/app-gw",
        group: "backend",
        branch: "master",
      },
      {
        name: "editor-tool",
        path: "/home/dev/projects/product/editor-tool",
        group: "product",
        branch: "main",
      },
    ]);
  });

  test("profile key 和 agentsTemplate 名称解耦", () => {
    const cfg = parseConfig({
        workspaceRoot: "$HOME/projects/workspaces",
        baseBranch: "master",
        remote: "origin",
        initAgentsTemplate: "work",
        profiles: {
          renamed: {
            name: "daily backend workspace",
          agentsTemplate: "work-01",
          repos: [
            "app-gw",
          ],
        },
      },
    });

    const resolved = resolveConfig(
      cfg,
      [
        "home: \"$HOME/projects\"",
        "projects:",
        "  - repo-name: app-gw",
        "    group: backend",
      ].join("\n"),
      {
        home: "/home/dev",
        templateRoot: "/home/dev/.config/code-ws/templates/agents",
      },
    );

    expect(resolved.profiles.renamed?.name).toBe("daily backend workspace");
    expect(resolved.profiles.renamed?.agentsTemplate).toBe(
      "/home/dev/.config/code-ws/templates/agents/work-01",
    );
  });
});
