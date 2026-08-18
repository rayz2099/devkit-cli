import { expect, test } from "bun:test";
import { JenkinsClient } from "../src/client";
import { JenkinsErr } from "../src/types";
import type { Runtime } from "../src/types";

const runtime: Runtime = {
  audience: "agent",
  secret: "token",
  profile: {
    name: "prod",
    url: "http://jenkins.test",
    username: "admin",
    password: "",
    apiToken: "token",
  },
};

test("拒绝 start/rerun 主分支", async () => {
  const client = new JenkinsClient(runtime, async () => new Response("no", { status: 500 }));
  await expect(client.startRun("dt-vrec/master")).rejects.toBeInstanceOf(JenkinsErr);
  await expect(client.rerun("dt-vrec/main", "1")).rejects.toBeInstanceOf(JenkinsErr);
});

test("listRuns 映射 IN_PROGRESS", async () => {
  const client = new JenkinsClient(runtime, async () =>
    new Response(JSON.stringify({
      builds: [{ number: 3, result: null, building: true, timestamp: 1, duration: 0 }],
    })),
  );
  const items = await client.listRuns("dt-vrec/feature/x", 5);
  expect(items[0]?.result).toBe("IN_PROGRESS");
});
