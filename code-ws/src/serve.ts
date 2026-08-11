import { spawn } from "node:child_process";
import { networkInterfaces, platform } from "node:os";
import { basename } from "node:path";
import {
  listFileIndex,
  readBlob,
  readRawFile,
  resolveServeRoot,
  toPosixRel,
} from "./serve-fs";
import { renderServeHtml } from "./serve-ui";
import {
  isUnsupportedWatchError,
  startServeWatch,
  type WatchCtl,
  type WatchEvent,
} from "./serve-watch";

/** 固定默认端口, 避免每次随机分配导致书签/脚本失效. */
export const DEFAULT_SERVE_PORT = 7001;

export type ServeArgs = {
  cmd: "serve";
  path?: string;
  lan: boolean;
  watch: boolean;
  port?: number;
};

export type ServeOpts = {
  root: string;
  host: string;
  port: number;
  lan: boolean;
  watch: boolean;
};

export type WatchStatus = "active" | "disabled" | "unavailable";

/** 运行时读模型: 启动时构建, watch 拓扑变更后更新. */
export type ServeRuntime = {
  root: string;
  files: string[];
  watch: WatchStatus;
};

/**
 * 组装 serve 运行参数, 把 CLI 语义收敛成 HTTP 服务配置.
 */
export function buildServeOpts(
  args: ServeArgs,
  cwd: string = process.cwd(),
): ServeOpts {
  const root = resolveServeRoot(args.path, cwd);
  const host = args.lan ? "0.0.0.0" : "127.0.0.1";
  const port = args.port ?? DEFAULT_SERVE_PORT;
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error("port must be an integer between 1 and 65535");
  }
  return {
    root,
    host,
    port,
    lan: args.lan,
    watch: args.watch,
  };
}

/**
 * 收集可打印的访问 URL.
 * 局域网模式优先列出网卡 IPv4, 默认打开也走首个 LAN 地址.
 */
export function collectAccessUrls(
  host: string,
  port: number,
  lan: boolean,
): string[] {
  if (!lan && host === "127.0.0.1") {
    return [`http://127.0.0.1:${port}`];
  }
  if (!lan && host === "localhost") {
    return [`http://localhost:${port}`];
  }

  // 有序去重: LAN 在前, 本机 loopback 在后, 避免默认 open 落到 127.
  const urls: string[] = [];
  const push = (url: string): void => {
    if (!urls.includes(url)) {
      urls.push(url);
    }
  };

  if (lan || host === "0.0.0.0" || host === "::") {
    for (const ip of listLanIPv4()) {
      push(`http://${ip}:${port}`);
    }
  }
  if (host !== "0.0.0.0" && host !== "::") {
    push(`http://${host}:${port}`);
  }
  push(`http://127.0.0.1:${port}`);
  return urls;
}

function listLanIPv4(): string[] {
  const nets = networkInterfaces();
  const ips: string[] = [];
  for (const entries of Object.values(nets)) {
    if (entries === undefined) {
      continue;
    }
    for (const entry of entries) {
      // Node/Bun 的 family 可能是 "IPv4" 或 4, 统一成字符串比较.
      const family = String(entry.family);
      if (entry.internal || (family !== "IPv4" && family !== "4")) {
        continue;
      }
      ips.push(entry.address);
    }
  }
  return ips;
}

function json(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: {
      "content-type": "application/json; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}

function text(msg: string, status: number): Response {
  return new Response(msg, {
    status,
    headers: {
      "content-type": "text/plain; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}

function wantsHtml(req: Request): boolean {
  const accept = req.headers.get("accept") ?? "";
  return accept.includes("text/html");
}

/**
 * 处理只读演示请求: API / raw / SPA.
 * 索引由 runtime 投影提供, blob/tree 实时读盘, watch 负责投影一致性.
 */
export function handleServeRequest(
  req: Request,
  runtime: ServeRuntime,
): Response {
  const root = runtime.root;
  const url = new URL(req.url);
  if (req.method !== "GET" && req.method !== "HEAD") {
    return text("method not allowed", 405);
  }

  try {
    if (url.pathname === "/api/blob") {
      const rel = url.searchParams.get("path") ?? "";
      return json(readBlob(root, rel));
    }

    if (url.pathname === "/api/index") {
      return json({
        files: runtime.files,
      });
    }

    if (url.pathname === "/api/meta") {
      return json({
        root: basename(root),
        rootPath: root,
        watch: runtime.watch,
      });
    }

    if (url.pathname === "/api/tree") {
      const rel = url.searchParams.get("path") ?? "";
      const blob = readBlob(root, rel);
      if (blob.type !== "dir") {
        return text("not a directory", 400);
      }
      return json({
        path: blob.path,
        entries: blob.entries ?? [],
      });
    }

    if (url.pathname === "/raw" || url.pathname.startsWith("/raw/")) {
      const rel = decodeURIComponent(url.pathname.slice("/raw/".length));
      const raw = readRawFile(root, rel);
      return new Response(raw.buf, {
        headers: {
          "content-type": raw.contentType,
          "cache-control": "no-store",
        },
      });
    }

    // SPA 深链: 浏览器导航一律回 HTML, 内容由前端再拉 API.
    if (wantsHtml(req) || url.pathname === "/" || !url.pathname.startsWith("/api/")) {
      // 若明确请求非 HTML 的文件路径, 仍可走 raw 友好提示; 默认给 UI.
      if (!wantsHtml(req) && url.pathname !== "/" && !url.pathname.startsWith("/api/")) {
        const rel = decodeURIComponent(url.pathname.replace(/^\/+/, ""));
        try {
          const blob = readBlob(root, rel);
          if (blob.type === "file" && blob.binary) {
            const raw = readRawFile(root, rel);
            return new Response(raw.buf, {
              headers: {
                "content-type": raw.contentType,
                "cache-control": "no-store",
              },
            });
          }
          if (blob.type === "file" && blob.content !== undefined) {
            return new Response(blob.content, {
              headers: {
                "content-type": "text/plain; charset=utf-8",
                "cache-control": "no-store",
              },
            });
          }
        } catch {
          // fallthrough to SPA/not found via API semantics
        }
      }

      const html = renderServeHtml(basename(root));
      return new Response(html, {
        headers: {
          "content-type": "text/html; charset=utf-8",
          "cache-control": "no-store",
        },
      });
    }

    return text("not found", 404);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg === "not found") {
      return text("not found", 404);
    }
    if (msg === "path denied" || msg === "path escapes root" || msg === "invalid path") {
      return text(msg, 403);
    }
    return text(msg, 400);
  }
}

/**
 * 先构建文件索引再 bind 端口, 避免首请求冷启动扫盘与重复 walk.
 */
export function buildServeRuntime(root: string): ServeRuntime {
  const files = listFileIndex(root);
  return {
    root,
    files,
    watch: "disabled",
  };
}

/**
 * 启动只读 HTTP 服务并阻塞进程, 因为演示期需要常驻监听.
 */
export function startServe(args: ServeArgs): void {
  const opts = buildServeOpts(args);
  console.log(`code-ws serve`);
  console.log(`  root:  ${opts.root}`);
  console.log(`  index: building...`);
  const indexStarted = Date.now();
  const runtime = buildServeRuntime(opts.root);
  console.log(
    `  index: ${runtime.files.length} files (${Date.now() - indexStarted}ms)`,
  );

  let ctl: WatchCtl | undefined;
  let stopServer: (() => void) | undefined;
  let publish = (
    _event: WatchEvent | { type: "watch-status"; status: WatchStatus },
  ): void => {};
  if (opts.watch) {
    try {
      ctl = startServeWatch({
        root: opts.root,
        onTopology() {
          runtime.files = listFileIndex(opts.root);
        },
        onEvent(event) {
          publish(event);
        },
        onError(err) {
          if (isUnsupportedWatchError(err)) {
            runtime.watch = "unavailable";
            console.warn(`  watch: unavailable; continuing without watch`);
            publish({ type: "watch-status", status: "unavailable" });
            ctl?.close();
            return;
          }
          const msg = err instanceof Error ? err.message : String(err);
          console.error(`code-ws: watch failed: ${msg}`);
          ctl?.close();
          stopServer?.();
          process.exitCode = 1;
        },
      });
      runtime.watch = "active";
    } catch (err) {
      if (!isUnsupportedWatchError(err)) {
        throw err;
      }
      runtime.watch = "unavailable";
      console.warn(`  watch: unavailable; continuing without watch`);
    }
  }

  const server = (() => {
    try {
      return Bun.serve({
        hostname: opts.host,
        port: opts.port,
        fetch(req, srv) {
          const url = new URL(req.url);
          if (url.pathname === "/api/watch") {
            if (runtime.watch !== "active") {
              return text("watch is not active", 503);
            }
            const upgraded = srv.upgrade(req);
            if (upgraded) {
              return undefined;
            }
            return text("websocket upgrade required", 426);
          }
          return handleServeRequest(req, runtime);
        },
        websocket: {
          open(ws) {
            ws.subscribe("serve-watch");
          },
          message() {
            // watch 是单向 invalidation channel, 客户端消息没有业务语义。
          },
          close(ws) {
            ws.unsubscribe("serve-watch");
          },
        },
      });
    } catch (err) {
      // bind 失败时 watcher 必须释放, 否则 CLI 报错后仍会占住进程。
      ctl?.close();
      throw err;
    }
  })();
  stopServer = () => server.stop(true);
  publish = (event): void => {
    server.publish("serve-watch", JSON.stringify(event));
  };

  const port = server.port;
  if (port === undefined) {
    throw new Error("failed to bind serve port");
  }
  const urls = collectAccessUrls(opts.host, port, opts.lan);
  console.log(`  bind:  ${opts.host}:${port}`);
  console.log(`  watch: ${runtime.watch}`);
  for (const url of urls) {
    console.log(`  url:   ${url}`);
  }
  if (opts.lan) {
    // 默认监听局域网无鉴权, 明确提示以免误暴露源码树.
    console.log(`  note:  no auth; deny-list only. stop with Ctrl+C`);
  }

  // 默认打开首个 URL (LAN 优先), 减少演示时手机扫不到 127 的问题.
  const homeUrl = urls[0];
  if (homeUrl === undefined) {
    throw new Error("no access url");
  }
  openBrowser(homeUrl);
}

/**
 * 按平台拉起系统默认浏览器.
 * 只负责 fire-and-forget, 不把浏览器失败反向变成 serve 失败.
 */
function openBrowser(url: string): void {
  const os = platform();
  if (os === "darwin") {
    spawn("open", [url], { stdio: "ignore", detached: true }).unref();
    return;
  }
  if (os === "win32") {
    spawn("cmd", ["/c", "start", "", url], {
      stdio: "ignore",
      detached: true,
    }).unref();
    return;
  }
  spawn("xdg-open", [url], { stdio: "ignore", detached: true }).unref();
}

// re-export for tests that care about path helpers via serve surface
export { toPosixRel };
