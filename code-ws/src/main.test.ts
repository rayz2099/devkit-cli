import { describe, expect, test } from "bun:test";
import {
  destroyWorkspace,
  forkWorkspace,
  parseCliArgs,
  resolveDefaultConfig,
} from "./main";

describe("parseCliArgs", () => {
  test("解析 init branch 和 profile", () => {
    const args = parseCliArgs([
      "init",
      "feature/spec101",
      "-t",
      "work-01",
    ]);

    expect(args).toEqual({
      cmd: "init",
      branch: "feature/spec101",
      profile: "work-01",
      config: undefined,
      verbose: false,
    });
  });

  test("解析 init verbose 参数", () => {
    const args = parseCliArgs([
      "init",
      "feature/spec101",
      "-t",
      "work-01",
      "-v",
    ]);

    expect(args).toEqual({
      cmd: "init",
      branch: "feature/spec101",
      profile: "work-01",
      config: undefined,
      verbose: true,
    });
  });

  test("解析 init branch 和首个 project", () => {
    const args = parseCliArgs([
      "init",
      "feature/spec101",
      "shared-lib",
      "-v",
    ]);

    expect(args).toEqual({
      cmd: "init",
      branch: "feature/spec101",
      project: "shared-lib",
      config: undefined,
      verbose: true,
    });
  });

  test("拒绝 init 同时指定 project 和 profile", () => {
    expect(() =>
      parseCliArgs([
        "init",
        "feature/spec101",
        "shared-lib",
        "-t",
        "work-01",
      ]),
    ).toThrow("usage: code-ws init <branch> <project>");
  });

  test("解析 completion fish", () => {
    const args = parseCliArgs([
      "completion",
      "fish",
    ]);

    expect(args).toEqual({
      cmd: "completion-fish",
    });
  });

  test("解析 serve 默认参数", () => {
    const args = parseCliArgs([
      "serve",
    ]);

    expect(args).toEqual({
      cmd: "serve",
      path: undefined,
      lan: true,
      watch: true,
      port: undefined,
    });
  });

  test("解析 serve path --lan --port", () => {
    const args = parseCliArgs([
      "serve",
      "./docs",
      "--lan",
      "--port",
      "8787",
    ]);

    expect(args).toEqual({
      cmd: "serve",
      path: "./docs",
      lan: true,
      watch: true,
      port: 8787,
    });
  });

  test("解析 serve --local", () => {
    const args = parseCliArgs([
      "serve",
      "--local",
    ]);

    expect(args).toEqual({
      cmd: "serve",
      path: undefined,
      lan: false,
      watch: true,
      port: undefined,
    });
  });

  test("解析 serve --no-watch", () => {
    const args = parseCliArgs([
      "serve",
      "--no-watch",
    ]);

    expect(args).toEqual({
      cmd: "serve",
      path: undefined,
      lan: true,
      watch: false,
      port: undefined,
    });
  });

  test("解析 help 参数", () => {
    const args = parseCliArgs([
      "add",
      "project",
      "my-room",
      "-h",
    ]);

    expect(args).toEqual({
      cmd: "help",
    });
  });

  test("解析 projects", () => {
    const args = parseCliArgs([
      "projects",
      "--config",
      "/tmp/config.json",
    ]);

    expect(args).toEqual({
      cmd: "projects",
      config: "/tmp/config.json",
    });
  });

  test("解析 config check", () => {
    const args = parseCliArgs([
      "config",
      "check",
      "--config",
      "/tmp/config.json",
    ]);

    expect(args).toEqual({
      cmd: "config-check",
      config: "/tmp/config.json",
    });
  });

  test("解析 add project", () => {
    const args = parseCliArgs([
      "add",
      "project",
      "shared-lib",
      "--config",
      "/tmp/config.json",
      "-v",
    ]);

    expect(args).toEqual({
      cmd: "add-project",
      repo: "shared-lib",
      branch: undefined,
      config: "/tmp/config.json",
      verbose: true,
    });
  });

  test("解析 add project 固定 checkout 分支", () => {
    const args = parseCliArgs([
      "add",
      "project",
      "my-room",
      "--branch",
      "feature/room-organizing",
    ]);

    expect(args).toEqual({
      cmd: "add-project",
      repo: "my-room",
      branch: "feature/room-organizing",
      config: undefined,
      verbose: false,
    });
  });

  test("解析 remove project", () => {
    const args = parseCliArgs([
      "remove",
      "project",
      "shared-lib",
      "--config",
      "/tmp/config.json",
      "-v",
    ]);

    expect(args).toEqual({
      cmd: "remove-project",
      repo: "shared-lib",
      config: "/tmp/config.json",
      verbose: true,
    });
  });

  test("解析 sync master", () => {
    const args = parseCliArgs([
      "sync",
      "master",
      "--config",
      "/tmp/config.json",
      "-v",
    ]);

    expect(args).toEqual({
      cmd: "sync-master",
      config: "/tmp/config.json",
      verbose: true,
    });
  });

  test("解析 destroy workspace", () => {
    const args = parseCliArgs([
      "destroy",
      "--config",
      "/tmp/config.json",
      "-v",
    ]);

    expect(args).toEqual({
      cmd: "destroy-workspace",
      config: "/tmp/config.json",
      verbose: true,
    });
  });

  test("解析 fork workspace", () => {
    const args = parseCliArgs([
      "fork",
      "feature/diamond-card2",
      "--config",
      "/tmp/config.json",
      "-v",
    ]);

    expect(args).toEqual({
      cmd: "fork-workspace",
      branch: "feature/diamond-card2",
      config: "/tmp/config.json",
      verbose: true,
    });
  });

  test("拒绝缺少目标分支的 fork", () => {
    expect(() =>
      parseCliArgs([
        "fork",
      ]),
    ).toThrow("usage: code-ws fork <branch>");
  });
});

describe("destroyWorkspace", () => {
  test("安全检查通过后只卸载 git worktree 并保留 workspace 目录", () => {
    const plans: string[][] = [];
    const logs: string[] = [];
    const checks: Array<{
      wsDir: string;
      repos: string[];
      remote: string;
      baseBranch: string;
    }> = [];

    destroyWorkspace(
      {
        cmd: "destroy-workspace",
        verbose: false,
      },
      {
        workspaceRoot: "/tmp",
        baseBranch: "master",
        remote: "origin",
        initAgentsTemplate: "/templates/work",
        projects: [
          {
            name: "app-gw",
            path: "/src/app-gw",
            group: "backend",
            branch: "master",
          },
          {
            name: "app-api",
            path: "/src/app-api",
            group: "backend",
            branch: "master",
          },
        ],
        profiles: {},
      },
      "/tmp/spec101-workspace",
      {
        branch: "feature/spec101",
        repos: [
          "app-gw",
          "app-api",
        ],
        checkoutBranches: {},
      },
      {
        assertDisposable: (wsDir, repos, remote, baseBranch) => {
          checks.push({
            wsDir,
            repos: repos.map((repo) => repo.name),
            remote,
            baseBranch,
          });
        },
        runPlan: (plan) => {
          plans.push(plan.map((cmd) => cmd.repo));
        },
        log: (msg) => {
          logs.push(msg);
        },
      },
    );

    expect(checks).toEqual([
      {
        wsDir: "/tmp/spec101-workspace",
        repos: [
          "app-gw",
          "app-api",
        ],
        remote: "origin",
        baseBranch: "master",
      },
    ]);
    expect(plans).toEqual([
      [
        "app-gw",
        "app-api",
      ],
    ]);
    expect(logs).toEqual([
      "workspace worktrees cleared: /tmp/spec101-workspace",
      "workspace directory kept: /tmp/spec101-workspace",
    ]);
  });

  test("存在未提交改动或未合并 commit 时终止且不卸载 worktree", () => {
    const plans: string[][] = [];
    const logs: string[] = [];

    expect(() =>
      destroyWorkspace(
        {
          cmd: "destroy-workspace",
          verbose: false,
        },
        {
          workspaceRoot: "/tmp",
          baseBranch: "master",
          remote: "origin",
          initAgentsTemplate: "/templates/work",
          projects: [
            {
              name: "app-gw",
              path: "/src/app-gw",
              group: "backend",
              branch: "master",
            },
          ],
          profiles: {},
        },
        "/tmp/spec101-workspace",
        {
          branch: "feature/spec101",
          repos: [
            "app-gw",
          ],
          checkoutBranches: {},
        },
        {
          assertDisposable: () => {
            throw new Error(
              "workspace cleanup aborted: repo=app-gw path=/tmp/spec101-workspace/app-gw\n" +
                "reason: uncommitted changes",
            );
          },
          runPlan: (plan) => {
            plans.push(plan.map((cmd) => cmd.repo));
          },
          log: (msg) => {
            logs.push(msg);
          },
        },
      ),
    ).toThrow("uncommitted changes");

    expect(plans).toEqual([]);
    expect(logs).toEqual([]);
  });

  test("worktree remove 失败时立即终止", () => {
    const logs: string[] = [];

    expect(() =>
      destroyWorkspace(
        {
          cmd: "destroy-workspace",
          verbose: false,
        },
        {
          workspaceRoot: "/tmp",
          baseBranch: "master",
          remote: "origin",
          initAgentsTemplate: "/templates/work",
          projects: [
            {
              name: "app-gw",
              path: "/src/app-gw",
              group: "backend",
              branch: "master",
            },
          ],
          profiles: {},
        },
        "/tmp/spec101-workspace",
        {
          branch: "feature/spec101",
          repos: [
            "app-gw",
          ],
          checkoutBranches: {},
        },
        {
          assertDisposable: () => {},
          runPlan: () => {
            throw new Error("remove failed");
          },
          log: (msg) => {
            logs.push(msg);
          },
        },
      ),
    ).toThrow("remove failed");

    expect(logs).toEqual([]);
  });
});

describe("forkWorkspace", () => {
  test("目标 workspace 已存在且确认后先删除再 fork", () => {
    const prompts: string[] = [];
    const removedDirs: string[] = [];
    const plans: string[][] = [];
    const madeDirs: string[] = [];
    const copiedEntries: string[] = [];
    const wsFiles: string[] = [];
    const projectFiles: string[] = [];
    const logs: string[] = [];

    forkWorkspace(
      {
        cmd: "fork-workspace",
        branch: "feature/diamond-card2",
        verbose: false,
      },
      {
        workspaceRoot: "/tmp",
        baseBranch: "master",
        remote: "origin",
        initAgentsTemplate: "/templates/work",
        projects: [
          {
            name: "diamond-card",
            path: "/src/diamond-card",
            group: "backend",
            branch: "master",
          },
        ],
        profiles: {},
      },
      "/tmp/diamond-card-workspace",
      {
        branch: "feature/diamond-card",
        repos: [
          "diamond-card",
        ],
        checkoutBranches: {},
      },
      {
        dstExists: () => true,
        assertSrcClean: () => {},
        assertDstDisposable: () => {},
        assertDstTargets: () => {},
        readAnswer: (msg) => {
          prompts.push(msg);
          return "y";
        },
        removeDir: (path) => {
          removedDirs.push(path);
        },
        mkdir: (path) => {
          madeDirs.push(path);
        },
        runPlan: (plan) => {
          plans.push(plan.map((cmd) => cmd.step));
        },
        copyEntries: (src, dst) => {
          copiedEntries.push(`${src} -> ${dst}`);
        },
        applyAgents: () => {},
        writeWsFile: (wsDir, branch) => {
          wsFiles.push(`${wsDir}:${branch}`);
          return "diamond-card2.code-workspace";
        },
        writeWsProject: (wsDir, branch) => {
          projectFiles.push(`${wsDir}:${branch}`);
        },
        log: (msg) => {
          logs.push(msg);
        },
      },
    );

    expect(prompts).toEqual([
      "workspace already exists: /tmp/diamond-card2-workspace\n" +
        "delete it and continue? [y/N] ",
    ]);
    expect(plans).toEqual([
      [
        "worktree-remove",
      ],
      [
        "worktree-prune",
        "worktree-fork",
      ],
    ]);
    expect(removedDirs).toEqual(["/tmp/diamond-card2-workspace"]);
    expect(madeDirs).toEqual(["/tmp/diamond-card2-workspace"]);
    expect(copiedEntries).toEqual([
      "/tmp/diamond-card-workspace -> /tmp/diamond-card2-workspace",
    ]);
    expect(wsFiles).toEqual([
      "/tmp/diamond-card2-workspace:feature/diamond-card2",
    ]);
    expect(projectFiles).toEqual([
      "/tmp/diamond-card2-workspace:feature/diamond-card2",
    ]);
    expect(logs).toEqual([
      "workspace forked: /tmp/diamond-card2-workspace/diamond-card2.code-workspace",
    ]);
  });
});

describe("resolveDefaultConfig", () => {
  test("默认读取 HOME 下的 code-ws 配置", () => {
    const cfg = resolveDefaultConfig(
      "file:///repo/devkit-cli/code-ws/src/main.ts",
      "/usr/local/bin/bun",
      {
        HOME: "/home/dev",
      },
    );

    expect(cfg).toBe("/home/dev/.config/code-ws/config.json");
  });

  test("默认优先读取 XDG_CONFIG_HOME 下的 code-ws 配置", () => {
    const cfg = resolveDefaultConfig(
      "file:///$bunfs/root/main.ts",
      "/Users/linran/.local/bin/code-ws",
      {
        HOME: "/Users/linran",
        XDG_CONFIG_HOME: "/tmp/xdg",
      },
    );

    expect(cfg).toBe("/tmp/xdg/code-ws/config.json");
  });
});
