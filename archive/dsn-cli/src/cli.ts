#!/usr/bin/env bun
import { runCmd } from "./run";
import { DsnErr } from "./types";

/** 为什么: 入口只负责进程码, Gate/驱动必须和用法错误分开. */
export async function main(argv: string[] = Bun.argv.slice(2)): Promise<number> {
  try {
    const out = await runCmd(argv);
    if (out.type === "exit") {
      return out.code;
    }
    process.stdout.write(out.body);
    return out.code ?? 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    process.stderr.write(`${message}\n`);
    return error instanceof DsnErr ? error.code : 1;
  }
}

if (import.meta.main) {
  process.exit(await main());
}
