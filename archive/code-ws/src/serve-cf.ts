import { spawn, type ChildProcess } from "node:child_process";

/** 二进制只从 PATH 找。Quick Tunnel 没有可写进 config.json 的身份。 */
export const CLOUDFLARED_BIN = "cloudflared";

const TUNNEL_URL_RE = /https:\/\/([a-z0-9-]+)\.trycloudflare\.com/gi;
const ANSI_RE = /\u001b\[[0-9;]*m/g;
/** 申请隧道的 API 主机，失败日志里会带上它，不是这次的公网地址。 */
const PROVISION_HOST = "api";
const LOG_CAP = 16 * 1024;

export type TunnelHooks = {
  onUrl: (url: string) => void;
  onFatal: (msg: string) => void;
};

export type TunnelCtl = {
  close: () => void;
};

/**
 * 回源固定 127.0.0.1：连接器和 serve 在同一台机器。
 * --no-autoupdate：升级会重启进程，这次的随机域名会一起丢掉。
 */
export function quickTunnelArgs(port: number): string[] {
  const origin = `http://127.0.0.1:${port}`;
  return [
    "tunnel",
    "--no-autoupdate",
    "--url",
    origin,
  ];
}

/**
 * 从 cloudflared 日志里取出公网地址。
 * 跳过 api.trycloudflare.com，否则申请失败时会把接口地址当成隧道。
 */
export function extractQuickTunnelUrl(text: string): string | undefined {
  const plain = text.replace(ANSI_RE, "");
  TUNNEL_URL_RE.lastIndex = 0;
  for (const match of plain.matchAll(TUNNEL_URL_RE)) {
    const host = match[1]?.toLowerCase();
    if (host === undefined || host === PROVISION_HOST) {
      continue;
    }
    return match[0];
  }
  return undefined;
}

function appendLog(prev: string, chunk: string): string {
  const next = prev + chunk;
  if (next.length <= LOG_CAP) {
    return next;
  }
  return next.slice(next.length - LOG_CAP);
}

function errno(err: Error): string | undefined {
  if ("code" in err) {
    const code = (err as { code?: unknown }).code;
    if (typeof code === "string") {
      return code;
    }
  }
  return undefined;
}

function killChild(child: ChildProcess): void {
  if (child.exitCode !== null || child.signalCode !== null) {
    return;
  }
  child.kill("SIGTERM");
}

/**
 * 子进程退出即隧道消失。随机地址没有第二个连接器，调用方必须结束 serve。
 */
export function startTunnel(
  bin: string,
  args: string[],
  hooks: TunnelHooks,
): TunnelCtl {
  const child = spawn(bin, args, {
    stdio: ["ignore", "pipe", "pipe"],
  });
  if (child.stdout === null || child.stderr === null) {
    throw new Error("tunnel stdio must be piped");
  }
  let done = false;
  let url: string | undefined;
  let log = "";

  const finish = (fn: () => void): void => {
    if (done) {
      return;
    }
    done = true;
    fn();
  };

  const take = (chunk: string): void => {
    log = appendLog(log, chunk);
    if (url !== undefined) {
      return;
    }
    const found = extractQuickTunnelUrl(log);
    if (found === undefined) {
      return;
    }
    url = found;
    hooks.onUrl(found);
  };

  child.stdout?.setEncoding("utf8");
  child.stderr?.setEncoding("utf8");
  child.stdout?.on("data", take);
  child.stderr?.on("data", take);

  child.on("error", (err) => {
    finish(() => {
      if (errno(err) === "ENOENT") {
        hooks.onFatal(`${bin} not found on PATH`);
        return;
      }
      hooks.onFatal(err.message);
    });
  });

  child.on("exit", (code, signal) => {
    finish(() => {
      const why = signal ?? String(code);
      const tail = log.trim();
      if (url === undefined) {
        hooks.onFatal(`${bin} exited (${why}) before a tunnel url\n${tail}`);
        return;
      }
      hooks.onFatal(`${bin} exited (${why})\n${tail}`);
    });
  });

  return {
    close() {
      finish(() => {
        killChild(child);
      });
    },
  };
}

/**
 * Ctrl+C / SIGTERM 先关掉 cloudflared 再退出，避免随机隧道在 serve 死后还挂着。
 */
function armStopSignals(inner: TunnelCtl): TunnelCtl {
  let stopped = false;
  const stop = (code: number): void => {
    if (stopped) {
      return;
    }
    stopped = true;
    process.off("SIGINT", onInt);
    process.off("SIGTERM", onTerm);
    inner.close();
    process.exit(code);
  };
  const onInt = (): void => {
    stop(130);
  };
  const onTerm = (): void => {
    stop(143);
  };
  process.once("SIGINT", onInt);
  process.once("SIGTERM", onTerm);
  return {
    close() {
      if (stopped) {
        return;
      }
      stopped = true;
      process.off("SIGINT", onInt);
      process.off("SIGTERM", onTerm);
      inner.close();
    },
  };
}

/** 为已绑定的 serve 端口拉起 Quick Tunnel。 */
export function startQuickTunnel(port: number, hooks: TunnelHooks): TunnelCtl {
  const args = quickTunnelArgs(port);
  const inner = startTunnel(CLOUDFLARED_BIN, args, hooks);
  return armStopSignals(inner);
}
