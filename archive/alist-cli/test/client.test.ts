import { afterEach, expect, test } from "bun:test";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { AlistClient } from "../src/client";
import { AlistErr } from "../src/types";
import type { Runtime } from "../src/types";

const runtime: Runtime = {
  audience: "agent",
  profile: {
    name: "home",
    url: "http://alist.test",
    username: "admin",
    password: "secret",
  },
};

const dirs: string[] = [];

afterEach(() => {
  for (const dir of dirs) {
    rmSync(dir, { recursive: true, force: true });
  }
  dirs.length = 0;
});

function tmpFile(content: string): string {
  const dir = mkdtempSync(join(tmpdir(), "alist-put-"));
  dirs.push(dir);
  const file = join(dir, "seq");
  writeFileSync(file, content);
  return file;
}

function ok(data: unknown): Response {
  return new Response(JSON.stringify({ code: 200, message: "success", data }), { status: 200 });
}

test("先 login 再带 Authorization", async () => {
  const paths: string[] = [];
  const auths: string[] = [];
  const client = new AlistClient(runtime, async (input, init) => {
    const url = new URL(String(input));
    paths.push(url.pathname);
    auths.push(new Headers(init?.headers).get("Authorization") ?? "");
    if (url.pathname.endsWith("/login")) {
      return ok({ token: "tok" });
    }
    return ok({ content: null });
  });
  await expect(client.listDir({
    path: "/",
    password: "",
    page: 1,
    size: 100,
    refresh: false,
  })).resolves.toEqual([]);
  expect(paths).toEqual(["/api/auth/login", "/api/fs/list"]);
  expect(auths[1]).toBe("tok");
});

test("listDir 解析 content, null 当空列表", async () => {
  const bodies: unknown[] = [];
  const client = new AlistClient(runtime, async (input, init) => {
    const url = new URL(String(input));
    if (url.pathname.endsWith("/login")) {
      return ok({ token: "tok" });
    }
    bodies.push(JSON.parse(String(init?.body)));
    return ok({
      content: [
        {
          name: "a.txt",
          size: 3,
          is_dir: false,
          modified: "2024-01-01T00:00:00.000Z",
          sign: "",
          thumb: "",
          type: 2,
        },
      ],
    });
  });
  const entries = await client.listDir({
    path: "/disk",
    password: "p",
    page: 2,
    size: 50,
    refresh: true,
  });
  expect(bodies[0]).toEqual({
    path: "/disk",
    password: "p",
    page: 2,
    per_page: 50,
    refresh: true,
  });
  expect(entries).toEqual([
    {
      name: "a.txt",
      size: 3,
      isDir: false,
      modified: "2024-01-01T00:00:00.000Z",
      sign: "",
      thumb: "",
      type: 2,
    },
  ]);
});

test("envelope code 非 200 失败", async () => {
  const client = new AlistClient(runtime, async () =>
    new Response(JSON.stringify({ code: 401, message: "guest user is disabled", data: null }), {
      status: 200,
    }),
  );
  await expect(client.listDir({
    path: "/",
    password: "",
    page: 1,
    size: 100,
    refresh: false,
  })).rejects.toBeInstanceOf(AlistErr);
});

test("put 走 PUT /api/fs/put, File-Path 编码", async () => {
  const seen: Array<{ method?: string; path: string; filePath: string; asTask: string; len: string }> = [];
  const src = tmpFile("hello");
  const client = new AlistClient(runtime, async (input, init) => {
    const url = new URL(String(input));
    if (url.pathname.endsWith("/login")) {
      return ok({ token: "tok" });
    }
    const headers = new Headers(init?.headers);
    seen.push({
      method: init?.method,
      path: url.pathname,
      filePath: headers.get("File-Path") ?? "",
      asTask: headers.get("As-Task") ?? "",
      len: headers.get("Content-Length") ?? "",
    });
    return ok(null);
  });
  await expect(client.putFile(src, "/baidu/a b.txt", false, "")).resolves.toBeUndefined();
  expect(seen[0]).toEqual({
    method: "PUT",
    path: "/api/fs/put",
    filePath: "%2Fbaidu%2Fa%20b.txt",
    asTask: "false",
    len: "5",
  });
});

test("put as-task 解析 task", async () => {
  const src = tmpFile("hello");
  const client = new AlistClient(runtime, async (input) => {
    const url = new URL(String(input));
    if (url.pathname.endsWith("/login")) {
      return ok({ token: "tok" });
    }
    return ok({
      id: "t1",
      name: "a",
      progress: 0,
      state: 0,
      status: "pending",
      error: "",
    });
  });
  await expect(client.putFile(src, "/a", true, "")).resolves.toEqual({
    id: "t1",
    name: "a",
    progress: 0,
    state: 0,
    status: "pending",
    error: "",
  });
});

test("mkdir POST /api/fs/mkdir", async () => {
  const bodies: unknown[] = [];
  const client = new AlistClient(runtime, async (input, init) => {
    const url = new URL(String(input));
    if (url.pathname.endsWith("/login")) {
      return ok({ token: "tok" });
    }
    expect(url.pathname).toBe("/api/fs/mkdir");
    bodies.push(JSON.parse(String(init?.body)));
    return ok(null);
  });
  await client.mkdir("/baidu/test-dir", "");
  expect(bodies[0]).toEqual({ path: "/baidu/test-dir", password: "" });
});
