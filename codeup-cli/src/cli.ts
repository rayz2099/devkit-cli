#!/usr/bin/env bun
import { CodeupErr } from "./types";
import { runCmd } from "./run";

/** 为什么: 入口只负责进程码, API/用法错误必须分开. */
export async function main(argv: string[] = Bun.argv.slice(2)): Promise<number> {
  try {
    process.stdout.write(await runCmd(argv));
    return 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    process.stderr.write(`${message}\n`);
    return error instanceof CodeupErr ? error.code : 1;
  }
}

if (import.meta.main) {
  process.exit(await main());
}
