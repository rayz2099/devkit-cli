import { chmodSync, mkdtempSync, mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { parseFileCfg } from "../src/config";
import type { Deps } from "../src/run";
import type { FileCfg, SpawnFn } from "../src/types";

export const TEST_HOME = "/tmp/rclone-home";
export const SAMPLE_SRC = "/tmp/photograph";

export function samplePair(over: SamplePair = {}): Record<string, unknown> {
  return {
    name: over.name ?? "v1",
    algorithm: over.algorithm ?? "secretbox",
    key: over.key ?? "one",
    salt: over.salt ?? "two",
    default: over.default ?? true,
  };
}

export function parseCfg(body: Record<string, unknown> | string, path = "mem.json"): FileCfg {
  const text = typeof body === "string" ? body : JSON.stringify(body);
  return parseFileCfg(text, path, TEST_HOME);
}

export function tempHome(): string {
  return mkdtempSync(join(tmpdir(), "rclone-cli-"));
}

export function sampleCfg(over: SampleOver = {}): Record<string, unknown> {
  const target: Record<string, unknown> = {
    name: "123",
    url: "https://webdav.123pan.cn/webdav",
    vendor: "other",
    user: over.user ?? "user",
    pass: over.pass ?? "p@ss,w:rd='x'\"y",
    root: "photograph",
    skipLinks: over.skipLinks ?? true,
    skipHidden: over.skipHidden ?? true,
    sizeOnly: over.sizeOnly ?? true,
    transfers: 1,
    checkers: 4,
    retries: 10,
    retriesSleep: "15s",
    lowLevelRetries: 20,
    timeout: "1h",
    contimeout: "60s",
    tpslimit: 2,
    pacerMinSleep: "200ms",
    encryptFilenames: over.encryptFilenames ?? false,
  };
  if (over.crypt !== undefined) {
    target.crypt = over.crypt;
  }
  const task: Record<string, unknown> = {
    name: "photograph-123",
    source: "photograph",
    target: "123",
  };
  if (over.task !== undefined) {
    Object.assign(task, over.task);
  }
  return {
    rcloneBin: "rclone",
    sources: [{ name: "photograph", path: SAMPLE_SRC }],
    targets: [target],
    tasks: [task],
  };
}

export function writeCfg(home: string, body: unknown, mode = 0o600): void {
  const dir = join(home, ".config", "rclone-cli");
  mkdirSync(dir, { recursive: true });
  const path = join(dir, "config.json");
  const text = typeof body === "string" ? body : JSON.stringify(body);
  writeFileSync(path, text);
  chmodSync(path, mode);
}

export function testDeps(home: string, spawn: SpawnFn): Deps {
  return {
    home,
    spawn,
    now: () => new Date("2026-09-22T00:00:00.000Z"),
    id: () => "run-1",
    cwd: "/tmp",
  };
}

/** 为什么: obscure 和 copy 走同一个 spawn. 桩把口令换成不含原文字的 token, 才能断言 argv 里没有明文. */
export function stubSpawn(code: number, stderr = ""): { spawn: SpawnFn; copies: string[][] } {
  const copies: string[][] = [];
  const spawn: SpawnFn = async (_bin, args) => {
    if (args[2] === "obscure") {
      const plain = args[args.length - 1] ?? "";
      const obscured = Buffer.from(`obscured:${plain}`).toString("base64url");
      return { code: 0, stdout: `${obscured}\n`, stderr: "" };
    }
    copies.push(args);
    return { code, stdout: "", stderr };
  };
  return { spawn, copies };
}

type SamplePair = {
  name?: string;
  algorithm?: string;
  key?: string;
  salt?: string;
  default?: boolean;
};

type SampleOver = {
  user?: string;
  pass?: string;
  skipLinks?: boolean;
  skipHidden?: boolean;
  sizeOnly?: boolean;
  encryptFilenames?: boolean;
  crypt?: unknown;
  task?: Record<string, unknown>;
};
