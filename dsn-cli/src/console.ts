import { spawnSync } from "node:child_process";
import { DsnErr, type Kind } from "./types";

type ConsoleSpec = {
  bin: string;
  args: string[];
};

/** 为什么: 人的最短路径是官方客户端, dsn-cli 只负责把 Profile.url 展开成 argv. */
export function consoleSpec(kind: Kind, url: string): ConsoleSpec {
  if (kind === "mysql" || kind === "doris") {
    return mysqlSpec(url);
  }
  if (kind === "redis") {
    return redisSpec(url);
  }
  if (kind === "mongodb") {
    return { bin: "mongosh", args: [url] };
  }
  throw new Error(`kind ${kind} has no Console`);
}

function mysqlSpec(url: string): ConsoleSpec {
  const parsed = new URL(url);
  const args: string[] = [];
  if (parsed.hostname !== "") {
    args.push("-h", parsed.hostname);
  }
  if (parsed.port !== "") {
    args.push("-P", parsed.port);
  }
  if (parsed.username !== "") {
    args.push("-u", decodeURIComponent(parsed.username));
  }
  if (parsed.password !== "") {
    args.push(`-p${decodeURIComponent(parsed.password)}`);
  }
  const db = decodeURIComponent(parsed.pathname.replace(/^\//, ""));
  if (db !== "") {
    args.push(db);
  }
  return { bin: "mysql", args };
}

function redisSpec(url: string): ConsoleSpec {
  const parsed = new URL(url);
  const args: string[] = [];
  if (parsed.hostname !== "") {
    args.push("-h", parsed.hostname);
  }
  if (parsed.port !== "") {
    args.push("-p", parsed.port);
  }
  if (parsed.password !== "") {
    args.push("-a", decodeURIComponent(parsed.password));
  }
  const db = parsed.pathname.replace(/^\//, "");
  if (db !== "") {
    args.push("-n", db);
  }
  return { bin: "redis-cli", args };
}

export function runConsole(kind: Kind, url: string): number {
  const spec = consoleSpec(kind, url);
  const ret = spawnSync(spec.bin, spec.args, { stdio: "inherit" });
  if (ret.error !== undefined) {
    const err = ret.error as NodeJS.ErrnoException;
    if (err.code === "ENOENT") {
      throw new Error(`${spec.bin} not found on PATH`);
    }
    throw new DsnErr(err.message, 3);
  }
  return ret.status ?? 1;
}

export function isTty(
  stdin = process.stdin,
  stdout = process.stdout,
): boolean {
  return stdin.isTTY === true && stdout.isTTY === true;
}
