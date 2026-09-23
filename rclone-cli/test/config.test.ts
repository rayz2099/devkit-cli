import { expect, test } from "bun:test";
import { join } from "node:path";
import { loadFileCfg, parseFileCfg } from "../src/config";
import { RcloneErr } from "../src/types";
import { parseCfg, sampleCfg, samplePair, tempHome, TEST_HOME, writeCfg } from "./fixture";

const examplePath = join(import.meta.dir, "../config.example.json");

test("example config loads", async () => {
  const text = await Bun.file(examplePath).text();
  const cfg = parseFileCfg(text, examplePath, "/opt/home");
  expect(cfg.rcloneBin).toBe("rclone");
  expect(cfg.sources[0]?.name).toBe("photograph");
  expect(cfg.sources[0]?.path).toBe("/opt/home/workspace/photograph");
  expect(cfg.targets[0]?.root).toBe("photograph");
  expect(cfg.targets[0]?.skipLinks).toBe(true);
  expect(cfg.targets[0]?.skipHidden).toBe(true);
  expect(cfg.targets[0]?.sizeOnly).toBe(true);
  expect(cfg.targets[0]?.pacerMinSleep).toBe("200ms");
  expect(cfg.targets[0]?.encryptFilenames).toBe(false);
  expect(cfg.targets[0]?.crypt).toBeUndefined();
  expect(cfg.tasks[0]?.name).toBe("photograph-123");
  expect(cfg.tasks.map((task) => task.name)).toEqual(["photograph-123"]);
});

test("source path expands $HOME and ${HOME}", () => {
  const body = sampleCfg();
  const sources = body.sources as Record<string, unknown>[];
  const source = sources[0];
  if (source === undefined) {
    throw new Error("missing source");
  }
  source.path = "$HOME/workspace/photograph";
  expect(parseCfg(body, "c.json").sources[0]?.path).toBe(`${TEST_HOME}/workspace/photograph`);
  source.path = "${HOME}/workspace/photograph";
  expect(parseCfg(body, "c.json").sources[0]?.path).toBe(`${TEST_HOME}/workspace/photograph`);
});

test("source path does not expand other $ vars", () => {
  const body = sampleCfg();
  const sources = body.sources as Record<string, unknown>[];
  const source = sources[0];
  if (source === undefined) {
    throw new Error("missing source");
  }
  source.path = "$HOMEWORK/photograph";
  expect(() => parseCfg(body, "c.json")).toThrow("sources[0].path must be absolute");
});

test("missing field names the path", () => {
  expect(() => parseCfg(`{ "sources": [], "targets": [{}], "tasks": [] }`, "c.json"))
    .toThrow("c.json targets[0].name is required");
});

test("crypt object is not an array", () => {
  const body = sampleCfg({ crypt: { key: "one", salt: "two" } });
  expect(() => parseCfg(body, "c.json"))
    .toThrow("c.json targets[0].crypt must be an array of key pairs");
});

test("crypt pair missing salt does not degrade", () => {
  const pair = samplePair();
  delete pair.salt;
  const body = sampleCfg({ crypt: [pair] });
  expect(() => parseCfg(body, "c.json"))
    .toThrow("targets[0].crypt[0].salt is required");
});

test("old crypt password fields are rejected", () => {
  const body = sampleCfg({
    crypt: [{ name: "v1", algorithm: "secretbox", password: "one", password2: "two", default: true }],
  });
  expect(() => parseCfg(body, "c.json"))
    .toThrow("use key and salt, not password/password2");
});

test("empty crypt array is not plaintext", () => {
  const body = sampleCfg({ crypt: [] });
  expect(() => parseCfg(body, "c.json"))
    .toThrow("crypt must have at least one key pair");
});

test("crypt rejects two default pairs", () => {
  const body = sampleCfg({
    crypt: [samplePair({ name: "a" }), samplePair({ name: "b", default: true })],
  });
  expect(() => parseCfg(body, "c.json"))
    .toThrow("requires exactly one default key pair");
});

test("omitted crypt default uses the first pair", () => {
  const first = samplePair({ name: "v1" });
  delete first.default;
  const second = samplePair({ name: "legacy", key: "old", salt: "s" });
  delete second.default;
  delete second.algorithm;
  const cfg = parseCfg(sampleCfg({ crypt: [first, second] }), "c.json");
  expect(cfg.targets[0]?.crypt?.[0]?.default).toBe(true);
  expect(cfg.targets[0]?.crypt?.[1]?.default).toBe(false);
  expect(cfg.targets[0]?.crypt?.[1]?.algorithm).toBe("secretbox");
});

test("crypt algorithm must be secretbox", () => {
  const body = sampleCfg({ crypt: [samplePair({ algorithm: "aes" })] });
  expect(() => parseCfg(body, "c.json"))
    .toThrow("algorithm must be secretbox");
});

test("two crypt pairs keep the default", () => {
  const body = sampleCfg({
    crypt: [samplePair({ name: "v1" }), samplePair({ name: "legacy", key: "old", salt: "s", default: false })],
  });
  const cfg = parseCfg(body, "c.json");
  expect(cfg.targets[0]?.crypt?.map((item) => item.name)).toEqual(["v1", "legacy"]);
  expect(cfg.targets[0]?.crypt?.find((item) => item.default)?.algorithm).toBe("secretbox");
});

test("encryptFilenames true requires crypt", () => {
  expect(() => parseCfg(sampleCfg({ encryptFilenames: true }), "c.json"))
    .toThrow("encryptFilenames requires crypt");
  const body = sampleCfg({
    crypt: [samplePair()],
    encryptFilenames: true,
    task: { encryptFilenames: true },
  });
  expect(parseCfg(body, "c.json").tasks[0]?.encryptFilenames).toBe(true);
});

test("task encryptFilenames true without crypt fails", () => {
  const body = sampleCfg({ task: { encryptFilenames: true } });
  expect(() => parseCfg(body, "c.json"))
    .toThrow("encryptFilenames requires crypt on the target");
});

test("task cannot override the remote", () => {
  const body = sampleCfg();
  const tasks = body.tasks as Record<string, unknown>[];
  const task = tasks[0];
  if (task === undefined) {
    throw new Error("missing task");
  }
  task.url = "https://evil.example";
  expect(() => parseCfg(body, "c.json")).toThrow("url is not allowed");
});

test("defaultTask is rejected", () => {
  const body = sampleCfg();
  body.defaultTask = "photograph-123";
  expect(() => parseCfg(body, "c.json")).toThrow("defaultTask is not allowed");
});

test("unknown source name fails", () => {
  const body = sampleCfg({ task: { source: "missing" } });
  expect(() => parseCfg(body, "c.json"))
    .toThrow("tasks[0].source not found: missing");
});

test("relative source path fails", () => {
  const text = `{
    "sources": [{ "name": "photograph", "path": "photograph" }],
    "targets": [],
    "tasks": []
  }`;
  expect(() => parseCfg(text, "c.json")).toThrow("sources[0].path must be absolute");
});

test("comments and trailing commas parse", () => {
  const cfg = parseCfg(`{
    // keep
    "sources": [{ "name": "photograph", "path": "/tmp/photo", }],
    "targets": [{
      "name": "123",
      "url": "https://webdav.123pan.cn/webdav",
      "vendor": "other",
      "user": "",
      "pass": "",
      "root": "photograph",
      "skipLinks": true,
      "skipHidden": true,
      "sizeOnly": true,
      "transfers": 1,
      "checkers": 4,
      "retries": 10,
      "retriesSleep": "15s",
      "lowLevelRetries": 20,
      "timeout": "1h",
      "contimeout": "60s",
      "tpslimit": 2,
      "pacerMinSleep": "200ms",
      "encryptFilenames": false,
    }],
    "tasks": [{ "name": "photograph-123", "source": "photograph", "target": "123", }],
  }`, "c.json");
  expect(cfg.targets[0]?.skipLinks).toBe(true);
  expect(cfg.targets[0]?.skipHidden).toBe(true);
  expect(cfg.targets[0]?.sizeOnly).toBe(true);
  expect(cfg.targets[0]?.pacerMinSleep).toBe("200ms");
});

test("omitted target knobs use defaults", () => {
  const body = sampleCfg();
  const target = (body.targets as Record<string, unknown>[])[0];
  if (target === undefined) {
    throw new Error("missing target");
  }
  delete target.skipLinks;
  delete target.skipHidden;
  delete target.sizeOnly;
  delete target.pacerMinSleep;
  delete target.encryptFilenames;
  delete target.vendor;
  const cfg = parseCfg(body, "c.json");
  expect(cfg.targets[0]?.skipLinks).toBe(true);
  expect(cfg.targets[0]?.skipHidden).toBe(true);
  expect(cfg.targets[0]?.sizeOnly).toBe(true);
  expect(cfg.targets[0]?.pacerMinSleep).toBe("200ms");
  expect(cfg.targets[0]?.encryptFilenames).toBe(false);
  expect(cfg.targets[0]?.vendor).toBe("other");
});

test("secrets require mode 600", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg(), 0o644);
  const path = join(home, ".config", "rclone-cli", "config.json");
  await expect(loadFileCfg(path, home)).rejects.toThrow("config file mode must be 600");
});

test("mode 600 with secrets loads", async () => {
  const home = tempHome();
  writeCfg(home, sampleCfg(), 0o600);
  const path = join(home, ".config", "rclone-cli", "config.json");
  const cfg = await loadFileCfg(path, home);
  expect(cfg.tasks[0]?.name).toBe("photograph-123");
});

test("missing config file throws RcloneErr", async () => {
  const home = tempHome();
  const path = join(home, ".config", "rclone-cli", "config.json");
  try {
    await loadFileCfg(path, home);
    throw new Error("should have thrown");
  } catch (error) {
    expect(error).toBeInstanceOf(RcloneErr);
    expect((error as RcloneErr).code).toBe(1);
    expect((error as Error).message).toContain("config file not found");
  }
});
