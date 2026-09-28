import { describe, expect, test } from "bun:test";
import {
  extractQuickTunnelUrl,
  quickTunnelArgs,
  startTunnel,
} from "./serve-cf";

describe("quick tunnel args", () => {
  test("回源到本机已绑定端口", () => {
    expect(quickTunnelArgs(7001)).toEqual([
      "tunnel",
      "--no-autoupdate",
      "--url",
      "http://127.0.0.1:7001",
    ]);
  });
});

describe("extractQuickTunnelUrl", () => {
  test("从日志和 ANSI 里取出第一条地址", () => {
    const log = [
      "INF Requesting new quick Tunnel",
      "INF |  \u001b[1mhttps://demo-ray.trycloudflare.com\u001b[0m  |",
      "INF https://later-one.trycloudflare.com",
    ].join("\n");
    expect(extractQuickTunnelUrl(log)).toBe("https://demo-ray.trycloudflare.com");
  });

  test("地址被拆到两次输出时仍能拼上", () => {
    const head = "visit https://split-name.";
    const tail = "trycloudflare.com now";
    expect(extractQuickTunnelUrl(head + tail)).toBe(
      "https://split-name.trycloudflare.com",
    );
  });

  test("申请失败日志里的 API 域名不是公网地址", () => {
    const fail = [
      'ERR failed to request quick Tunnel error="Post \\"https://api.trycloudflare.com/tunnel\\": EOF"',
    ].join("\n");
    expect(extractQuickTunnelUrl(fail)).toBeUndefined();

    const later = [
      fail,
      "INF |  https://aged-makers-consciousness-mississippi.trycloudflare.com |",
    ].join("\n");
    expect(extractQuickTunnelUrl(later)).toBe(
      "https://aged-makers-consciousness-mississippi.trycloudflare.com",
    );
  });
});

describe("startTunnel", () => {
  test("子进程打出地址后保持运行，close 不当成失败", async () => {
    let url = "";
    let fatal = "";
    const script = [
      "console.error('https://demo-ray.trycloudflare.com')",
      "setInterval(() => {}, 1000)",
    ].join(";");
    const ctl = startTunnel(process.execPath, [
      "-e",
      script,
    ], {
      onUrl(found) {
        url = found;
      },
      onFatal(msg) {
        fatal = msg;
      },
    });
    await waitFor(() => url.length > 0);
    ctl.close();
    await Bun.sleep(50);
    expect(url).toBe("https://demo-ray.trycloudflare.com");
    expect(fatal).toBe("");
  });

  test("还没打出地址就退出则失败", async () => {
    let fatal = "";
    startTunnel(process.execPath, [
      "-e",
      "process.exit(2)",
    ], {
      onUrl() {
        throw new Error("unexpected url");
      },
      onFatal(msg) {
        fatal = msg;
      },
    });
    await waitFor(() => fatal.length > 0);
    expect(fatal).toContain("exited");
    expect(fatal).toContain("before a tunnel url");
  });

  test("地址出现后进程退出仍然失败", async () => {
    let fatal = "";
    const script = [
      "console.error('https://demo-ray.trycloudflare.com')",
      "process.exit(0)",
    ].join(";");
    startTunnel(process.execPath, [
      "-e",
      script,
    ], {
      onUrl() {},
      onFatal(msg) {
        fatal = msg;
      },
    });
    await waitFor(() => fatal.length > 0);
    expect(fatal).toContain("exited (0)");
    expect(fatal).not.toContain("before a tunnel url");
  });

  test("申请接口报错退出时带上 cloudflared 日志", async () => {
    let url = "";
    let fatal = "";
    const script = [
      "console.error('failed to request quick Tunnel: Post \"https://api.trycloudflare.com/tunnel\": EOF')",
      "process.exit(1)",
    ].join(";");
    startTunnel(process.execPath, [
      "-e",
      script,
    ], {
      onUrl(found) {
        url = found;
      },
      onFatal(msg) {
        fatal = msg;
      },
    });
    await waitFor(() => fatal.length > 0);
    expect(url).toBe("");
    expect(fatal).toContain("before a tunnel url");
    expect(fatal).toContain("https://api.trycloudflare.com/tunnel");
  });

  test("PATH 上没有二进制则失败", async () => {
    let fatal = "";
    startTunnel("code-ws-cloudflared-missing", [
      "tunnel",
    ], {
      onUrl() {
        throw new Error("unexpected url");
      },
      onFatal(msg) {
        fatal = msg;
      },
    });
    await waitFor(() => fatal.length > 0);
    expect(fatal).toBe("code-ws-cloudflared-missing not found on PATH");
  });
});

function waitFor(ready: () => boolean): Promise<void> {
  return new Promise((resolve, reject) => {
    const started = Date.now();
    const timer = setInterval(() => {
      if (ready()) {
        clearInterval(timer);
        resolve();
        return;
      }
      if (Date.now() - started > 2000) {
        clearInterval(timer);
        reject(new Error("timed out"));
      }
    }, 10);
  });
}
