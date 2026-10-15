import { spawnSync } from "node:child_process";

export type CmdResult = {
  stdout: string;
  stderr: string;
  status: number;
};

/**
 * 参数数组直接交给 spawn, 避免 alias、tag 或路径进入 shell 插值链路.
 */
export function runCmd(
  bin: string,
  args: string[],
  cwd?: string,
): CmdResult {
  const ret = spawnSync(bin, args, {
    cwd,
    encoding: "utf8",
  });
  if (ret.error !== undefined) {
    throw ret.error;
  }
  return {
    stdout: ret.stdout ?? "",
    stderr: ret.stderr ?? "",
    status: ret.status ?? 1,
  };
}

export function mustRun(
  bin: string,
  args: string[],
  cwd?: string,
): string {
  const ret = runCmd(bin, args, cwd);
  if (ret.status !== 0) {
    const detail = ret.stderr.trim() || ret.stdout.trim();
    throw new Error(`${bin} ${args.join(" ")} failed${detail ? `: ${detail}` : ""}`);
  }
  return ret.stdout.trim();
}
