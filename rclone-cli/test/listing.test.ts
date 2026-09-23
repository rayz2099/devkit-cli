import { expect, test } from "bun:test";
import { checkFailed, clipInv, computeInv, parseLsJson } from "../src/listing";

test("parseLsJson keeps files and drops dirs", () => {
  const items = parseLsJson(JSON.stringify([
    { Path: "album/a.NEF", Size: 10, IsDir: false },
    { Path: "album", Size: 0, IsDir: true },
    { Path: "album/b.NEF.bin", Size: 20 },
  ]));
  expect(items).toEqual([
    { path: "album/a.NEF", size: 10 },
    { path: "album/b.NEF.bin", size: 20 },
  ]);
});

test("computeInv treats leftover .bin as the same object", () => {
  const report = computeInv("photograph-123", "", [
    { path: "a.NEF", size: 10 },
    { path: "b.NEF", size: 20 },
    { path: "c.NEF", size: 30 },
  ], [
    { path: "a.NEF", size: 10 },
    { path: "b.NEF.bin", size: 20 },
    { path: "c.NEF", size: 99 },
    { path: "only-remote.JPG", size: 1 },
  ], true);
  expect(report.counts).toEqual({
    ok: 1,
    "ok-bin": 1,
    missing: 0,
    differ: 1,
    extra: 1,
  });
  expect(checkFailed(report)).toBe(true);
  expect(report.rows.find((row) => row.path === "b.NEF")?.state).toBe("ok-bin");
  expect(report.rows.find((row) => row.path === "c.NEF")?.via).toBe("name");
  expect(report.rows.find((row) => row.path === "only-remote.JPG")?.state).toBe("extra");
});

test("same-name wins when both name and .bin exist", () => {
  const report = computeInv("t", "", [
    { path: "a.NEF", size: 10 },
  ], [
    { path: "a.NEF", size: 10 },
    { path: "a.NEF.bin", size: 10 },
  ], true);
  expect(report.counts.ok).toBe(1);
  expect(report.counts.extra).toBe(1);
  expect(report.rows.find((row) => row.path === "a.NEF.bin")?.state).toBe("extra");
  expect(checkFailed(report)).toBe(false);
});

test("clipInv keeps full counts and slices rows", () => {
  const report = computeInv("t", "", [
    { path: "a.NEF", size: 1 },
    { path: "b.NEF", size: 2 },
    { path: "c.NEF", size: 3 },
  ], [
    { path: "a.NEF", size: 1 },
    { path: "b.NEF", size: 2 },
    { path: "c.NEF", size: 3 },
  ], false);
  const clipped = clipInv(report, 2);
  expect(clipped.rows).toHaveLength(2);
  expect(clipped.total).toBe(3);
  expect(clipped.limit).toBe(2);
  expect(clipped.counts.ok).toBe(3);
});

test("plain tasks do not alias .bin", () => {
  const report = computeInv("t", "", [
    { path: "a.NEF", size: 10 },
  ], [
    { path: "a.NEF.bin", size: 10 },
  ], false);
  expect(report.counts.missing).toBe(1);
  expect(report.counts.extra).toBe(1);
  expect(checkFailed(report)).toBe(true);
});
