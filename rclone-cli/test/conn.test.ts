import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { buildLocalCrypt, buildPlainRemote, buildRemote, parseConn } from "../src/conn";
import { ensureEmptyConf } from "../src/config";
import { obscureOne, spawnProc } from "../src/spawn";
import type { Target } from "../src/types";
import { parseCfg, sampleCfg, samplePair, tempHome } from "./fixture";

const secret = "p@ss,w:rd='x'\"y";

function targetOf(body: Record<string, unknown> = sampleCfg()): Target {
  const cfg = parseCfg(body);
  const target = cfg.targets[0];
  if (target === undefined) {
    throw new Error("missing target");
  }
  return target;
}

test("password with comma colon and quotes round-trips", () => {
  const target = targetOf();
  const remote = buildRemote(target, { pass: secret }, "album", "standard");
  const parsed = parseConn(remote);
  expect(parsed.config.pass).toBe(secret);
  expect(parsed.config.pacer_min_sleep).toBe("200ms");
  expect(parsed.config.url).toBe(target.url);
  expect(parsed.path).toBe("photograph/album");
  expect(remote.startsWith(":webdav,")).toBe(true);
});

test("crypt folder name is root, subpath stays on the outer path", () => {
  const target = targetOf(sampleCfg({
    crypt: [samplePair({ key: "one", salt: "two,x:y" })],
  }));
  const remote = buildRemote(target, {
    pass: secret,
    crypt: [{ name: "v1", password: "one'one", password2: "two,x:y" }],
  }, "album", "standard");
  const outer = parseConn(remote);
  expect(outer.name).toBe(":crypt");
  expect(outer.path).toBe("album");
  expect(outer.config.password).toBe("one'one");
  expect(outer.config.password2).toBe("two,x:y");
  expect(outer.config.filename_encryption).toBe("standard");
  expect(outer.config.directory_name_encryption).toBe("true");
  const innerRaw = outer.config.remote;
  if (innerRaw === undefined) {
    throw new Error("missing nested remote");
  }
  const inner = parseConn(innerRaw);
  expect(inner.config.pass).toBe(secret);
  expect(inner.path).toBe("photograph");
  expect(remote.includes("photograph-crypt")).toBe(false);
});

test("encryptFilenames off leaves file and directory names plain", () => {
  const target = targetOf(sampleCfg({
    crypt: [samplePair()],
  }));
  const remote = buildRemote(target, {
    pass: secret,
    crypt: [{ name: "v1", password: "one", password2: "two" }],
  }, "", "off");
  const outer = parseConn(remote);
  expect(outer.config.filename_encryption).toBe("off");
  expect(outer.config.directory_name_encryption).toBe("false");
  expect(outer.config.suffix).toBe("none");
  expect(outer.path).toBe("");
  const innerRaw = outer.config.remote;
  if (innerRaw === undefined) {
    throw new Error("missing nested remote");
  }
  expect(parseConn(innerRaw).path).toBe("photograph");
});

test("plain webdav remote keeps the ciphertext name", () => {
  const target = targetOf(sampleCfg({ crypt: [samplePair()] }));
  const remote = buildPlainRemote(target, secret, "album/DSC_0746.JPG.bin");
  expect(remote.startsWith(":webdav,")).toBe(true);
  expect(remote).toContain("photograph/album/DSC_0746.JPG.bin");
  expect(remote).not.toContain(":crypt");
});

test("local crypt decrypts a file we already fetched", () => {
  const remote = buildLocalCrypt("/tmp/blob", { name: "v1", password: "one", password2: "two" }, "DSC_0746.JPG.bin");
  const parsed = parseConn(remote);
  expect(parsed.name).toBe(":crypt");
  expect(parsed.path).toBe("DSC_0746.JPG.bin");
  expect(parsed.config.remote).toBe("/tmp/blob");
  expect(parsed.config.filename_encryption).toBe("off");
  expect(parsed.config.suffix).toBe("none");
});

test("legacy bin suffix is quoted on the crypt remote", () => {
  const target = targetOf(sampleCfg({
    crypt: [samplePair()],
  }));
  const remote = buildRemote(target, {
    pass: secret,
    crypt: [{ name: "v1", password: "one", password2: "two" }],
  }, "album/DSC_0001.NEF", "off", ".bin");
  const outer = parseConn(remote);
  expect(outer.config.suffix).toBe(".bin");
  expect(outer.path).toBe("album/DSC_0001.NEF");
});

test("rclone obscure value survives the connection string", async () => {
  const home = tempHome();
  const empty = ensureEmptyConf(home);
  const plain = "p@ss,w:rd";
  const obscured = await obscureOne("rclone", empty, plain, spawnProc);
  expect(obscured).not.toContain(plain);
  expect(readFileSync(empty, "utf8")).toBe("");
  const target = targetOf();
  const remote = buildRemote(target, { pass: obscured }, "", "standard");
  const parsed = parseConn(remote);
  expect(parsed.config.pass).toBe(obscured);
  expect(parsed.path).toBe("photograph");
  expect(remote).not.toContain(plain);
});
