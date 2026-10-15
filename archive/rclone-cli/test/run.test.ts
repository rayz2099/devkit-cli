import { expect, test } from "bun:test";
import { appendFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { main } from "../src/cli";
import { runsFile } from "../src/config";
import { readRuns } from "../src/runs";
import { runCmd } from "../src/run";
import { RcloneErr } from "../src/types";
import type { SpawnFn } from "../src/types";
import { sampleCfg, samplePair, stubSpawn, tempHome, testDeps, writeCfg } from "./fixture";

const plain = "p@ss,w:rd='x'\"y";

function writeCopyDest(args: string[]): void {
  const dest = args[4];
  if (dest === undefined || dest === "") {
    return;
  }
  mkdirSync(dirname(dest), { recursive: true });
  writeFileSync(dest, "ok");
}

test("push with no task exits 2 and lists task names", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  const deps = testDeps(home, stubSpawn(0).spawn);
  const chunks: string[] = [];
  const orig = process.stderr.write.bind(process.stderr);
  process.stderr.write = ((chunk: string | Uint8Array) => {
    chunks.push(String(chunk));
    return true;
  }) as typeof process.stderr.write;
  try {
    const code = await main(["push"], deps);
    expect(code).toBe(2);
    const text = chunks.join("");
    expect(text).toContain("photograph-123");
    expect(text).toContain("no default task");
  } finally {
    process.stderr.write = orig;
  }
  const runs = await readRuns(home);
  expect(runs).toHaveLength(0);
});

test("failed push still writes a run and never calls sync", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  const stub = stubSpawn(7, "boom\n");
  const out = await runCmd(["push", "photograph-123"], testDeps(home, stub.spawn));
  expect(out.code).toBe(7);
  expect(out.text).toContain("failed (7)");
  expect(stub.copies).toHaveLength(1);
  const argv = stub.copies[0] ?? [];
  expect(argv[2]).toBe("copy");
  expect(argv.includes("sync")).toBe(false);
  expect(argv.join("\n")).not.toContain("rclone/rclone.conf");
  expect(argv.join("\n")).not.toContain(plain);
  const cfgArg = argv[argv.indexOf("--config") + 1] ?? "";
  expect(cfgArg).toEndWith("/rclone-cli/empty.conf");
  expect(readFileSync(cfgArg, "utf8")).toBe("");
  const runs = await readRuns(home);
  expect(runs).toHaveLength(1);
  expect(runs[0]?.code).toBe(7);
  expect(runs[0]?.task).toBe("photograph-123");
  expect(runs[0]?.error).toContain("boom");
  expect(runs[0]?.args.join("\n")).not.toContain(plain);
  expect(runs[0]?.remote).toBe("123:photograph");
});

test("dry-run agent has no progress and records the run", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  const stderr = [
    "Transferred:   \t 0 B / 0 B, -, 0 B/s, ETA -",
    "Checks:                 4 / 4, -",
    "Transferred:            0 / 0, -",
  ].join("\n");
  const stub = stubSpawn(0, stderr);
  const out = await runCmd(
    ["agent", "push", "photograph-123", "album", "--dry-run"],
    testDeps(home, stub.spawn),
  );
  expect(out.code).toBe(0);
  const body = JSON.parse(out.text) as { dryRun: boolean; checks?: number; remote: string };
  expect(body.dryRun).toBe(true);
  expect(body.checks).toBe(4);
  expect(body.remote).toBe("123:photograph/album");
  const argv = stub.copies[0] ?? [];
  expect(argv).toContain("--dry-run");
  expect(argv).not.toContain("--progress");
  expect(argv.includes("sync")).toBe(false);
  expect(argv[3]).toContain("/album");
  expect(argv[4]?.endsWith(":photograph/album")).toBe(true);
});

test("ls does not spawn rclone and hides the password", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  let called = false;
  const spawn: SpawnFn = async () => {
    called = true;
    return { code: 0, stdout: "", stderr: "" };
  };
  const out = await runCmd(["ls"], testDeps(home, spawn));
  expect(called).toBe(false);
  expect(out.code).toBe(0);
  expect(out.text).toContain("photograph-123");
  expect(out.text).not.toContain(plain);
});

test("path outside the source does not transfer", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  const stub = stubSpawn(0);
  const out = await runCmd(["push", "photograph-123", "/etc"], testDeps(home, stub.spawn));
  expect(out.code).toBe(1);
  expect(out.text).toContain("outside");
  expect(stub.copies).toHaveLength(0);
  const runs = await readRuns(home);
  expect(runs).toHaveLength(1);
  expect(runs[0]?.code).toBe(1);
});

test("pull copies and check lists with lsjson", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  const stub = stubSpawn(0);
  await runCmd(["pull", "photograph-123"], testDeps(home, stub.spawn));
  const checked = await runCmd(["check", "photograph-123"], testDeps(home, stub.spawn));
  const pull = stub.copies[0] ?? [];
  expect(pull[2]).toBe("copy");
  expect(pull[3]?.startsWith(":webdav")).toBe(true);
  expect(pull[4]).toBe("/tmp/photograph");
  expect(pull.includes("sync")).toBe(false);
  expect(checked.text).toContain("check photograph-123");
  expect(checked.text).toContain("ok");
  const lists = stub.copies.filter((item) => item[2] === "lsjson");
  expect(lists.length).toBeGreaterThanOrEqual(2);
  expect(lists[0]?.includes("sync")).toBe(false);
  expect(lists[0]).toContain("--recursive");
  expect(lists[0]).not.toContain("--progress");
});

test("runs prints newest first", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  const stub = stubSpawn(0);
  let n = 0;
  const deps = testDeps(home, stub.spawn);
  deps.id = () => {
    n += 1;
    return `run-${n}`;
  };
  await runCmd(["push", "photograph-123"], deps);
  await runCmd(["check", "photograph-123"], deps);
  const out = await runCmd(["runs"], deps);
  const lines = out.text.split("\n");
  expect(lines[0]).toContain("check");
  expect(lines[1]).toContain("push");
});

test("corrupt run record fails closed", async () => {
  const home = tempHome();
  const path = runsFile(home);
  mkdirSync(join(home, ".local", "share", "rclone-cli", "runs"), { recursive: true });
  appendFileSync(path, "{not json}\n");
  await expect(readRuns(home)).rejects.toThrow("runs.jsonl line 1 is invalid");
});

test("pull of a file tells the user to download", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  const stub = stubSpawn(0);
  const out = await runCmd(
    ["pull", "photograph-123", "album/DSC_0001.NEF"],
    testDeps(home, stub.spawn),
  );
  expect(out.code).toBe(2);
  expect(out.text).toContain("download one file");
  expect(stub.copies).toHaveLength(0);
});

test("download writes the basename into cwd", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  const stub = stubSpawn(0);
  const deps = testDeps(home, stub.spawn);
  deps.cwd = "/tmp/x";
  const out = await runCmd(["download", "photograph-123", "album/DSC_0746.JPG"], deps);
  expect(out.code).toBe(0);
  expect(stub.copies[0]?.[2]).toBe("copyto");
  expect(stub.copies[0]?.[4]).toBe("/tmp/x/DSC_0746.JPG");
  expect(stub.copies[0]).not.toContain("--exclude");
  expect(stub.copies[0]).toContain("-q");
  expect(stub.copies[0]).not.toContain("--progress");
});

test("download missing remote file says not found", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({
    pass: plain,
    crypt: [samplePair()],
  }));
  const copies: string[][] = [];
  const spawn: SpawnFn = async (_bin, args) => {
    if (args[2] === "obscure") {
      return { code: 0, stdout: "obscured\n", stderr: "" };
    }
    copies.push(args);
    return { code: 0, stdout: "[]\n", stderr: "" };
  };
  const out = await runCmd(
    ["download", "photograph-123", "album/missing.JPG"],
    testDeps(home, spawn),
  );
  expect(out.code).toBe(1);
  expect(out.text).toContain("remote file not found");
  expect(copies.every((item) => item[2] !== "copyto")).toBe(true);
});

test("get without a path exits 2", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  const chunks: string[] = [];
  const orig = process.stderr.write.bind(process.stderr);
  process.stderr.write = ((chunk: string | Uint8Array) => {
    chunks.push(String(chunk));
    return true;
  }) as typeof process.stderr.write;
  try {
    const code = await main(["get", "photograph-123"], testDeps(home, stubSpawn(0).spawn));
    expect(code).toBe(2);
    expect(chunks.join("")).toContain("get requires a path");
  } finally {
    process.stderr.write = orig;
  }
});

test("get fetches ciphertext once then decrypts locally", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({
    pass: plain,
    crypt: [samplePair()],
  }));
  const copies: string[][] = [];
  const spawn: SpawnFn = async (_bin, args) => {
    if (args[2] === "obscure") {
      return { code: 0, stdout: "obscured\n", stderr: "" };
    }
    copies.push(args);
    if (args[2] === "lsjson") {
      return {
        code: 0,
        stdout: JSON.stringify([{ Path: "DSC_0001.NEF.bin", Size: 1, IsDir: false }]),
        stderr: "",
      };
    }
    if (args[2] === "copyto") {
      writeCopyDest(args);
    }
    return { code: 0, stdout: "", stderr: "Transferred:            1 / 1, 100%\n" };
  };
  const dest = join(home, "DSC_0001.NEF");
  const out = await runCmd(
    ["get", "photograph-123", "album/DSC_0001.NEF", dest],
    testDeps(home, spawn),
  );
  expect(out.code).toBe(0);
  expect(copies[0]?.[2]).toBe("lsjson");
  expect(copies[0]?.[3]?.startsWith(":webdav")).toBe(true);
  expect(copies[0]?.[copies[0].indexOf("--retries") + 1]).toBe("1");
  const copytos = copies.filter((item) => item[2] === "copyto");
  expect(copytos).toHaveLength(2);
  expect(copytos[0]?.[3]?.startsWith(":webdav")).toBe(true);
  expect(copytos[0]?.[3]).toContain("DSC_0001.NEF.bin");
  expect(copytos[0]).toContain("-q");
  expect(copytos[1]?.[3]?.startsWith(":crypt")).toBe(true);
  expect(copytos[1]?.join("\n")).toContain("suffix='none'");
  expect(existsSync(dest)).toBe(true);
});

test("get retries the next crypt pair after decrypt fails", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({
    pass: plain,
    crypt: [samplePair(), samplePair({ name: "legacy", key: "old", salt: "s", default: false })],
  }));
  const copies: string[][] = [];
  const spawn: SpawnFn = async (_bin, args, sink) => {
    if (args[2] === "obscure") {
      return { code: 0, stdout: "obscured\n", stderr: "" };
    }
    copies.push(args);
    if (args[2] === "lsjson") {
      return {
        code: 0,
        stdout: JSON.stringify([{ Path: "DSC_0001.NEF", Size: 1, IsDir: false }]),
        stderr: "",
      };
    }
    const src = args[3] ?? "";
    if (args[2] === "copyto" && src.startsWith(":crypt")) {
      const crypts = copies.filter((item) => item[2] === "copyto" && (item[3] ?? "").startsWith(":crypt"));
      if (crypts.length === 1) {
        sink.writeErr("NOTICE: Failed to copyto: failed to authenticate decrypted block - bad password?\n");
        return { code: 1, stdout: "", stderr: "secretbox authentication failed\n" };
      }
    }
    if (args[2] === "copyto") {
      writeCopyDest(args);
    }
    return { code: 0, stdout: "", stderr: "Transferred:            1 / 1, 100%\n" };
  };
  const dest = join(home, "DSC_0001.NEF");
  const out = await runCmd(
    ["get", "photograph-123", "album/DSC_0001.NEF", dest],
    testDeps(home, spawn),
  );
  expect(out.code).toBe(0);
  const copytos = copies.filter((item) => item[2] === "copyto");
  expect(copytos.filter((item) => (item[3] ?? "").startsWith(":webdav"))).toHaveLength(1);
  expect(copytos.filter((item) => (item[3] ?? "").startsWith(":crypt"))).toHaveLength(2);
  expect(copytos[0]?.[copytos[0].indexOf("--retries") + 1]).toBe("1");
  expect(existsSync(dest)).toBe(true);
});

test("download lists over webdav once and does not fetch twice", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({
    pass: plain,
    crypt: [samplePair(), samplePair({ name: "legacy", key: "old", salt: "s", default: false })],
  }));
  const copies: string[][] = [];
  const spawn: SpawnFn = async (_bin, args) => {
    if (args[2] === "obscure") {
      return { code: 0, stdout: "obscured\n", stderr: "" };
    }
    copies.push(args);
    if (args[2] === "lsjson") {
      return {
        code: 0,
        stdout: JSON.stringify([{ Path: "DSC_0746.JPG.bin", Size: 1, IsDir: false }]),
        stderr: "",
      };
    }
    if (args[2] === "copyto") {
      writeCopyDest(args);
    }
    return { code: 0, stdout: "", stderr: "Transferred:            1 / 1, 100%\n" };
  };
  const deps = testDeps(home, spawn);
  deps.cwd = home;
  const out = await runCmd(
    ["download", "photograph-123", "2025-04-04/DSC_0746.JPG.bin"],
    deps,
  );
  expect(out.code).toBe(0);
  expect(copies.filter((item) => item[2] === "lsjson")).toHaveLength(1);
  expect(copies[0]?.[3]?.startsWith(":webdav")).toBe(true);
  const fetches = copies.filter((item) => item[2] === "copyto" && (item[3] ?? "").startsWith(":webdav"));
  expect(fetches).toHaveLength(1);
  expect(fetches[0]?.[fetches[0].indexOf("--retries") + 1]).toBe("1");
});

test("check treats leftover .bin as present and extras do not fail", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({
    pass: plain,
    crypt: [samplePair()],
  }));
  const spawn: SpawnFn = async (_bin, args) => {
    if (args[2] === "obscure") {
      return { code: 0, stdout: "obscured\n", stderr: "" };
    }
    const root = args[3] ?? "";
    if (root.startsWith("/")) {
      return {
        code: 0,
        stdout: JSON.stringify([
          { Path: "a.NEF", Size: 10, IsDir: false },
          { Path: "b.NEF", Size: 20, IsDir: false },
        ]),
        stderr: "",
      };
    }
    return {
      code: 0,
      stdout: JSON.stringify([
        { Path: "a.NEF", Size: 10, IsDir: false },
        { Path: "b.NEF.bin", Size: 20, IsDir: false },
        { Path: "cloud-only.JPG", Size: 3, IsDir: false },
      ]),
      stderr: "",
    };
  };
  const out = await runCmd(["check", "photograph-123"], testDeps(home, spawn));
  expect(out.code).toBe(0);
  expect(out.text).toContain("ok-bin");
  expect(out.text).toContain("extra");
  expect(out.text).toContain("cloud-only.JPG");
  const listed = await runCmd(["ls", "photograph-123"], testDeps(home, spawn));
  expect(listed.code).toBe(0);
  expect(listed.text).toContain("a.NEF");
  expect(listed.text).toContain("b.NEF");
  expect(listed.text).toContain("limit=10");
  const one = await runCmd(["ls", "photograph-123", "-n", "1"], testDeps(home, spawn));
  expect(one.text).toContain("showing 1/3");
  expect(one.text).toContain("a.NEF");
  expect(one.text).not.toContain("b.NEF");
});

test("pull of a tree fills leftover .bin after same-name", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({
    pass: plain,
    crypt: [samplePair()],
  }));
  const copies: string[][] = [];
  const spawn: SpawnFn = async (_bin, args) => {
    if (args[2] === "obscure") {
      return { code: 0, stdout: "obscured\n", stderr: "" };
    }
    copies.push(args);
    return { code: 0, stdout: "", stderr: "Transferred:            1 / 1, 100%\n" };
  };
  const out = await runCmd(["pull", "photograph-123"], testDeps(home, spawn));
  expect(out.code).toBe(0);
  expect(copies).toHaveLength(2);
  expect(copies[0]?.join("\n")).toContain("suffix='none'");
  expect(copies[0]).toContain("--exclude");
  expect(copies[0]).toContain("*.bin");
  expect(copies[1]?.join("\n")).toContain("suffix='.bin'");
  expect(copies[1]).toContain("--ignore-existing");
});

test("download -v prints rclone logs and pair retries", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({
    pass: plain,
    crypt: [samplePair()],
  }));
  const chunks: string[] = [];
  const orig = process.stderr.write.bind(process.stderr);
  process.stderr.write = ((chunk: string | Uint8Array) => {
    chunks.push(String(chunk));
    return true;
  }) as typeof process.stderr.write;
  const spawn: SpawnFn = async (_bin, args, sink) => {
    if (args[2] === "obscure") {
      return { code: 0, stdout: "obscured\n", stderr: "" };
    }
    if (args[2] === "lsjson") {
      return {
        code: 0,
        stdout: JSON.stringify([{ Path: "DSC_0746.JPG.bin", Size: 1, IsDir: false }]),
        stderr: "",
      };
    }
    if (args[2] === "copyto") {
      writeCopyDest(args);
      sink.writeErr("NOTICE: copied\n");
    }
    return { code: 0, stdout: "", stderr: "" };
  };
  try {
    const deps = testDeps(home, spawn);
    deps.cwd = home;
    const out = await runCmd(["download", "photograph-123", "album/DSC_0746.JPG.bin", "-v"], deps);
    expect(out.code).toBe(0);
    const text = chunks.join("");
    expect(text).toContain("fetch ");
    expect(text).toContain("decrypt v1");
    expect(text).toContain("NOTICE: copied");
  } finally {
    process.stderr.write = orig;
  }
});

test("complete lists get and task names", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg({ pass: plain }));
  const root = await runCmd(["__complete", ""], testDeps(home, stubSpawn(0).spawn));
  expect(root.text).toContain("get");
  const tasks = await runCmd(["__complete", "get", ""], testDeps(home, stubSpawn(0).spawn));
  expect(tasks.text).toContain("photograph-123");
  const lsTasks = await runCmd(["__complete", "ls", ""], testDeps(home, stubSpawn(0).spawn));
  expect(lsTasks.text).toContain("photograph-123");
  const fish = await runCmd(["completion", "fish"], testDeps(home, stubSpawn(0).spawn));
  expect(fish.text).toContain("push pull check get download ls");
});

test("missing config on push with no task is exit 2", async () => {
  const home = tempHome();
  try {
    await runCmd(["push"], testDeps(home, stubSpawn(0).spawn));
    throw new Error("should have thrown");
  } catch (error) {
    expect(error).toBeInstanceOf(RcloneErr);
    expect((error as RcloneErr).code).toBe(2);
    expect((error as Error).message).toContain("config file not found");
  }
});
