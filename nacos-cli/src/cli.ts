#!/usr/bin/env bun
import { parseArgs } from "./args";
import { runCmd } from "./run";

/** 为什么: 入口只负责进程码, 业务错误统一落到 stderr. */
export async function main(argv: string[] = Bun.argv.slice(2)): Promise<number> {
  try {
    process.stdout.write(await runCmd(parseArgs(argv)));
    return 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    process.stderr.write(`${message}\n`);
    return 1;
  }
}

if (import.meta.main) {
  process.exit(await main());
}
