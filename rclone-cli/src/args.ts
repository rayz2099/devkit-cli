import { RcloneErr } from "./types";
import type { Action, Audience, CliCmd } from "./types";

const BOOL_FLAGS = new Set(["-h", "--help", "--dry-run", "-v", "--verbose"]);
const XFER: Action[] = ["push", "pull", "check", "get", "download"];

/** 为什么: 自己切 argv, agent 前缀和 --dry-run 才能出现在任务名前面, 不引入命令框架. */
export function parseArgs(argv: string[]): CliCmd {
  if (argv[0] === "completion") {
    if (argv[1] === "fish" && argv.length === 2) {
      return { kind: "completion-fish" };
    }
    throw new RcloneErr(2, "usage: rclone-cli completion fish");
  }
  if (argv[0] === "__complete") {
    return parseComplete(argv.slice(1));
  }
  const taken = takeCmd(argv);
  if (taken.flags.has("-h") || taken.flags.has("--help") || taken.pos[0] === "help") {
    return { kind: "help" };
  }
  const audience = takeAudience(taken.pos);
  const head = taken.pos[0];
  if (head === undefined) {
    return { kind: "usage" };
  }
  if (head === "ls") {
    if (taken.flags.has("--dry-run")) {
      throw new RcloneErr(2, "unknown flag: --dry-run");
    }
    if (taken.pos.length > 3) {
      throw new RcloneErr(2, `unexpected argument: ${taken.pos[3] ?? ""}`);
    }
    const task = taken.pos[1];
    const path = taken.pos[2];
    if (task === undefined) {
      if (taken.flags.has("--limit")) {
        throw new RcloneErr(2, "ls without a task does not take --limit");
      }
      return { kind: "ls", audience: audience.value };
    }
    return {
      kind: "ls",
      audience: audience.value,
      task,
      ...(path === undefined ? {} : { path }),
      limit: readLimit(taken.flags, 10),
    };
  }
  if (head === "runs") {
    assertNoExtra(taken.pos, 1, taken.flags);
    rejectLimit(taken.flags);
    return { kind: "runs", audience: audience.value };
  }
  if (!isAction(head)) {
    return { kind: "usage", detail: `unknown command: ${head}` };
  }
  rejectLimit(taken.flags);
  if (head === "check" && taken.flags.has("--dry-run")) {
    throw new RcloneErr(2, "check does not take --dry-run");
  }
  const task = taken.pos[1];
  if (task === undefined || task === "") {
    return { kind: "usage" };
  }
  const path = taken.pos[2];
  if ((head === "get" || head === "download") && (path === undefined || path === "")) {
    return { kind: "usage", detail: `${head} requires a path` };
  }
  const dest = taken.pos[3];
  const extraAt = head === "get" || head === "download" ? 4 : 3;
  if (taken.pos.length > extraAt) {
    throw new RcloneErr(2, `unexpected argument: ${taken.pos[extraAt] ?? ""}`);
  }
  return {
    kind: "xfer",
    action: head,
    audience: audience.value,
    task,
    ...(path === undefined ? {} : { path }),
    ...((head === "get" || head === "download") && dest !== undefined ? { dest } : {}),
    dryRun: head !== "check" && taken.flags.has("--dry-run"),
    verbose: taken.flags.has("-v") || taken.flags.has("--verbose"),
  };
}

export function usageText(names: string[], detail?: string): string {
  const lines = [
    "usage:",
    "  rclone-cli push <task> [path] [--dry-run]",
    "  rclone-cli pull <task> [path] [--dry-run]",
    "  rclone-cli check <task> [path]",
    "  rclone-cli get <task> <path> [dest] [--dry-run] [-v]",
    "  rclone-cli download <task> <path> [dest] [--dry-run] [-v]",
    "  rclone-cli ls",
    "  rclone-cli ls <task> [path] [--limit n]",
    "  rclone-cli runs",
    "  rclone-cli completion fish",
    "",
    "no default task; a task name is required.",
    "",
    "tasks:",
  ];
  if (names.length === 0) {
    lines.push("  (none)");
  } else {
    for (const name of names) {
      lines.push(`  ${name}`);
    }
  }
  if (detail !== undefined && detail !== "") {
    lines.push("", detail);
  }
  return lines.join("\n");
}

export function helpText(): string {
  return [
    "usage:",
    "  rclone-cli push <task> [path] [--dry-run]",
    "  rclone-cli pull <task> [path] [--dry-run]",
    "  rclone-cli check <task> [path]",
    "  rclone-cli get <task> <path> [dest] [--dry-run] [-v]",
    "  rclone-cli download <task> <path> [dest] [--dry-run] [-v]",
    "  rclone-cli ls",
    "  rclone-cli ls <task> [path] [--limit n]",
    "  rclone-cli runs",
    "  rclone-cli completion fish",
    "",
    "no default task; a task name is required.",
    "ls with no task prints config. ls <task> lists files, default --limit 10.",
    "check lists both sides and compares size; leftover .bin counts as present.",
    "get writes one decrypted file into the source tree unless dest is set.",
    "download writes one file to the current directory unless dest is set.",
    "pull restores a directory into the source tree; a file path is download.",
    "-v / --verbose prints rclone logs and key-pair retries. default is quiet.",
  ].join("\n");
}

function parseComplete(argv: string[]): CliCmd {
  const tokens = [...argv];
  const current = tokens.length > 0 ? (tokens.pop() ?? "") : "";
  return { kind: "complete", tokens, current };
}

function takeCmd(argv: string[]): { flags: Map<string, string>; pos: string[] } {
  const flags = new Map<string, string>();
  const pos: string[] = [];
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index] ?? "";
    if (BOOL_FLAGS.has(token)) {
      flags.set(token, "true");
      continue;
    }
    if (token.startsWith("--limit=")) {
      flags.set("--limit", token.slice("--limit=".length));
      continue;
    }
    if (token === "--limit" || token === "-n") {
      const next = argv[index + 1];
      if (next === undefined || next.startsWith("-")) {
        throw new RcloneErr(2, `${token} requires a value`);
      }
      flags.set("--limit", next);
      index += 1;
      continue;
    }
    if (token.startsWith("-")) {
      throw new RcloneErr(2, `unknown flag: ${token}`);
    }
    pos.push(token);
  }
  return { flags, pos };
}

/** 为什么: ls 默认 10 条, 避免整库打到终端. 缺省和显式都走同一条正整数校验. */
function readLimit(flags: Map<string, string>, whenOmit: number): number {
  const raw = flags.get("--limit");
  if (raw === undefined) {
    return whenOmit;
  }
  if (!/^[1-9][0-9]*$/.test(raw)) {
    throw new RcloneErr(2, "--limit must be a positive integer");
  }
  return Number(raw);
}

function takeAudience(pos: string[]): { value: Audience } {
  const head = pos[0];
  if (head !== "agent" && head !== "human") {
    return { value: "human" };
  }
  pos.shift();
  return { value: head };
}

function assertNoExtra(pos: string[], count: number, flags: Map<string, string>): void {
  if (pos.length > count) {
    throw new RcloneErr(2, `unexpected argument: ${pos[count] ?? ""}`);
  }
  if (flags.has("--dry-run")) {
    throw new RcloneErr(2, "unknown flag: --dry-run");
  }
}

function rejectLimit(flags: Map<string, string>): void {
  if (flags.has("--limit")) {
    throw new RcloneErr(2, "unknown flag: --limit");
  }
}

function isAction(value: string): value is Action {
  return XFER.includes(value as Action);
}
