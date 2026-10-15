import { describe, expect, test } from "bun:test";
import { applyMergeSpecs, parseFromSpecs } from "./merge";
import type { RepoCfg } from "./types";

const room: RepoCfg = {
  name: "my-room",
  path: "/src/my-room",
  group: "frontend",
  branch: "master",
};

const norma: RepoCfg = {
  name: "norma",
  path: "/src/norma",
  group: "backend",
  branch: "master",
};

describe("parseFromSpecs", () => {
  test("解析多个 repo=branch", () => {
    expect(parseFromSpecs([
      "my-room=V0.0.1",
      "norma-content=release/1",
    ])).toEqual({
      "my-room": "V0.0.1",
      "norma-content": "release/1",
    });
  });

  test("同一个仓库重复时拒绝", () => {
    expect(() => parseFromSpecs([
      "my-room=V0.0.1",
      "my-room=V0.0.2",
    ])).toThrow("--from repeated for repo: my-room");
  });

  test("缺少等号或分支名时拒绝", () => {
    expect(() => parseFromSpecs(["my-room"])).toThrow("--from requires repo=branch: my-room");
    expect(() => parseFromSpecs(["my-room="])).toThrow("--from requires repo=branch: my-room=");
  });
});

describe("applyMergeSpecs", () => {
  test("指定仓库改写 merge，其余写成自己的主分支", () => {
    const repos = applyMergeSpecs(
      [room, norma],
      { "my-room": "v0.0.1" },
      (_repo, ref) => ref.toUpperCase(),
    );

    expect(repos.map((repo) => [repo.name, repo.merge])).toEqual([
      ["my-room", "V0.0.1"],
      ["norma", "master"],
    ]);
  });

  test("保留已记录的 merge", () => {
    const repos = applyMergeSpecs(
      [{ ...room, merge: "V0.0.1" }, norma],
      {},
      () => {
        throw new Error("resolve should not run");
      },
    );

    expect(repos[0]?.merge).toBe("V0.0.1");
    expect(repos[1]?.merge).toBe("master");
  });

  test("仓库不在本次选择里时拒绝", () => {
    expect(() => applyMergeSpecs(
      [norma],
      { "my-room": "V0.0.1" },
      (_repo, ref) => ref,
    )).toThrow("--from repo is not selected: my-room");
  });
});
