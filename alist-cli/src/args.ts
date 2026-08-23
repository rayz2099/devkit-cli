import type { Audience, CliCmd } from "./types";

const VALUE_FLAGS = new Set([
  "-p",
  "--profile",
  "--page",
  "--size",
  "--password",
  "--src",
  "--dst",
]);
const BOOL_FLAGS = new Set(["-h", "--help", "--refresh", "--as-task"]);

/** 为什么: 自己解析 argv, Audience 前缀和 -p 才能前后插, 不绑框架. */
export function parseArgs(argv: string[]): CliCmd {
  if (argv[0] === "completion") {
    if (argv[1] === "fish") {
      return { kind: "completion-fish" };
    }
    throw new Error("usage: alist-cli completion fish");
  }
  if (argv[0] === "__complete") {
    return parseComplete(argv.slice(1));
  }

  const { flags, pos } = takeCmd(argv);
  if (flags.has("-h") || flags.has("--help") || pos[0] === "help") {
    return { kind: "help", topic: pos[0] === "help" ? pos[1] : pos[0] };
  }

  let audience: Audience = "human";
  const rest = [...pos];
  if (rest[0] === "agent" || rest[0] === "human") {
    audience = rest[0];
    rest.shift();
  }
  const profile = firstNonEmpty(flags.get("-p"), flags.get("--profile"));
  const head = rest[0];
  if (head === undefined) {
    return { kind: "help" };
  }
  if (head === "ls") {
    return parseLs(rest.slice(1), flags, audience, profile);
  }
  if (head === "put") {
    return parsePut(rest.slice(1), flags, audience, profile);
  }
  if (head === "mkdir") {
    return parseMkdir(rest.slice(1), flags, audience, profile);
  }
  if (head === "sync") {
    return parseSync(rest.slice(1), flags, audience, profile);
  }
  throw new Error(`unknown command: ${head}`);
}

export function helpText(topic?: string): string {
  if (topic === "ls") {
    return `alist-cli ls

Usage:
  alist-cli ls [path] [--page N] [--size N] [--refresh] [--password P]
`;
  }
  if (topic === "put") {
    return `alist-cli put

Usage:
  alist-cli put <src> <dst> [--as-task] [--password P]
`;
  }
  if (topic === "mkdir") {
    return `alist-cli mkdir

Usage:
  alist-cli mkdir <path> [--password P]
`;
  }
  if (topic === "sync") {
    return `alist-cli sync

Usage:
  alist-cli sync <src> <dst> [--as-task] [--password P]
`;
  }
  if (topic === "agent") {
    return `alist-cli agent

Usage:
  alist-cli agent ls [path]
  alist-cli agent put <src> <dst>
  alist-cli agent mkdir <path>
  alist-cli agent sync <src> <dst>
`;
  }
  return `alist-cli

Usage:
  alist-cli [-p profile] [agent|human] <command>

Commands:
  ls [path]
  put <src> <dst>
  mkdir <path>
  sync <src> <dst>
  completion fish

Env:
  ALIST_ADDRESS
  ALIST_USERNAME
  ALIST_PASSWORD
`;
}

function parseLs(
  argv: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  return {
    kind: "ls",
    audience,
    profile,
    path: firstNonEmpty(argv[0], "/") ?? "/",
    password: flags.get("--password") ?? "",
    page: readInt(flags.get("--page"), 1),
    size: readInt(flags.get("--size"), 100),
    refresh: flags.has("--refresh"),
  };
}

function parsePut(
  argv: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  const src = need(
    firstNonEmpty(flags.get("--src"), argv[0]),
    "put: missing <src>",
  );
  const dst = need(
    firstNonEmpty(flags.get("--dst"), argv[1]),
    "put: missing <dst>",
  );
  return {
    kind: "put",
    audience,
    profile,
    src,
    dst,
    asTask: flags.has("--as-task"),
    password: flags.get("--password") ?? "",
  };
}

function parseMkdir(
  argv: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  return {
    kind: "mkdir",
    audience,
    profile,
    path: need(argv[0], "mkdir: missing <path>"),
    password: flags.get("--password") ?? "",
  };
}

function parseSync(
  argv: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  const src = need(
    firstNonEmpty(flags.get("--src"), argv[0]),
    "sync: missing <src>",
  );
  const dst = need(
    firstNonEmpty(flags.get("--dst"), argv[1]),
    "sync: missing <dst>",
  );
  return {
    kind: "sync",
    audience,
    profile,
    src,
    dst,
    asTask: flags.has("--as-task"),
    password: flags.get("--password") ?? "",
  };
}

function parseComplete(argv: string[]): CliCmd {
  const tokens = [...argv];
  const current = tokens.length > 0 ? (tokens.pop() ?? "") : "";
  return { kind: "complete", tokens, current };
}

export function takeCmd(argv: string[]): { flags: Map<string, string>; pos: string[] } {
  const flags = new Map<string, string>();
  const pos: string[] = [];
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (token === undefined) {
      continue;
    }
    if (VALUE_FLAGS.has(token)) {
      const value = argv[index + 1];
      if (value === undefined) {
        throw new Error(`${token} requires a value`);
      }
      flags.set(token, value);
      index += 1;
      continue;
    }
    if (BOOL_FLAGS.has(token)) {
      flags.set(token, "true");
      continue;
    }
    if (token.startsWith("-")) {
      throw new Error(`unknown flag: ${token}`);
    }
    pos.push(token);
  }
  return { flags, pos };
}

function readInt(raw: string | undefined, whenOmit: number): number {
  if (raw === undefined) {
    return whenOmit;
  }
  const value = Number.parseInt(raw, 10);
  if (!Number.isFinite(value) || value <= 0) {
    throw new Error(`invalid number: ${raw}`);
  }
  return value;
}

function firstNonEmpty(...values: Array<string | undefined>): string | undefined {
  for (const value of values) {
    if (value !== undefined && value.trim() !== "") {
      return value.trim();
    }
  }
  return undefined;
}

function need(value: string | undefined, message: string): string {
  if (value === undefined || value.trim() === "") {
    throw new Error(message);
  }
  return value;
}
