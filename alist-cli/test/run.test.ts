import { afterEach, expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { AlistApi } from "../src/client";
import { runCmd } from "../src/run";
import type { FsEntry, PutTask } from "../src/types";

const origHome = process.env.HOME;
const homes: string[] = [];
const files: string[] = [];
const noEnv: NodeJS.ProcessEnv = {};

afterEach(() => {
  for (const dir of [...homes, ...files]) {
    rmSync(dir, { recursive: true, force: true });
  }
  homes.length = 0;
  files.length = 0;
  if (origHome !== undefined) {
    process.env.HOME = origHome;
  } else {
    delete process.env.HOME;
  }
});

function withHome(): void {
  const home = mkdtempSync(join(tmpdir(), "alist-cli-"));
  homes.push(home);
  mkdirSync(join(home, ".config", "alist-cli"), { recursive: true });
  writeFileSync(
    join(home, ".config", "alist-cli", "config.json"),
    JSON.stringify({
      defaultProfile: "home",
      profiles: [{
        name: "home",
        url: "http://alist.test",
        username: "admin",
        password: "secret",
      }],
    }),
  );
  process.env.HOME = home;
}

function tmpTree(): { root: string; a: string; b: string } {
  const root = mkdtempSync(join(tmpdir(), "alist-sync-"));
  files.push(root);
  mkdirSync(join(root, "sub"));
  const a = join(root, "a.txt");
  const b = join(root, "sub", "b.txt");
  writeFileSync(a, "a");
  writeFileSync(b, "b");
  return { root, a, b };
}

const sample: FsEntry = {
  name: "a.txt",
  size: 3,
  isDir: false,
  modified: "2024-01-01T00:00:00.000Z",
  sign: "",
  thumb: "",
  type: 2,
};

test("help 不读配置", async () => {
  const out = await runCmd(["help"]);
  expect(out).toContain("alist-cli");
  expect(out).toContain("sync <src> <dst>");
  expect(out).toContain("ALIST_ADDRESS");
});

test("ls human 表格, agent JSON", async () => {
  withHome();
  const fake: AlistApi = {
    listDir: async () => [sample],
    putFile: async () => undefined,
    mkdir: async () => undefined,
  };
  const human = await runCmd(["ls", "/disk"], () => fake, noEnv);
  expect(human).toContain("NAME");
  expect(human).toContain("a.txt");
  expect(human).toContain("file");
  const agent = await runCmd(["agent", "ls", "/disk"], () => fake, noEnv);
  expect(JSON.parse(agent)).toEqual([sample]);
});

test("put human ok, agent 结构化", async () => {
  withHome();
  const { a } = tmpTree();
  const fake: AlistApi = {
    listDir: async () => [],
    putFile: async () => undefined,
    mkdir: async () => undefined,
  };
  const human = await runCmd(["put", a, "/disk/a.txt"], () => fake, noEnv);
  expect(human).toBe("ok /disk/a.txt\n");
  const agent = await runCmd(["agent", "put", a, "/disk/a.txt"], () => fake, noEnv);
  expect(JSON.parse(agent)).toMatchObject({ src: a, dst: "/disk/a.txt" });
});

test("put task.error 失败", async () => {
  withHome();
  const { a } = tmpTree();
  const task: PutTask = {
    id: "t1",
    name: "a",
    progress: 0,
    state: 0,
    status: "failed",
    error: "boom",
  };
  const fake: AlistApi = {
    listDir: async () => [],
    putFile: async () => task,
    mkdir: async () => undefined,
  };
  await expect(runCmd(["put", a, "/disk/a.txt", "--as-task"], () => fake, noEnv)).rejects.toThrow("boom");
});

test("sync 递归 put 每个文件", async () => {
  withHome();
  const { root, a, b } = tmpTree();
  const puts: Array<{ src: string; dst: string }> = [];
  const fake: AlistApi = {
    listDir: async () => [],
    putFile: async (src, dst) => {
      puts.push({ src, dst });
      return undefined;
    },
    mkdir: async () => undefined,
  };
  const out = await runCmd(["sync", root, "/disk"], () => fake, noEnv);
  expect(puts).toEqual([
    { src: a, dst: "/disk/a.txt" },
    { src: b, dst: "/disk/sub/b.txt" },
  ]);
  expect(out).toContain("uploaded 2 files");
});

test("mkdir agent JSON", async () => {
  withHome();
  const fake: AlistApi = {
    listDir: async () => [],
    putFile: async () => undefined,
    mkdir: async () => undefined,
  };
  const out = await runCmd(["agent", "mkdir", "/disk/x"], () => fake, noEnv);
  expect(JSON.parse(out)).toEqual({ path: "/disk/x", ok: true });
});

test("无 config 文件时走 ALIST_*", async () => {
  const home = mkdtempSync(join(tmpdir(), "alist-cli-"));
  homes.push(home);
  process.env.HOME = home;
  let url = "";
  const fake: AlistApi = {
    listDir: async () => [],
    putFile: async () => undefined,
    mkdir: async () => undefined,
  };
  await runCmd(
    ["ls"],
    (runtime) => {
      url = runtime.profile.url;
      return fake;
    },
    {
      ALIST_ADDRESS: "http://from-env",
      ALIST_USERNAME: "env-user",
      ALIST_PASSWORD: "env-pass",
    },
  );
  expect(url).toBe("http://from-env");
});
