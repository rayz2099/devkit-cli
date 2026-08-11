import { describe, expect, test } from "bun:test";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import {
  buildServeOpts,
  buildServeRuntime,
  collectAccessUrls,
  handleServeRequest,
} from "./serve";
import { resolveServeRoot } from "./serve-fs";
import { renderServeClientScript } from "./serve-ui-client";

describe("handleServeRequest", () => {
  test("API 返回文件与 SPA", async () => {
    const dir = join(process.cwd(), "code-ws", ".tmp-serve-http");
    mkdirSync(join(dir, "docs"), { recursive: true });
    writeFileSync(join(dir, "docs", "a.md"), "# demo\n\n```mermaid\ngraph TD; A-->B;\n```\n");
    writeFileSync(join(dir, "README.md"), "# root readme\n");
    writeFileSync(join(dir, "src.ts"), "export const x = 1;\n");
    const root = resolveServeRoot(dir, process.cwd());
    const runtime = buildServeRuntime(root);

    const blobRes = await handleServeRequest(
      new Request("http://127.0.0.1/api/blob?path=docs/a.md"),
      runtime,
    );
    expect(blobRes.status).toBe(200);
    const blob = await blobRes.json() as { language: string; content: string };
    expect(blob.language).toBe("markdown");
    expect(blob.content).toContain("mermaid");

    const denied = await handleServeRequest(
      new Request("http://127.0.0.1/api/blob?path=.env"),
      runtime,
    );
    expect(denied.status).toBe(403);

    const rootBlob = await handleServeRequest(
      new Request("http://127.0.0.1/api/blob?path="),
      runtime,
    );
    expect(rootBlob.status).toBe(200);
    const rootJson = await rootBlob.json() as { type: string; readme?: string };
    expect(rootJson.type).toBe("dir");
    expect(rootJson.readme).toBe("README.md");

    const indexRes = await handleServeRequest(
      new Request("http://127.0.0.1/api/index"),
      runtime,
    );
    const indexJson = await indexRes.json() as { files: string[] };
    expect(indexJson.files).toContain("README.md");
    expect(indexJson.files).toContain("docs/a.md");

    const metaRes = await handleServeRequest(
      new Request("http://127.0.0.1/api/meta"),
      runtime,
    );
    const meta = await metaRes.json() as { watch: string };
    expect(meta.watch).toBe("disabled");

    const html = await handleServeRequest(
      new Request("http://127.0.0.1/docs/a.md", {
        headers: { accept: "text/html" },
      }),
      runtime,
    );
    expect(html.status).toBe(200);
    expect(html.headers.get("content-type")).toContain("text/html");
    const body = await html.text();
    expect(body).toContain("Go to file");
    expect(body).toContain("mermaid");
  });
});

describe("buildServeOpts", () => {
  test("默认 lan 绑定 0.0.0.0", () => {
    const opts = buildServeOpts({
      cmd: "serve",
      lan: true,
      watch: true,
    }, process.cwd());
    expect(opts.host).toBe("0.0.0.0");
    expect(opts.port).toBe(7001);
    expect(opts.watch).toBe(true);
  });

  test("local 模式绑定 loopback", () => {
    const opts = buildServeOpts({
      cmd: "serve",
      lan: false,
      watch: false,
    }, process.cwd());
    expect(opts.host).toBe("127.0.0.1");
    expect(opts.watch).toBe(false);
  });
});

describe("collectAccessUrls", () => {
  test("lan 时局域网 URL 排在 127 之前", () => {
    const urls = collectAccessUrls("0.0.0.0", 7001, true);
    expect(urls.at(-1)).toBe("http://127.0.0.1:7001");
    if (urls.length > 1) {
      expect(urls[0]).not.toBe("http://127.0.0.1:7001");
    }
  });

  test("local 仅返回 loopback", () => {
    expect(collectAccessUrls("127.0.0.1", 7001, false)).toEqual([
      "http://127.0.0.1:7001",
    ]);
  });
});

describe("serve UI client", () => {
  test("生成的浏览器脚本语法有效", () => {
    const script = renderServeClientScript(JSON.stringify("demo"));
    expect(() => new Function(script)).not.toThrow();
  });
});
