#!/usr/bin/env bun

import {
  copyFileSync,
  existsSync,
  mkdirSync,
  writeFileSync,
} from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { discoverMods, selectMods, type CliMod } from "./modules";

type Cmd = "list" | "clean" | "build" | "install";

const root = dirname(dirname(fileURLToPath(import.meta.url)));

function run(
  cwd: string,
  args: string[],
): void {
  const [bin, ...rest] = args;
  if (bin === undefined) {
    throw new Error("command args must not be empty");
  }

  const ret = spawnSync(bin, rest, {
    cwd,
    stdio: "inherit",
  });
  if (ret.status !== 0) {
    throw new Error(`command failed: ${args.join(" ")}`);
  }
}

function listMods(mods: CliMod[]): void {
  for (const mod of mods) {
    console.log(mod.name);
  }
}

function runJust(
  mod: CliMod,
  task: Cmd,
): void {
  run(mod.root, [
    "just",
    task,
  ]);
  console.log(`${task} ${mod.name}`);
}

/**
 * 仅在 XDG 配置缺失时从 example 复制, 避免开源仓示例覆盖本机真配置.
 */
export function bootstrapModConf(
  mod: CliMod,
  home: string,
): void {
  const pairs: Array<[string, string]> = [
    ["config.example.json", "config.json"],
    ["project.example.yml", "project.yml"],
    ["config.example.yml", "config.yml"],
  ];

  const dstDir = join(home, ".config", mod.name);
  let ensured = false;

  for (const [srcName, dstName] of pairs) {
    const src = join(mod.root, srcName);
    if (!existsSync(src)) {
      continue;
    }

    if (!ensured) {
      mkdirSync(dstDir, {
        recursive: true,
      });
      ensured = true;
    }

    const dst = join(dstDir, dstName);
    if (existsSync(dst)) {
      console.log(`skip existing ${mod.name} config ${dstName}`);
      continue;
    }

    copyFileSync(src, dst);
    console.log(`bootstrap ${mod.name} config ${dstName} -> ${dst}`);
  }

  // code-ws 等模块的 templates 仍在仓库内, 用标记让 XDG 配置能反查 module root.
  const templatesDir = join(mod.root, "templates");
  if (existsSync(templatesDir)) {
    if (!ensured) {
      mkdirSync(dstDir, {
        recursive: true,
      });
      ensured = true;
    }
    const marker = join(dstDir, "module-root");
    writeFileSync(marker, `${mod.root}\n`);
    console.log(`bootstrap ${mod.name} module-root -> ${marker}`);
  }
}

function installMod(mod: CliMod): void {
  if (process.env.HOME === undefined || process.env.HOME.length === 0) {
    throw new Error("HOME is required for install");
  }

  runJust(mod, "install");
  bootstrapModConf(mod, process.env.HOME);
}

function usage(): string {
  return [
    "usage: devkit-cli <list|clean|build|install> [module...]",
    "",
    "examples:",
    "  devkit-cli list",
    "  devkit-cli build",
    "  devkit-cli clean code-ws",
  ].join("\n");
}

export function main(args: string[]): void {
  const [cmd, ...names] = args;
  if (cmd === undefined || cmd === "help" || cmd === "-h" || cmd === "--help") {
    console.log(usage());
    return;
  }

  if (!["list", "clean", "build", "install"].includes(cmd)) {
    throw new Error(`unknown command: ${cmd}`);
  }

  const mods = discoverMods(root);
  const selected = selectMods(mods, names);
  const action = cmd as Cmd;

  if (action === "list") {
    listMods(selected);
    return;
  }

  for (const mod of selected) {
    if (action === "clean") {
      runJust(mod, "clean");
    } else if (action === "build") {
      runJust(mod, "build");
    } else {
      installMod(mod);
    }
  }
}

if (import.meta.main) {
  try {
    main(Bun.argv.slice(2));
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error(`devkit-cli: ${msg}`);
    process.exit(1);
  }
}
