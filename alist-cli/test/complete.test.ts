import { afterEach, expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { completeValues } from "../src/complete";

const origHome = process.env.HOME;
const homes: string[] = [];

afterEach(() => {
  for (const dir of homes) {
    rmSync(dir, { recursive: true, force: true });
  }
  homes.length = 0;
  if (origHome !== undefined) {
    process.env.HOME = origHome;
  } else {
    delete process.env.HOME;
  }
});

test("根命令候选", async () => {
  expect(await completeValues([], "")).toContain("ls");
  expect(await completeValues([], "")).toContain("sync");
  expect(await completeValues(["l"], "l")).toContain("ls");
});

test("completion 子命令", async () => {
  expect(await completeValues(["completion"], "")).toEqual(["fish"]);
});

test("-p 补全 profile 名", async () => {
  const home = mkdtempSync(join(tmpdir(), "alist-comp-"));
  homes.push(home);
  mkdirSync(join(home, ".config", "alist-cli"), { recursive: true });
  writeFileSync(
    join(home, ".config", "alist-cli", "config.json"),
    JSON.stringify({
      defaultProfile: "home",
      profiles: [
        { name: "home", url: "http://a", username: "u", password: "p" },
        { name: "work", url: "http://b", username: "u", password: "p" },
      ],
    }),
  );
  process.env.HOME = home;
  expect(await completeValues(["-p"], "")).toEqual(["home", "work"]);
});
