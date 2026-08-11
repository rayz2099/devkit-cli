import { describe, expect, test } from "bun:test";
import {
  createWorkspaceJson,
  createWorkspaceProjectYaml,
  parseWorkspaceProject,
  removeRepoFromWorkspaceJson,
  workspaceNameFromBranch,
} from "./workspace";

describe("workspaceNameFromBranch", () => {
  test("从 feature 分支末段生成 workspace 名称", () => {
    expect(workspaceNameFromBranch("feature/spec101")).toBe("spec101-workspace");
  });

  test("普通分支名直接生成 workspace 名称", () => {
    expect(workspaceNameFromBranch("spec101")).toBe("spec101-workspace");
  });
});

describe("parseWorkspaceProject", () => {
  test("读取 repo 固定 branch", () => {
    const project = parseWorkspaceProject([
      "branch: feature/level",
      "projects:",
      "  - repo-name: my-room",
      "    group: frontend",
      "    branch: feature/room-organizing",
      "",
    ].join("\n"));

    expect(project).toEqual({
      branch: "feature/level",
      repos: [
        "my-room",
      ],
      checkoutBranches: {
        "my-room": "feature/room-organizing",
      },
    });
  });

  test("repo branch 不覆盖 workspace branch", () => {
    const project = parseWorkspaceProject([
      "branch: feature/level",
      "projects:",
      "  - repo-name: norma",
      "    group: backend",
      "  - repo-name: my-room",
      "    group: frontend",
      "    branch: room-organizing",
      "",
    ].join("\n"));

    expect(project.branch).toBe("feature/level");
    expect(project.checkoutBranches).toEqual({
      "my-room": "room-organizing",
    });
  });

  test("兼容读取旧 checkout-branch 字段", () => {
    const project = parseWorkspaceProject([
      "branch: feature/level",
      "projects:",
      "  - repo-name: my-room",
      "    group: frontend",
      "    checkout-branch: feature/room-organizing",
      "",
    ].join("\n"));

    expect(project.checkoutBranches).toEqual({
      "my-room": "feature/room-organizing",
    });
  });
});

describe("createWorkspaceJson", () => {
  test("生成相对路径 folders", () => {
    const json = createWorkspaceJson([
      "app-gw",
      "app-api",
    ]);

    expect(json).toEqual({
      folders: [
        {
          name: "app-gw",
          path: "app-gw",
        },
        {
          name: "app-api",
          path: "app-api",
        },
      ],
      settings: {},
    });
  });
});

describe("removeRepoFromWorkspaceJson", () => {
  test("从 VS Code workspace folders 中移除 repo", () => {
    const json = removeRepoFromWorkspaceJson(
      {
        folders: [
          {
            name: "app-gw",
            path: "app-gw",
          },
          {
            name: "app-api",
            path: "app-api",
          },
        ],
        settings: {},
      },
      "app-api",
    );

    expect(json).toEqual({
      folders: [
        {
          name: "app-gw",
          path: "app-gw",
        },
      ],
      settings: {},
    });
  });

  test("repo 不存在时拒绝重写 workspace", () => {
    expect(() =>
      removeRepoFromWorkspaceJson(
        {
          folders: [
            {
              name: "app-gw",
              path: "app-gw",
            },
          ],
          settings: {},
        },
        "app-api",
      ),
    ).toThrow("repo not found in workspace: app-api");
  });
});

describe("createWorkspaceProjectYaml", () => {
  test("生成包含固定 branch 和项目描述的 project.yml", () => {
    const txt = createWorkspaceProjectYaml(
      "feature/spec101",
      [
        {
          name: "shared-lib",
          path: "/home/dev/projects/backend/shared-lib",
          group: "backend",
          branch: "master",
          description: "是公共的`share-lib`, 定义了`http endpoint` 和 `dubbo endpoint`.",
        },
        {
          name: "app-gw",
          path: "/home/dev/projects/backend/app-gw",
          group: "backend",
          branch: "master",
          checkoutBranch: "feature/room-organizing",
        },
      ],
    );

    expect(txt).toBe([
      "branch: feature/spec101",
      "projects:",
      "  - repo-name: shared-lib",
      "    group: backend",
      "    description: 是公共的`share-lib`, 定义了`http endpoint` 和 `dubbo endpoint`.",
      "  - repo-name: app-gw",
      "    group: backend",
      "    branch: feature/room-organizing",
      "",
    ].join("\n"));
  });

  test("重新生成 project.yml 时保留自定义顶层变量", () => {
    const txt = createWorkspaceProjectYaml(
      "feature/gacha-x",
      [
        {
          name: "dt-script",
          path: "/home/dev/projects/backend/dt-script",
          group: "backend",
          branch: "master",
        },
      ],
      [
        "it-env-no: 044",
      ],
    );

    expect(txt).toBe([
      "branch: feature/gacha-x",
      "it-env-no: 044",
      "projects:",
      "  - repo-name: dt-script",
      "    group: backend",
      "",
    ].join("\n"));
  });
});
