import { expect, test } from "bun:test";
import { checkProfiles, doctorOut, DOCTOR_CONC } from "../src/doctor";
import { probeStmt } from "../src/drivers";
import type { Kind, Profile, Timeouts } from "../src/types";

const timeouts: Timeouts = { connectMs: 40, execMs: 40 };

function profile(name: string, kind: Kind = "mysql"): Profile {
  return {
    name,
    kind,
    url: `mysql://127.0.0.1:3306/${name}`,
    access: "read",
  };
}

function hang(): Promise<never> {
  return new Promise(() => {});
}

async function fakeProbe(_kind: Kind, url: string): Promise<void> {
  const name = new URL(url).pathname.replace(/^\//, "");
  if (name === "slow") {
    await hang();
  }
  if (name === "down") {
    throw new Error("connection refused");
  }
}

test("probeStmt 按 kind 固定 ping", () => {
  expect(probeStmt("mysql")).toBe("SELECT 1");
  expect(probeStmt("doris")).toBe("SELECT 1");
  expect(probeStmt("redis")).toBe("PING");
  expect(probeStmt("mongodb")).toBe('{"ping":1}');
  expect(probeStmt("elasticsearch")).toBe("GET /");
  expect(probeStmt("kafka")).toBe("ping");
});

test("doctor 并行, 挂死的 profile 不拖住其余", async () => {
  const started = Date.now();
  const rows = await checkProfiles(
    [profile("slow"), profile("fast"), profile("down")],
    timeouts,
    fakeProbe,
  );
  const elapsed = Date.now() - started;
  expect(elapsed).toBeLessThan(250);
  expect(rows.map((row) => `${row.name}:${row.status}`)).toEqual([
    "slow:fail",
    "fast:ok",
    "down:fail",
  ]);
  expect(rows[0]?.error).toContain("timeout");
  expect(rows[2]?.error).toBe("connection refused");
});

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

test("doctor 限并发, 避免连接风暴误报 ETIMEDOUT", async () => {
  let live = 0;
  let peak = 0;
  const rows = await checkProfiles(
    Array.from({ length: 10 }, (_, index) => profile(`p${index}`)),
    { connectMs: 200, execMs: 200 },
    async () => {
      live += 1;
      const load = live;
      peak = Math.max(peak, live);
      await delay(30);
      live -= 1;
      if (load > DOCTOR_CONC) {
        throw new Error("connect ETIMEDOUT");
      }
    },
  );
  expect(peak).toBeLessThanOrEqual(DOCTOR_CONC);
  expect(rows.every((row) => row.status === "ok")).toBe(true);
});

test("doctorOut 固定列", () => {
  const data = doctorOut([
    { name: "buy", kind: "mysql", status: "ok", ms: 12, error: "" },
    { name: "cache", kind: "redis", status: "fail", ms: 80, error: "timeout 80ms" },
  ]);
  expect(data.columns).toEqual(["name", "kind", "status", "ms", "error"]);
  expect(data.rows[1]?.status).toBe("fail");
});
