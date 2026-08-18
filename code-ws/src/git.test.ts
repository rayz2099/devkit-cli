import { describe, expect, test } from "bun:test";
import {
  assertDisposableWorktrees,
  buildForkGitPlan,
  buildGitPlan,
  buildSyncBaseBranchPlan,
  buildWorktreeRemovePlan,
  confirmDeletePrompt,
  confirmRemoveProjectPrompt,
  existingDirMsg,
  gitFailureMsg,
  gitStepLabel,
  hasUnsafeWork,
  hasUncommittedWork,
  parseBranchWorktreePath,
  runGitPlanKeepGoing,
  shouldConfirmRemoveProject,
  shouldDeleteExistingDir,
  shouldIgnoreCmdFailure,
  unsafeWorkMsg,
} from "./git";

describe("buildGitPlan", () => {
  test("创建 worktree 前先清理 git 缺失 worktree 注册项", () => {
    const plan = buildGitPlan({
      branch: "feature/spec101",
      baseBranch: "master",
      remote: "origin",
      workspaceDir: "/tmp/spec101-workspace",
      repos: [
        {
          name: "app-gw",
          path: "/src/app-gw",
          group: "backend",
          branch: "master",
        },
      ],
    });

    expect(plan[0]).toEqual({
      repo: "app-gw",
      branch: "workspace",
      step: "worktree-prune",
      cwd: "/src/app-gw",
      args: ["git", "worktree", "prune"],
    });
  });

  test("为每个 repo 生成严格的 git 初始化命令", () => {
    const plan = buildGitPlan({
      branch: "feature/spec101",
      baseBranch: "master",
      remote: "origin",
      workspaceDir: "/tmp/spec101-workspace",
      repos: [
        {
          name: "app-gw",
          path: "/src/app-gw",
          group: "backend",
          branch: "master",
        },
      ],
    });

    expect(plan).toEqual([
      {
        repo: "app-gw",
        branch: "workspace",
        step: "worktree-prune",
        cwd: "/src/app-gw",
        args: ["git", "worktree", "prune"],
      },
      {
        repo: "app-gw",
        branch: "master",
        step: "fetch",
        cwd: "/src/app-gw",
        args: ["git", "fetch", "origin", "master"],
      },
      {
        repo: "app-gw",
        branch: "feature/spec101",
        step: "worktree",
        cwd: "/src/app-gw",
        args: [
          "git",
          "worktree",
          "add",
          "-b",
          "feature/spec101",
          "/tmp/spec101-workspace/app-gw",
          "origin/master",
        ],
      },
    ]);
  });

  test("本地分支已存在时直接创建 worktree, 不重复创建 branch", () => {
    const plan = buildGitPlan({
      branch: "feature/spec101",
      baseBranch: "master",
      remote: "origin",
      workspaceDir: "/tmp/spec101-workspace",
      branchExists: () => true,
      repos: [
        {
          name: "app-gw",
          path: "/src/app-gw",
          group: "backend",
          branch: "master",
        },
      ],
    });

    expect(plan.at(-1)).toEqual({
      repo: "app-gw",
      branch: "feature/spec101",
      step: "worktree",
      cwd: "/src/app-gw",
      args: [
        "git",
        "worktree",
        "add",
        "/tmp/spec101-workspace/app-gw",
        "feature/spec101",
      ],
    });
  });

  test("repo 声明主分支时使用 repo branch 初始化 worktree", () => {
    const plan = buildGitPlan({
      branch: "feature/spec101",
      baseBranch: "master",
      remote: "origin",
      workspaceDir: "/tmp/spec101-workspace",
      repos: [
        {
          name: "editor-tool",
          path: "/src/editor-tool",
          group: "product",
          branch: "main",
        },
      ],
    });

    expect(plan).toEqual([
      {
        repo: "editor-tool",
        branch: "workspace",
        step: "worktree-prune",
        cwd: "/src/editor-tool",
        args: ["git", "worktree", "prune"],
      },
      {
        repo: "editor-tool",
        branch: "main",
        step: "fetch",
        cwd: "/src/editor-tool",
        args: ["git", "fetch", "origin", "main"],
      },
      {
        repo: "editor-tool",
        branch: "feature/spec101",
        step: "worktree",
        cwd: "/src/editor-tool",
        args: [
          "git",
          "worktree",
          "add",
          "-b",
          "feature/spec101",
          "/tmp/spec101-workspace/editor-tool",
          "origin/main",
        ],
      },
    ]);
  });

  test("repo 声明 checkout branch 时使用 detached worktree", () => {
    const plan = buildGitPlan({
      branch: "feature/level",
      baseBranch: "master",
      remote: "origin",
      workspaceDir: "/tmp/level-workspace",
      repos: [
        {
          name: "my-room",
          path: "/src/my-room",
          group: "frontend",
          branch: "master",
          checkoutBranch: "feature/room-organizing",
        },
      ],
    });

    expect(plan).toEqual([
      {
        repo: "my-room",
        branch: "workspace",
        step: "worktree-prune",
        cwd: "/src/my-room",
        args: ["git", "worktree", "prune"],
      },
      {
        repo: "my-room",
        branch: "feature/room-organizing",
        step: "fetch",
        cwd: "/src/my-room",
        args: ["git", "fetch", "origin", "feature/room-organizing"],
      },
      {
        repo: "my-room",
        branch: "feature/room-organizing",
        step: "worktree",
        cwd: "/src/my-room",
        args: [
          "git",
          "worktree",
          "add",
          "--detach",
          "/tmp/level-workspace/my-room",
          "origin/feature/room-organizing",
        ],
      },
    ]);
  });
});

describe("buildSyncBaseBranchPlan", () => {
  test("为每个 repo 生成本地 master 同步命令", () => {
    const plan = buildSyncBaseBranchPlan({
      baseBranch: "master",
      remote: "origin",
      branchWorktreePath: () => undefined,
      repos: [
        {
          name: "app-gw",
          path: "/src/app-gw",
          group: "backend",
          branch: "master",
        },
      ],
    });

    expect(plan).toEqual([
      {
        repo: "app-gw",
        branch: "master",
        step: "sync",
        cwd: "/src/app-gw",
        args: ["git", "fetch", "origin", "master:master"],
      },
    ]);
  });

  test("本地 master 已被 worktree checkout 时在该 worktree 内快进拉取", () => {
    const plan = buildSyncBaseBranchPlan({
      baseBranch: "master",
      remote: "origin",
      branchWorktreePath: () => "/src/app-gw",
      repos: [
        {
          name: "app-gw",
          path: "/src/app-gw",
          group: "backend",
          branch: "master",
        },
      ],
    });

    expect(plan).toEqual([
      {
        repo: "app-gw",
        branch: "master",
        step: "sync",
        cwd: "/src/app-gw",
        args: ["git", "pull", "--ff-only", "origin", "master"],
      },
    ]);
  });

  test("repo 声明主分支时同步对应本地分支", () => {
    const plan = buildSyncBaseBranchPlan({
      baseBranch: "master",
      remote: "origin",
      branchWorktreePath: () => undefined,
      repos: [
        {
          name: "editor-tool",
          path: "/src/editor-tool",
          group: "product",
          branch: "main",
        },
      ],
    });

    expect(plan).toEqual([
      {
        repo: "editor-tool",
        branch: "main",
        step: "sync",
        cwd: "/src/editor-tool",
        args: ["git", "fetch", "origin", "main:main"],
      },
    ]);
  });

  test("repo 声明 checkout branch 时同步固定分支", () => {
    const plan = buildSyncBaseBranchPlan({
      baseBranch: "master",
      remote: "origin",
      branchWorktreePath: () => undefined,
      repos: [
        {
          name: "my-room",
          path: "/src/my-room",
          group: "frontend",
          branch: "master",
          checkoutBranch: "feature/room-organizing",
        },
      ],
    });

    expect(plan).toEqual([
      {
        repo: "my-room",
        branch: "feature/room-organizing",
        step: "sync",
        cwd: "/src/my-room",
        args: [
          "git",
          "fetch",
          "origin",
          "feature/room-organizing:feature/room-organizing",
        ],
      },
    ]);
  });
});

describe("parseBranchWorktreePath", () => {
  test("从 git worktree porcelain 输出解析指定分支路径", () => {
    const path = parseBranchWorktreePath(
      [
        "worktree /src/app-gw",
        "HEAD e1",
        "branch refs/heads/master",
        "",
        "worktree /tmp/gacha-x-workspace/app-gw",
        "HEAD e2",
        "branch refs/heads/feature/gacha-x",
      ].join("\n"),
      "master",
    );

    expect(path).toBe("/src/app-gw");
  });
});

describe("buildWorktreeRemovePlan", () => {
  test("workspace 已存在时按 repo 生成 git worktree remove 命令", () => {
    const plan = buildWorktreeRemovePlan(
      "/tmp/spec101-workspace",
      [
        {
          name: "app-gw",
          path: "/src/app-gw",
          group: "backend",
          branch: "master",
        },
      ],
    );

    expect(plan).toEqual([
      {
        repo: "app-gw",
        branch: "workspace",
        step: "worktree-remove",
        cwd: "/src/app-gw",
        args: [
          "git",
          "worktree",
          "remove",
          "--force",
          "/tmp/spec101-workspace/app-gw",
        ],
        ignoredErrors: ["is not a working tree"],
      },
    ]);
  });

  test("worktree remove 遇到普通残留目录时允许继续清理", () => {
    const ignored = shouldIgnoreCmdFailure(
      {
        repo: "app-gw",
        branch: "workspace",
        step: "worktree-remove",
        cwd: "/src/app-gw",
        args: ["git", "worktree", "remove"],
        ignoredErrors: ["is not a working tree"],
      },
      "fatal: '/tmp/spec101-workspace/app-gw' is not a working tree",
    );

    expect(ignored).toBe(true);
  });
});

describe("buildForkGitPlan", () => {
  test("从源 workspace worktree 的 HEAD 派生新 worktree", () => {
    const plan = buildForkGitPlan({
      srcWorkspaceDir: "/tmp/diamond-card-workspace",
      dstWorkspaceDir: "/tmp/diamond-card2-workspace",
      dstBranch: "feature/diamond-card2",
      repos: [
        {
          name: "app-api",
          path: "/src/app-api",
          group: "backend",
          branch: "master",
        },
      ],
    });

    expect(plan).toEqual([
      {
        repo: "app-api",
        branch: "workspace",
        step: "worktree-prune",
        cwd: "/src/app-api",
        args: ["git", "worktree", "prune"],
      },
      {
        repo: "app-api",
        branch: "feature/diamond-card2",
        step: "worktree-fork",
        cwd: "/tmp/diamond-card-workspace/app-api",
        args: [
          "git",
          "worktree",
          "add",
          "-b",
          "feature/diamond-card2",
          "/tmp/diamond-card2-workspace/app-api",
          "HEAD",
        ],
      },
    ]);
  });

  test("固定 checkout branch 的 repo fork 时复用 detached HEAD", () => {
    const plan = buildForkGitPlan({
      srcWorkspaceDir: "/tmp/level-workspace",
      dstWorkspaceDir: "/tmp/level2-workspace",
      dstBranch: "feature/level2",
      repos: [
        {
          name: "my-room",
          path: "/src/my-room",
          group: "frontend",
          branch: "master",
          checkoutBranch: "room-organizing",
        },
      ],
    });

    expect(plan).toEqual([
      {
        repo: "my-room",
        branch: "workspace",
        step: "worktree-prune",
        cwd: "/src/my-room",
        args: ["git", "worktree", "prune"],
      },
      {
        repo: "my-room",
        branch: "room-organizing",
        step: "worktree-fork",
        cwd: "/tmp/level-workspace/my-room",
        args: [
          "git",
          "worktree",
          "add",
          "--detach",
          "/tmp/level2-workspace/my-room",
          "HEAD",
        ],
      },
    ]);
  });
});

describe("workspace cleanup safety", () => {
  test("刷新远端目标分支后允许删除已合并 worktree", () => {
    const cmds: string[][] = [];

    assertDisposableWorktrees(
      "/tmp/spec101-workspace",
      [
        {
          name: "app-api",
          path: "/src/app-api",
          group: "backend",
          branch: "master",
        },
      ],
      "origin",
      "master",
      {
        pathExists: () => true,
        isTree: () => true,
        gitOut: (_cwd, args) => {
          cmds.push(args);
          return "";
        },
        isMerged: (_cwd, head, target) =>
          head === "HEAD" && target === "origin/master",
      },
    );

    expect(cmds).toEqual([
      ["git", "status", "--porcelain"],
      ["git", "fetch", "--quiet", "origin", "master"],
    ]);
  });

  test("HEAD 未合并到最新远端目标分支时中断", () => {
    expect(() =>
      assertDisposableWorktrees(
        "/tmp/spec101-workspace",
        [
          {
            name: "app-api",
            path: "/src/app-api",
            group: "backend",
            branch: "master",
          },
        ],
        "origin",
        "master",
        {
          pathExists: () => true,
          isTree: () => true,
          gitOut: (_cwd, args) =>
            args.includes("rev-list") ? "2\n" : "",
          isMerged: () => false,
        },
      )
    ).toThrow("reason: unmerged commits: 2");
  });

  test("存在未提交改动时判定为不可删除", () => {
    const unsafe = hasUnsafeWork(
      " M server/App.kt\n",
      "0\n",
    );

    expect(unsafe).toBe(true);
  });

  test("存在本地 commit 时判定为不可删除", () => {
    const unsafe = hasUnsafeWork(
      "",
      "2\n",
    );

    expect(unsafe).toBe(true);
  });

  test("没有未提交改动且没有本地 commit 时允许删除", () => {
    const unsafe = hasUnsafeWork(
      "",
      "0\n",
    );

    expect(unsafe).toBe(false);
  });

  test("fork 只把未提交状态判定为不安全", () => {
    expect(hasUncommittedWork(" M server/App.kt\n")).toBe(true);
    expect(hasUncommittedWork("")).toBe(false);
  });

  test("不可删除原因包含 repo 和路径", () => {
    const msg = unsafeWorkMsg(
      "app-api",
      "/tmp/spec101-workspace/app-api",
      "local commits: 2",
    );

    expect(msg).toBe(
      "workspace cleanup aborted: repo=app-api path=/tmp/spec101-workspace/app-api\n" +
        "reason: local commits: 2",
    );
  });
});

describe("git log format", () => {
  test("workspace 已存在时提示用户可交互删除", () => {
    const msg = existingDirMsg(
      "workspace dir",
      "/Users/linran/projects/workspaces/ws-001-workspace",
    );

    expect(msg).toBe(
      "workspace dir already exists: /Users/linran/projects/workspaces/ws-001-workspace\n" +
        "delete it and continue? [y/N]",
    );
  });

  test("只接受 y 或 Y 确认删除已存在目录", () => {
    expect(shouldDeleteExistingDir("y")).toBe(true);
    expect(shouldDeleteExistingDir("Y")).toBe(true);
    expect(shouldDeleteExistingDir("yes")).toBe(false);
    expect(shouldDeleteExistingDir("")).toBe(false);
  });

  test("只接受 y 或 Y 确认移除项目", () => {
    expect(shouldConfirmRemoveProject("y")).toBe(true);
    expect(shouldConfirmRemoveProject("Y")).toBe(true);
    expect(shouldConfirmRemoveProject("yes")).toBe(false);
    expect(shouldConfirmRemoveProject("")).toBe(false);
  });

  test("生成交互删除问题", () => {
    const msg = confirmDeletePrompt(
      "/Users/linran/projects/workspaces/ws-001-workspace",
    );

    expect(msg).toBe(
      "workspace already exists: /Users/linran/projects/workspaces/ws-001-workspace\n" +
        "delete it and continue? [y/N] ",
    );
  });

  test("生成移除项目确认问题并说明后果", () => {
    const msg = confirmRemoveProjectPrompt(
      "app-api",
      "/tmp/spec101-workspace/app-api",
      "master",
    );

    expect(msg).toBe(
      "remove repo from workspace: app-api\n" +
        "worktree path: /tmp/spec101-workspace/app-api\n" +
        "effects:\n" +
        "  - runs git worktree remove --force for this repo\n" +
        "  - removes it from project.yml and .code-workspace folders\n" +
        "  - does not merge, push, or delete the git branch\n" +
        "confirm you have merged the branch into master. continue? [y/N] ",
    );
  });

  test("普通日志使用项目和分支描述当前步骤", () => {
    const msg = gitStepLabel({
      repo: "app-api",
      branch: "master",
      step: "checkout",
      cwd: "/src/app-api",
      args: ["git", "checkout", "master"],
    });

    expect(msg).toBe("[app-api] checkout master");
  });

  test("失败日志包含项目、分支、步骤和 git 错误摘要", () => {
    const msg = gitFailureMsg(
      {
        repo: "app-api",
        branch: "master",
        step: "checkout",
        cwd: "/src/app-api",
        args: ["git", "checkout", "master"],
      },
      "error: Your local changes would be overwritten\nAborting\n",
    );

    expect(msg).toBe(
      "git failed: repo=app-api branch=master step=checkout\n" +
        "reason: error: Your local changes would be overwritten",
    );
  });

  test("失败摘要跳过 git worktree 的普通进度行", () => {
    const msg = gitFailureMsg(
      {
        repo: "app-gw",
        branch: "feature/ws-001",
        step: "worktree",
        cwd: "/src/app-gw",
        args: ["git", "worktree", "add"],
      },
      [
        "Preparing worktree (new branch 'feature/ws-001')",
        "fatal: a branch named 'feature/ws-001' already exists",
      ].join("\n"),
    );

    expect(msg).toBe(
      "git failed: repo=app-gw branch=feature/ws-001 step=worktree\n" +
        "reason: fatal: a branch named 'feature/ws-001' already exists",
    );
  });
});

describe("runGitPlanKeepGoing", () => {
  test("单个命令失败后继续执行后续命令并返回失败数", () => {
    const called: string[] = [];
    const failures = runGitPlanKeepGoing(
      [
        {
          repo: "a",
          branch: "master",
          step: "sync",
          cwd: "/src/a",
          args: ["git", "fetch"],
        },
        {
          repo: "b",
          branch: "master",
          step: "sync",
          cwd: "/src/b",
          args: ["git", "fetch"],
        },
      ],
      false,
      (cmd) => {
        called.push(cmd.repo);
        if (cmd.repo === "a") {
          throw new Error("failed a");
        }
      },
      () => undefined,
    );

    expect(called).toEqual(["a", "b"]);
    expect(failures).toBe(1);
  });
});
