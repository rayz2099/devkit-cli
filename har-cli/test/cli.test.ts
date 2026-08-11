import { describe, expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { executeCli } from "../src/cli";

const fixturePath = resolve(import.meta.dir, "fixtures", "sample.har");
const fixtureDir = resolve(import.meta.dir, "fixtures");

describe("executeCli", () => {
  test("默认输出 request 和 response 的 HTTP 明文", async () => {
    const result = await executeCli([
      fixturePath,
      "-f",
      "http.status == 200 and body.status == 1",
    ]);

    expect(result.exitCode).toBe(0);
    expect(result.stderr).toBe("");
    expect(result.stdout).toContain("[Request]");
    expect(result.stdout).toContain("POST /api/order/create?trace=1 HTTP/1.1");
    expect(result.stdout).toContain("[Response]");
    expect(result.stdout).toContain("HTTP/1.1 200 OK");
  });

  test("支持 -f 作为过滤表达式短参数", async () => {
    const result = await executeCli([
      fixturePath,
      "-f",
      "body.status == 1 and http.status == 403",
      "-s",
      "path",
    ]);

    expect(result.exitCode).toBe(0);
    expect(result.stdout).toBe("/api/admin/report\n");
  });

  test("标量字段输出多条结果时不插入空行", async () => {
    const result = await executeCli([
      fixturePath,
      "-s",
      "path",
    ]);

    expect(result.exitCode).toBe(0);
    expect(result.stdout).toBe("/api/user/info\n/api/order/create\n/api/admin/report\n");
  });

  test("未指定 -f 时不应隐式过滤请求", async () => {
    const runtimeDir = mkdtempSync(resolve(process.cwd(), "dist", "no-filter-"));
    const harPath = resolve(runtimeDir, "only-ok.har");
    writeFileSync(harPath, JSON.stringify({
      log: {
        version: "1.2",
        creator: {
          name: "test",
          version: "1.0",
        },
        entries: [
          {
            request: {
              method: "GET",
              url: "https://example.com/api/health",
              httpVersion: "HTTP/1.1",
              headers: [],
            },
            response: {
              status: 200,
              statusText: "OK",
              httpVersion: "HTTP/1.1",
              headers: [],
              content: {
                mimeType: "application/json",
                text: "{\"status\":0}",
              },
            },
          },
        ],
      },
    }));

    const result = await executeCli([
      harPath,
      "-s",
      "path",
    ]);

    expect(result.exitCode).toBe(0);
    expect(result.stdout).toBe("/api/health\n");
  });

  test("prefix 输出多条结果时不插入空行", async () => {
    const result = await executeCli([
      fixturePath,
      "-s",
      "prefix",
      "-p",
      "2",
    ]);

    expect(result.exitCode).toBe(0);
    expect(result.stdout).toBe("/api/user/*\n/api/order/*\n/api/admin/*\n");
  });

  test("支持空的 -s 值并回退到默认 HTTP 明文输出", async () => {
    const result = await executeCli([
      fixturePath,
      "-f",
      "http.status == 200 and body.status == 1",
      "-s",
      "",
    ]);

    expect(result.exitCode).toBe(0);
    expect(result.stdout).toContain("POST /api/order/create?trace=1 HTTP/1.1");
  });

  test("支持自定义保留字段投影", async () => {
    const result = await executeCli([
      fixturePath,
      "-f",
      "body.status == 1 and http.status == 403",
      "-s",
      "path,http.status",
    ]);

    expect(result.exitCode).toBe(0);

    const lines = result.stdout.trim().split("\n");
    expect(lines).toHaveLength(1);
    expect(JSON.parse(lines[0])).toEqual({
      path: "/api/admin/report",
      http: {
        status: 403,
      },
    });
  });

  test("支持 help 子命令", async () => {
    const result = await executeCli(["help"]);

    expect(result.exitCode).toBe(0);
    expect(result.stdout).toContain("Usage:");
    expect(result.stdout).toContain("--filter");
    expect(result.stdout).not.toContain("--where");
    expect(result.stdout).toContain("-s");
  });

  test("支持输出 fish completion", async () => {
    const result = await executeCli(["completions", "fish"]);

    expect(result.exitCode).toBe(0);
    expect(result.stdout).toContain("complete -c har-cli");
    expect(result.stdout).toContain("-l filter");
    expect(result.stdout).toContain("-s f");
  });

  test("fish 补全允许按文件名前缀补全 HAR 文件", async () => {
    const completion = await executeCli(["completions", "fish"]);
    expect(completion.exitCode).toBe(0);

    mkdirSync(resolve(process.cwd(), "dist"), { recursive: true });
    const runtimeDir = mkdtempSync(resolve(process.cwd(), "dist", "fish-completion-"));
    const scriptPath = resolve(runtimeDir, "har-cli.fish");
    writeFileSync(scriptPath, completion.stdout);

    const output = Bun.spawnSync({
      cmd: [
        "fish",
        "-c",
        `source ${JSON.stringify(scriptPath)}; complete --do-complete "har-cli sam"`,
      ],
      cwd: fixtureDir,
      stdout: "pipe",
      stderr: "pipe",
    });

    expect(output.exitCode).toBe(0);
    expect(Buffer.from(output.stdout).toString("utf8")).toContain("sample.har");
  });
});
