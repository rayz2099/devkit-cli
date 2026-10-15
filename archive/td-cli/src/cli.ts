import { parseArgs, takeCmd } from "./args";
import { errBody } from "./output";
import { runCmd } from "./run";
import type { Audience } from "./types";

/** 为什么: 进程码和 agent 的 JSON 错误都在入口收口, 命令本身只返回正文. */
export async function main(argv: string[] = Bun.argv.slice(2)): Promise<number> {
  const audience = audienceOf(argv);
  try {
    process.stdout.write(await runCmd(parseArgs(argv)));
    return 0;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (audience === "agent") {
      process.stdout.write(errBody(message));
    } else {
      process.stderr.write(`${message}\n`);
    }
    return 1;
  }
}

function audienceOf(argv: string[]): Audience {
  try {
    const pos = takeCmd(argv).pos;
    if (pos[0] === "agent" || pos[0] === "human") {
      return pos[0];
    }
  } catch {
    return "human";
  }
  return "human";
}

if (import.meta.main) {
  process.exit(await main());
}
