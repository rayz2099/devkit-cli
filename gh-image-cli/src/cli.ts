#!/usr/bin/env bun

import { existsSync } from "node:fs";
import { relative } from "node:path";
import { fishCompletion } from "./completion";
import { loadConfig } from "./config";
import { ImagesStore, splitSource, validateVersion } from "./images";
import { runBuild } from "./actions";
import { resolveLocalRepo, resolveRepo } from "./repo";
import type { ImageRecord } from "./types";

function usage(): string {
  return [
    "usage:",
    "  gh-image-cli add <source> <alias> [version]",
    "  gh-image-cli add-dockerfile <script> <alias> [version]",
    "  gh-image-cli build <alias> [version]",
    "  gh-image-cli list [alias]",
    "  gh-image-cli completion fish",
  ].join("\n");
}

function target(registry: string, namespace: string, alias: string, version?: string): string {
  const base = `${registry}/${namespace}/${alias}`;
  return version === undefined ? base : `${base}:${version}`;
}

function sourceLabel(image: ImageRecord): string {
  return image.type === "mirror" ? image.source : image.script;
}

function requireArgs(args: string[], min: number, max: number): void {
  if (args.length < min || args.length > max) {
    throw new Error(usage());
  }
}

function add(args: string[]): void {
  requireArgs(args, 2, 3);
  const [rawSource, alias, rawVersion] = args as [string, string, string?];
  const parsed = splitSource(rawSource);
  if (parsed.version !== undefined && rawVersion !== undefined && parsed.version !== rawVersion) {
    throw new Error(`source tag ${parsed.version} conflicts with version ${rawVersion}`);
  }
  const version = rawVersion ?? parsed.version ?? "latest";
  validateVersion(version);
  const repo = resolveLocalRepo();
  const store = new ImagesStore(repo.imagesPath);
  store.addMirror(parsed.source, alias, version);
  store.save();
  console.log(`added: ${alias}:${version} <- ${parsed.source}:${version}`);
}

function addDockerfile(args: string[]): void {
  requireArgs(args, 2, 3);
  const [script, alias, version = "latest"] = args as [string, string, string?];
  const repo = resolveLocalRepo();
  if (!existsSync(`${repo.root}/${script}`)) {
    throw new Error(`Dockerfile build script not found: ${script}`);
  }
  const store = new ImagesStore(repo.imagesPath);
  store.addDockerfile(script, alias, version);
  store.save();
  console.log(`added: ${alias}:${version} <- ${script}`);
}

async function build(args: string[]): Promise<number> {
  requireArgs(args, 1, 2);
  const [alias, requestedVersion] = args as [string, string?];
  const repo = resolveRepo();
  const store = new ImagesStore(repo.imagesPath);
  const image = store.require(alias);
  const version = requestedVersion ?? store.versions(alias)[0] ?? "latest";
  validateVersion(version);
  store.setStatus(alias, version, "init", requestedVersion !== undefined || store.versions(alias).length === 0);
  store.save();

  const cfg = loadConfig();
  const dst = target(cfg.registry, cfg.namespace, alias, version);
  console.log(`alias: ${alias}`);
  console.log(`${image.type === "mirror" ? "source" : "script"}: ${sourceLabel(image)}`);
  console.log(`version: ${version}`);
  console.log(`workflow: ${image.type === "mirror" ? "sync-image.yml" : "build-dockerfile.yml"}`);
  console.log(`target: ${dst}`);

  let result;
  try {
    result = await runBuild(repo, image, alias, version, cfg);
  } catch (err) {
    // why: dispatch/查询异常同样代表本次同步没有完成, 不能让状态永久停在 init.
    const latest = new ImagesStore(repo.imagesPath);
    latest.setStatus(alias, version, "failed");
    latest.save();
    throw err;
  }
  // why: Action 等待期间用户可能编辑台账, 终态基于最新文件写入以免覆盖并发变更.
  const latest = new ImagesStore(repo.imagesPath);
  latest.setStatus(alias, version, result.status);
  latest.save();
  console.log(`status: ${result.status}`);
  console.log(`run: ${result.runUrl}`);
  console.log(`target: ${dst}`);
  if (result.status === "done") {
    return 0;
  }
  return result.status === "timeout" ? 124 : 1;
}

function list(args: string[]): void {
  requireArgs(args, 0, 1);
  const [filter] = args;
  const repo = resolveLocalRepo();
  const store = new ImagesStore(repo.imagesPath);
  if (filter !== undefined) {
    store.require(filter);
  }
  const cfg = loadConfig();
  const aliases = filter === undefined ? store.aliases() : [filter];
  console.log("ALIAS\tTYPE\tSOURCE/SCRIPT\tTARGET");
  for (const alias of aliases) {
    const image = store.require(alias);
    console.log(`${alias}\t${image.type}\t${sourceLabel(image)}\t${target(cfg.registry, cfg.namespace, alias)}`);
  }
}

function complete(args: string[]): void {
  try {
    const [kind, alias] = args;
    const repo = resolveLocalRepo();
    const store = new ImagesStore(repo.imagesPath);
    if (kind === "aliases") {
      console.log(store.aliases().join("\n"));
    } else if (kind === "versions" && alias !== undefined) {
      console.log(store.versions(alias).join("\n"));
    }
  } catch {
    // 补全属于交互热路径, 非项目目录和缺少 images.yaml 时必须静默.
  }
}

export async function main(args: string[]): Promise<number> {
  const [cmd, ...rest] = args;
  switch (cmd) {
    case undefined:
    case "help":
    case "-h":
    case "--help":
      console.log(usage());
      return 0;
    case "add":
      add(rest);
      return 0;
    case "add-dockerfile":
      addDockerfile(rest);
      return 0;
    case "build":
      return build(rest);
    case "list":
      list(rest);
      return 0;
    case "completion":
      if (rest.length !== 1 || rest[0] !== "fish") {
        throw new Error("only fish completion is supported");
      }
      console.log(fishCompletion());
      return 0;
    case "__complete":
      complete(rest);
      return 0;
    default:
      throw new Error(`unknown command: ${cmd}\n${usage()}`);
  }
}

if (import.meta.main) {
  try {
    process.exitCode = await main(Bun.argv.slice(2));
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error(`gh-image-cli: ${msg}`);
    process.exitCode = 2;
  }
}
