import { expect, test } from "bun:test";
import { parseArgs } from "../src/args";
import { buildArgv } from "../src/argv";
import { buildRemote } from "../src/conn";
import { BUILTIN_EXCLUDES, HIDDEN_EXCLUDES, makePlan, resolveRel, stripBin } from "../src/plan";
import { parseStats } from "../src/stats";
import { RcloneErr } from "../src/types";
import { parseCfg, sampleCfg, samplePair } from "./fixture";

const cfgFile = "/tmp/rclone-cli-empty.conf";

function planFor(
  action: "push" | "pull" | "check" | "get" | "download",
  body = sampleCfg(),
  given?: string,
  audience: "human" | "agent" = "human",
  dryRun = false,
) {
  const cfg = parseCfg(body);
  return makePlan(cfg, action, "photograph-123", given, dryRun, audience);
}

test("push argv is copy, never sync, and does not point at rclone.conf", () => {
  const plan = planFor("push");
  const remote = buildRemote(plan.target, { pass: "obscured" }, plan.rel, plan.nameEnc);
  const argv = buildArgv(plan, cfgFile, remote);
  expect(argv[0]).toBe("--config");
  expect(argv[1]).toBe(cfgFile);
  expect(argv[2]).toBe("copy");
  expect(argv.includes("sync")).toBe(false);
  expect(argv.join(" ")).not.toContain("rclone/rclone.conf");
  expect(argv).toContain("--skip-links");
  expect(argv).toContain("--size-only");
  expect(argv).toContain("--progress");
  expect(argv[argv.indexOf("--retries") + 1]).toBe("10");
  for (const pattern of BUILTIN_EXCLUDES) {
    expect(argv).toContain(pattern);
  }
  for (const pattern of HIDDEN_EXCLUDES) {
    expect(argv).toContain(pattern);
  }
  expect(argv.join(" ")).not.toContain("_work/**");
});

test("skipHidden false drops hidden excludes and keeps builtin", () => {
  const plan = planFor("push", sampleCfg({ skipHidden: false }));
  const argv = buildArgv(plan, cfgFile, "REMOTE");
  for (const pattern of HIDDEN_EXCLUDES) {
    expect(argv).not.toContain(pattern);
  }
  for (const pattern of BUILTIN_EXCLUDES) {
    expect(argv).toContain(pattern);
  }
});

test("pull swaps ends and never syncs", () => {
  const pulled = planFor("pull");
  const pullArgv = buildArgv(pulled, cfgFile, "REMOTE");
  expect(pullArgv[2]).toBe("copy");
  expect(pullArgv[3]).toBe("REMOTE");
  expect(pullArgv[4]).toBe(pulled.local);
  expect(pullArgv.includes("sync")).toBe(false);
});

test("agent and explicit false drop progress size-only and skip-links", () => {
  const body = sampleCfg({ sizeOnly: false, skipLinks: false });
  const plan = planFor("push", body, undefined, "agent", true);
  const argv = buildArgv(plan, cfgFile, "REMOTE");
  expect(argv).not.toContain("--progress");
  expect(argv).not.toContain("--size-only");
  expect(argv).not.toContain("--skip-links");
  expect(argv).toContain("--dry-run");
});

test("task can turn name encryption off and raise transfers", () => {
  const plan = planFor("push", sampleCfg({
    crypt: [samplePair()],
    encryptFilenames: true,
    task: { encryptFilenames: false, transfers: 8, tpslimit: 8 },
  }));
  const remote = buildRemote(plan.target, {
    pass: "obscured",
    crypt: [{ name: "v1", password: "one", password2: "two" }],
  }, plan.rel, plan.nameEnc);
  const argv = buildArgv(plan, cfgFile, remote);
  expect(plan.nameEnc).toBe("off");
  expect(remote).toContain("filename_encryption=off");
  expect(remote).toContain("directory_name_encryption=false");
  expect(remote).toContain("suffix='none'");
  expect(argv[argv.indexOf("--transfers") + 1]).toBe("8");
  expect(argv[argv.indexOf("--tpslimit") + 1]).toBe("8");
  expect(argv.includes("sync")).toBe(false);
});

test("task override wins and _work is not excluded", () => {
  const plan = planFor("push", sampleCfg({
    task: { excludes: ["*.tmp"], transfers: 8, tpslimit: 9, timeout: "2h" },
  }));
  const argv = buildArgv(plan, cfgFile, "REMOTE");
  expect(argv).toContain("*.tmp");
  expect(argv[argv.indexOf("--transfers") + 1]).toBe("8");
  expect(argv[argv.indexOf("--tpslimit") + 1]).toBe("9");
  expect(argv[argv.indexOf("--timeout") + 1]).toBe("2h");
  expect(argv[argv.indexOf("--checkers") + 1]).toBe("4");
  expect(argv.join(" ")).not.toContain("_work/**");
});

test("get uses copyto and swaps ends", () => {
  const plan = planFor("get", sampleCfg(), "album/DSC_0001.NEF");
  const argv = buildArgv(plan, cfgFile, "REMOTE");
  expect(argv[2]).toBe("copyto");
  expect(argv[3]).toBe("REMOTE");
  expect(argv[4]).toBe(plan.local);
  expect(argv.includes("sync")).toBe(false);
  expect(argv).not.toContain("--exclude");
  expect(argv).toContain("-q");
  expect(argv).not.toContain("--progress");
  expect(argv[argv.indexOf("--retries") + 1]).toBe("1");
  expect(argv[argv.indexOf("--retries-sleep") + 1]).toBe("1s");
  expect(argv[argv.indexOf("--low-level-retries") + 1]).toBe("1");
});

test("download defaults to cwd basename", () => {
  const cfg = parseCfg(sampleCfg());
  const plan = makePlan(cfg, "download", "photograph-123", "album/DSC_0001.NEF", false, "human", undefined, "/tmp/x");
  expect(plan.local).toBe("/tmp/x/DSC_0001.NEF");
  const argv = buildArgv(plan, cfgFile, "REMOTE");
  expect(argv[2]).toBe("copyto");
  expect(argv[4]).toBe("/tmp/x/DSC_0001.NEF");
  expect(argv).toContain("-q");
  expect(argv[argv.indexOf("--retries") + 1]).toBe("1");
});

test("download -v drops quiet and shows rclone logs", () => {
  const cfg = parseCfg(sampleCfg());
  const plan = makePlan(cfg, "download", "photograph-123", "album/a.JPG", false, "human", undefined, "/tmp/x", true);
  const argv = buildArgv(plan, cfgFile, "REMOTE");
  expect(plan.verbose).toBe(true);
  expect(argv).toContain("-v");
  expect(argv).not.toContain("-q");
  expect(argv).toContain("--progress");
});

test("parseArgs treats -v as verbose", () => {
  const quiet = parseArgs(["download", "photograph-123", "a.JPG"]);
  expect(quiet).toMatchObject({ kind: "xfer", verbose: false });
  const loud = parseArgs(["download", "photograph-123", "a.JPG", ".", "-v"]);
  expect(loud).toMatchObject({ kind: "xfer", dest: ".", verbose: true });
});

test("download of a .bin path strips the suffix into dest dir", () => {
  const cfg = parseCfg(sampleCfg());
  const plan = makePlan(
    cfg,
    "download",
    "photograph-123",
    "2025-04-04/DSC_0746.JPG.bin",
    false,
    "human",
    ".",
    "/tmp/x",
  );
  expect(plan.rel).toBe("2025-04-04/DSC_0746.JPG");
});

test("pull of a file path is rejected", () => {
  expect(() => planFor("pull", sampleCfg(), "album/DSC_0001.NEF")).toThrow("download one file");
});

test("root must match the source folder name", () => {
  const body = sampleCfg();
  const targets = body.targets as Record<string, unknown>[];
  const target = targets[0];
  if (target === undefined) {
    throw new Error("missing target");
  }
  target.root = "photograph-crypt";
  const cfg = parseCfg(body);
  expect(() => makePlan(cfg, "push", "photograph-123", undefined, false, "human"))
    .toThrow("root must match source folder photograph");
});

test("stripBin drops a trailing crypt suffix", () => {
  expect(stripBin("album/DSC_0746.JPG.bin")).toBe("album/DSC_0746.JPG");
  expect(stripBin("album/DSC_0746.JPG")).toBe("album/DSC_0746.JPG");
});

test("rel path must stay inside the source", () => {
  expect(resolveRel("/src", "a/b")).toEqual({ local: "/src/a/b", rel: "a/b" });
  expect(resolveRel("/src").rel).toBe("");
  expect(() => resolveRel("/src", "../etc")).toThrow(RcloneErr);
  expect(() => resolveRel("/src", "/etc")).toThrow("path outside source");
});

test("stats parse the last rclone summary", () => {
  const stderr = [
    "Transferred:   \t 1.000 KiB / 1.000 KiB, 100%, 1 KiB/s, ETA 0s",
    "Checks:                 2 / 2, 100%",
    "Transferred:            3 / 3, 100%",
    "Elapsed time:         1.0s",
  ].join("\n");
  expect(parseStats(stderr)).toEqual({ bytes: 1024, checks: 2, transfers: 3 });
  expect(parseStats("no stats")).toEqual({});
});
