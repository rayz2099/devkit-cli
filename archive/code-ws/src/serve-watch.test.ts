import { describe, expect, test } from "bun:test";
import {
  existsSync,
  mkdirSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { join } from "node:path";
import {
  isUnsupportedWatchError,
  reconcileWatchPaths,
  scanWatchPaths,
  startServeWatch,
  type WatchEvent,
} from "./serve-watch";

describe("serve watch snapshot", () => {
  test("忽略依赖、构建产物和 deny-list", () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-watch-scan");
    mkdirSync(join(dir, "src"), { recursive: true });
    mkdirSync(join(dir, "node_modules", "pkg"), { recursive: true });
    mkdirSync(join(dir, "target"), { recursive: true });
    writeFileSync(join(dir, "src", "main.ts"), "export {};\n");
    writeFileSync(join(dir, "node_modules", "pkg", "index.js"), "module.exports = {};\n");
    writeFileSync(join(dir, "target", "app.jar"), "binary");
    writeFileSync(join(dir, ".env"), "TOKEN=x\n");

    const paths = scanWatchPaths(dir);
    expect(paths.has("src")).toBe(true);
    expect(paths.has("src/main.ts")).toBe(true);
    expect([...paths].some((path) => path.startsWith("node_modules"))).toBe(false);
    expect([...paths].some((path) => path.startsWith("target"))).toBe(false);
    expect(paths.has(".env")).toBe(false);
  });

  test("将原子替换收敛为 change", () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-watch-change");
    mkdirSync(dir, { recursive: true });
    const file = join(dir, "README.md");
    writeFileSync(file, "before\n");
    const known = scanWatchPaths(dir);
    writeFileSync(file, "after\n");

    const event = reconcileWatchPaths({
      root: dir,
      known,
      paths: new Set(["README.md"]),
    });
    expect(event).toEqual({
      type: "files-changed",
      changes: [{ path: "README.md", kind: "change" }],
    });
  });

  test("识别 create/delete 并刷新快照", () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-watch-topology");
    mkdirSync(dir, { recursive: true });
    const oldFile = join(dir, "old.md");
    const newFile = join(dir, "new.md");
    if (existsSync(newFile)) {
      unlinkSync(newFile);
    }
    writeFileSync(oldFile, "old\n");
    const known = scanWatchPaths(dir);
    unlinkSync(oldFile);
    writeFileSync(newFile, "new\n");

    const event = reconcileWatchPaths({
      root: dir,
      known,
      paths: new Set(["old.md", "new.md"]),
    });
    expect(event).toEqual({
      type: "files-changed",
      changes: [
        { path: "new.md", kind: "create" },
        { path: "old.md", kind: "delete" },
      ],
    });
    expect(known.has("new.md")).toBe(true);
    expect(known.has("old.md")).toBe(false);
  });

  test("超过阈值时收敛为 resync", () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-watch-resync");
    mkdirSync(dir, { recursive: true });
    const event = reconcileWatchPaths({
      root: dir,
      known: new Set(),
      paths: new Set(["a", "b"]),
      maxChanges: 1,
    });
    expect(event).toEqual({ type: "resync" });
  });

  test("递归 watcher 推送实际文件变更", async () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-watch-live");
    mkdirSync(join(dir, "docs"), { recursive: true });
    const file = join(dir, "docs", "live.md");
    writeFileSync(file, "before\n");

    let resolveEvent: (event: WatchEvent) => void = () => {};
    let rejectEvent: (err: unknown) => void = () => {};
    const eventP = new Promise<WatchEvent>((resolve, reject) => {
      resolveEvent = resolve;
      rejectEvent = reject;
    });
    const ctl = startServeWatch({
      root: dir,
      onEvent: resolveEvent,
      onTopology() {},
      onError: rejectEvent,
      debounceMs: 20,
    });

    try {
      writeFileSync(file, "after\n");
      const event = await Promise.race([
        eventP,
        new Promise<never>((_resolve, reject) => {
          setTimeout(() => reject(new Error("watch event timeout")), 2000);
        }),
      ]);
      expect(event).toEqual({
        type: "files-changed",
        changes: [{ path: "docs/live.md", kind: "change" }],
      });
    } finally {
      ctl.close();
    }
  });
});

describe("serve watch fallback", () => {
  test("只对能力缺失降级", () => {
    expect(isUnsupportedWatchError({ code: "ENOTSUP" })).toBe(true);
    expect(isUnsupportedWatchError({ code: "ERR_FEATURE_UNAVAILABLE_ON_PLATFORM" })).toBe(true);
    expect(isUnsupportedWatchError({ code: "EACCES" })).toBe(false);
    expect(isUnsupportedWatchError(new Error("boom"))).toBe(false);
  });
});
