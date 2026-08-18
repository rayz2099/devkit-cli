import type { Audience, CliCmd } from "./types";

const VALUE_FLAGS = new Set(["-p", "--profile", "--limit", "--tail"]);
const BOOL_FLAGS = new Set(["-h", "--help"]);

/** 为什么: 自己解析 argv, Audience 前缀和 -p 才能前后插, 不绑框架. */
export function parseArgs(argv: string[]): CliCmd {
  if (argv[0] === "completion") {
    if (argv[1] === "fish") {
      return { kind: "completion-fish" };
    }
    throw new Error("usage: jenkins-cli completion fish");
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

  if (audience === "agent") {
    const alias = parseAlias(head, rest.slice(1), flags, audience, profile);
    if (alias !== undefined) {
      return alias;
    }
  }

  if (head === "job") {
    return parseJob(rest.slice(1), flags, audience, profile);
  }
  if (head === "run") {
    return parseRun(rest.slice(1), flags, audience, profile);
  }
  if (head === "log") {
    return parseLog(rest.slice(1), flags, audience, profile);
  }
  if (head === "queue") {
    return parseQueue(rest.slice(1), audience, profile);
  }
  throw new Error(`unknown command: ${head}`);
}

export function helpText(topic?: string): string {
  if (topic === "job") {
    return `jenkins-cli job

Usage:
  jenkins-cli job ls [folder]
  jenkins-cli job view <jobPath>
`;
  }
  if (topic === "run") {
    return `jenkins-cli run

Usage:
  jenkins-cli run ls <jobPath> [--limit N]
  jenkins-cli run view <jobPath> <buildNo>
  jenkins-cli run start <jobPath>
  jenkins-cli run cancel <jobPath> <buildNo>
  jenkins-cli run rerun <jobPath> <buildNo>
`;
  }
  if (topic === "agent") {
    return `jenkins-cli agent

Usage:
  jenkins-cli agent status <jobPath> [buildNo]
  jenkins-cli agent list <jobPath> [--limit N]
  jenkins-cli agent info <jobPath> <buildNo>
  jenkins-cli agent log <jobPath> <buildNo> [--tail N]
  jenkins-cli agent trigger <jobPath>
`;
  }
  return `jenkins-cli

Usage:
  jenkins-cli [-p profile] [agent|human] <command>

Commands:
  job ls|view
  run ls|view|start|cancel|rerun
  log <jobPath> <buildNo> [--tail N]
  queue ls
  completion fish

Agent aliases:
  status list info log trigger
`;
}

function parseAlias(
  head: string,
  rest: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd | undefined {
  if (head === "status") {
    const jobPath = need(rest[0], "status: missing <jobPath>");
    return {
      kind: "run-view",
      audience,
      profile,
      jobPath,
      buildNo: rest[1] ?? "lastBuild",
      slim: true,
    };
  }
  if (head === "list") {
    const jobPath = need(rest[0], "list: missing <jobPath>");
    return {
      kind: "run-ls",
      audience,
      profile,
      jobPath,
      limit: readInt(flags.get("--limit"), 10),
    };
  }
  if (head === "info") {
    const jobPath = need(rest[0], "info: missing <jobPath>");
    const buildNo = need(rest[1], "info: missing <buildNo>");
    return {
      kind: "run-view",
      audience,
      profile,
      jobPath,
      buildNo,
      slim: false,
    };
  }
  if (head === "trigger") {
    const jobPath = need(rest[0], "trigger: missing <jobPath>");
    return { kind: "run-start", audience, profile, jobPath };
  }
  return undefined;
}

function parseJob(
  argv: string[],
  _flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help") {
    return { kind: "help", topic: "job" };
  }
  if (sub === "ls") {
    return {
      kind: "job-ls",
      audience,
      profile,
      folder: argv[1] ?? "",
    };
  }
  if (sub === "view") {
    return {
      kind: "job-view",
      audience,
      profile,
      jobPath: need(argv[1], "job view: missing <jobPath>"),
    };
  }
  throw new Error(`unknown command: job ${sub}`);
}

function parseRun(
  argv: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help") {
    return { kind: "help", topic: "run" };
  }
  if (sub === "ls") {
    return {
      kind: "run-ls",
      audience,
      profile,
      jobPath: need(argv[1], "run ls: missing <jobPath>"),
      limit: readInt(flags.get("--limit"), 10),
    };
  }
  if (sub === "view") {
    return {
      kind: "run-view",
      audience,
      profile,
      jobPath: need(argv[1], "run view: missing <jobPath>"),
      buildNo: need(argv[2], "run view: missing <buildNo>"),
      slim: false,
    };
  }
  if (sub === "start") {
    return {
      kind: "run-start",
      audience,
      profile,
      jobPath: need(argv[1], "run start: missing <jobPath>"),
    };
  }
  if (sub === "cancel") {
    return {
      kind: "run-cancel",
      audience,
      profile,
      jobPath: need(argv[1], "run cancel: missing <jobPath>"),
      buildNo: need(argv[2], "run cancel: missing <buildNo>"),
    };
  }
  if (sub === "rerun") {
    return {
      kind: "run-rerun",
      audience,
      profile,
      jobPath: need(argv[1], "run rerun: missing <jobPath>"),
      buildNo: need(argv[2], "run rerun: missing <buildNo>"),
    };
  }
  throw new Error(`unknown command: run ${sub}`);
}

function parseLog(
  argv: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  const jobPath = need(argv[0], "log: missing <jobPath>");
  const buildNo = need(argv[1], "log: missing <buildNo>");
  const tailRaw = flags.get("--tail");
  return {
    kind: "log",
    audience,
    profile,
    jobPath,
    buildNo,
    tail: tailRaw === undefined ? undefined : readInt(tailRaw, 0),
  };
}

function parseQueue(
  argv: string[],
  audience: Audience,
  profile?: string,
): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help") {
    return { kind: "help", topic: "queue" };
  }
  if (sub === "ls") {
    return { kind: "queue-ls", audience, profile };
  }
  throw new Error(`unknown command: queue ${sub}`);
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

function readInt(raw: string | undefined, fallback: number): number {
  if (raw === undefined) {
    return fallback;
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
      return value;
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
