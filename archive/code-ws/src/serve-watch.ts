import {
  lstatSync,
  readdirSync,
  watch,
  type FSWatcher,
} from "node:fs";
import {
  isAbsolute,
  join,
  relative,
  resolve,
  sep,
} from "node:path";
import {
  isWatchIgnoredRelPath,
  resolveUnderRoot,
  toPosixRel,
} from "./serve-fs";

export type WatchKind = "create" | "change" | "delete";

export type WatchChange = {
  path: string;
  kind: WatchKind;
};

export type WatchEvent =
  | {
    type: "files-changed";
    changes: WatchChange[];
  }
  | {
    type: "resync";
  };

export type WatchCtl = {
  close(): void;
};

type ReconcileOpts = {
  root: string;
  known: Set<string>;
  paths: Set<string>;
  forceResync?: boolean;
  maxChanges?: number;
};

type StartWatchOpts = {
  root: string;
  onEvent: (event: WatchEvent) => void;
  onTopology: () => void;
  onError: (err: unknown) => void;
  debounceMs?: number;
  maxChanges?: number;
};

const WATCH_MAX_DEPTH = 64;

/**
 * 建立 watch 自己的路径快照, 因为 fs.watch 的 rename 事件不区分创建与删除。
 */
export function scanWatchPaths(root: string): Set<string> {
  const paths = new Set<string>();
  walkPaths({
    root,
    dir: root,
    paths,
    depth: 0,
  });
  return paths;
}

function walkPaths(opts: {
  root: string;
  dir: string;
  paths: Set<string>;
  depth: number;
}): void {
  if (opts.depth > WATCH_MAX_DEPTH) {
    return;
  }

  let names: string[];
  try {
    names = readdirSync(opts.dir);
  } catch {
    return;
  }

  for (const name of names) {
    const abs = join(opts.dir, name);
    const rel = toPosixRel(opts.root, abs);
    if (isWatchIgnoredRelPath(rel)) {
      continue;
    }

    let st;
    try {
      st = lstatSync(abs);
    } catch {
      continue;
    }
    if (st.isSymbolicLink()) {
      continue;
    }
    if (!st.isDirectory() && !st.isFile()) {
      continue;
    }

    opts.paths.add(rel);
    if (st.isDirectory()) {
      walkPaths({
        root: opts.root,
        dir: abs,
        paths: opts.paths,
        depth: opts.depth + 1,
      });
    }
  }
}

/**
 * 批次结束后再对照文件系统, 因为原子保存的 delete/create 应收敛成 change。
 */
export function reconcileWatchPaths(opts: ReconcileOpts): WatchEvent | undefined {
  const max = opts.maxChanges ?? 1000;
  if (opts.forceResync || opts.paths.size > max) {
    replaceKnown(opts.known, scanWatchPaths(opts.root));
    return { type: "resync" };
  }

  const changes: WatchChange[] = [];
  let topology = false;
  const paths = [...opts.paths].sort((a, b) => a.localeCompare(b));
  for (const path of paths) {
    const had = opts.known.has(path);
    const exists = isWatchablePath(opts.root, path);
    if (!had && !exists) {
      continue;
    }
    const kind: WatchKind = had
      ? (exists ? "change" : "delete")
      : "create";
    topology ||= kind !== "change";
    changes.push({ path, kind });
  }

  if (changes.length === 0) {
    return undefined;
  }
  if (topology) {
    replaceKnown(opts.known, scanWatchPaths(opts.root));
  }
  return {
    type: "files-changed",
    changes,
  };
}

function isWatchablePath(root: string, path: string): boolean {
  try {
    const st = lstatSync(resolveUnderRoot(root, path));
    return !st.isSymbolicLink() && (st.isDirectory() || st.isFile());
  } catch {
    return false;
  }
}

function replaceKnown(target: Set<string>, source: Set<string>): void {
  target.clear();
  for (const path of source) {
    target.add(path);
  }
}

function normalizeEventPath(root: string, filename: string | Buffer): string | undefined {
  const raw = String(filename);
  const abs = isAbsolute(raw) ? resolve(raw) : resolve(root, raw);
  const rel = relative(resolve(root), abs);
  if (rel === "" || rel === ".." || rel.startsWith(`..${sep}`)) {
    return undefined;
  }
  const path = rel.split(sep).join("/");
  if (isWatchIgnoredRelPath(path)) {
    return undefined;
  }
  return path;
}

/**
 * 只把运行时明确声明的能力缺失视为可降级, 避免吞掉权限和资源耗尽错误。
 */
export function isUnsupportedWatchError(err: unknown): boolean {
  if (typeof err !== "object" || err === null || !("code" in err)) {
    return false;
  }
  const code = String((err as { code?: unknown }).code ?? "");
  return code === "ERR_FEATURE_UNAVAILABLE_ON_PLATFORM"
    || code === "ERR_METHOD_NOT_IMPLEMENTED"
    || code === "ENOSYS"
    || code === "ENOTSUP";
}

/**
 * watcher 只输出归一化领域事件, HTTP/WebSocket 层不感知 fs.watch 的平台细节。
 */
export function startServeWatch(opts: StartWatchOpts): WatchCtl {
  const known = scanWatchPaths(opts.root);
  const paths = new Set<string>();
  const debounceMs = opts.debounceMs ?? 100;
  const maxChanges = opts.maxChanges ?? 1000;
  let forceResync = false;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const flush = (): void => {
    timer = undefined;
    const event = reconcileWatchPaths({
      root: opts.root,
      known,
      paths,
      forceResync,
      maxChanges,
    });
    paths.clear();
    forceResync = false;
    if (event === undefined) {
      return;
    }
    if (event.type === "resync" || event.changes.some((item) => item.kind !== "change")) {
      opts.onTopology();
    }
    opts.onEvent(event);
  };

  const schedule = (): void => {
    if (timer !== undefined) {
      clearTimeout(timer);
    }
    timer = setTimeout(flush, debounceMs);
  };

  let watcher: FSWatcher;
  watcher = watch(opts.root, { recursive: true }, (_event, filename) => {
    if (filename === null) {
      forceResync = true;
      schedule();
      return;
    }
    const path = normalizeEventPath(opts.root, filename);
    if (path === undefined) {
      return;
    }
    paths.add(path);
    schedule();
  });
  watcher.on("error", opts.onError);

  return {
    close(): void {
      if (timer !== undefined) {
        clearTimeout(timer);
      }
      watcher.close();
    },
  };
}
