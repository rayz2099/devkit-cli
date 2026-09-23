#!/usr/bin/env bun
import { defaultDeps, runCmd } from "./run";
import type { Deps } from "./run";
import { RcloneErr } from "./types";

/** 为什么: 入口只负责进程码. 用法错误走 stderr, 传输摘要走 stdout, 这样失败的 copy 仍能被上层读到. */
export async function main(argv: string[] = Bun.argv.slice(2), deps?: Deps): Promise<number> {
  try {
    const out = await runCmd(argv, deps ?? defaultDeps());
    writeOut(out.text);
    return out.code;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    process.stderr.write(`${message}\n`);
    if (error instanceof RcloneErr) {
      return error.code;
    }
    return 1;
  }
}

function writeOut(text: string): void {
  if (text === "") {
    return;
  }
  if (text.endsWith("\n")) {
    process.stdout.write(text);
    return;
  }
  process.stdout.write(`${text}\n`);
}

if (import.meta.main) {
  process.exit(await main());
}
