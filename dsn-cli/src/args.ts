import type { Audience, CliCmd, OutputFmt } from "./types";

const VALUE_FLAGS = new Set([
  "-p",
  "--profile",
  "--output",
  "--limit",
  "--timeout",
  "--connect-timeout",
  "-c",
  "--config",
]);
const BOOL_FLAGS = new Set(["-h", "--help", "--pretty"]);

/** 为什么: 自己解析 argv, Audience 前缀和 -p 才能前后插, 不绑框架. */
export function parseArgs(argv: string[]): CliCmd {
  if (argv[0] === "completion") {
    if (argv[1] === "fish") {
      return { kind: "completion-fish" };
    }
    throw new Error("usage: dsn-cli completion fish");
  }
  if (argv[0] === "__complete") {
    return parseComplete(argv.slice(1));
  }

  const { flags, pos } = takeCmd(argv);
  return attachCfg(parseCmd(flags, pos), flags);
}

function parseCmd(flags: Map<string, string>, pos: string[]): CliCmd {
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
    if (profile === undefined) {
      return { kind: "help" };
    }
    return { kind: "console", audience, profile };
  }

  if (head === "query") {
    return parseQuery(rest.slice(1), flags, audience, profile);
  }
  if (head === "doctor") {
    return parseDoctor(rest.slice(1), flags, audience, profile);
  }
  if (head === "completion") {
    throw new Error("usage: dsn-cli completion fish");
  }
  throw new Error(`unknown command: ${head}`);
}

function parseQuery(
  rest: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile: string | undefined,
): CliCmd {
  if (profile === undefined || profile.trim() === "") {
    throw new Error("-p is required");
  }
  const stmt = rest.join(" ").trim();
  if (stmt === "") {
    throw new Error("query: missing statement");
  }
  const output = readOutput(flags.get("--output"));
  const pretty = flags.has("--pretty");
  assertPretty(output, pretty);
  return {
    kind: "query",
    audience,
    profile,
    stmt,
    output,
    pretty,
    limit: readLimit(flags.get("--limit")),
    connectSec: readSec(flags.get("--connect-timeout"), 3),
    execSec: readSec(flags.get("--timeout"), 30),
  };
}

function parseDoctor(
  rest: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile: string | undefined,
): CliCmd {
  if (rest.length > 0) {
    throw new Error("doctor: unexpected argument");
  }
  if (profile !== undefined && profile.trim() === "") {
    throw new Error("-p is required");
  }
  const output = readOutput(flags.get("--output"));
  const pretty = flags.has("--pretty");
  assertPretty(output, pretty);
  return {
    kind: "doctor",
    audience,
    profile,
    output,
    pretty,
    connectSec: readSec(flags.get("--connect-timeout"), 3),
    execSec: readSec(flags.get("--timeout"), 5),
  };
}

function parseComplete(argv: string[]): CliCmd {
  if (argv.length === 0) {
    return { kind: "complete", tokens: [], current: "" };
  }
  const current = argv[argv.length - 1] ?? "";
  return { kind: "complete", tokens: argv.slice(0, -1), current };
}

export function helpText(topic?: string): string {
  if (topic === "query") {
    return `dsn-cli query

Usage:
  dsn-cli -p <profile> query '<stmt>' [--output json|csv|plain] [--pretty] [--limit N] [--timeout S]
  dsn-cli -p <profile> query peek <topic> [n]
  dsn-cli agent -p <profile> query '<stmt>' [--limit N] [--timeout S]
`;
  }
  if (topic === "doctor") {
    return `dsn-cli doctor

Usage:
  dsn-cli doctor [-p <profile>] [--output json|csv|plain] [--pretty] [--timeout S] [--connect-timeout S]
  dsn-cli agent doctor [-p <profile>] [--timeout S]

Probes Profiles with concurrency 4. Deadline is --connect-timeout + --timeout (defaults 3s + 5s).
`;
  }
  return `dsn-cli

Usage:
  dsn-cli [-c config.json] -p <profile>
  dsn-cli [-c config.json] -p <profile> query '<stmt>' [--output json|csv|plain] [--pretty] [--limit N] [--timeout S]
  dsn-cli [-c config.json] -p <profile> query peek <topic> [n]
  dsn-cli [-c config.json] agent -p <profile> query '<stmt>' [--limit N] [--timeout S]
  dsn-cli [-c config.json] doctor [-p <profile>] [--timeout S] [--connect-timeout S]
  dsn-cli [-c config.json] agent doctor [-p <profile>] [--timeout S]
  dsn-cli completion fish

TTY -p opens the vendor client (mysql / psql / redis-cli / mongosh).
agent and non-TTY cannot enter that Console.
doctor probes with concurrency 4; a hung Profile does not stall the rest.
`;
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

function assertPretty(output: OutputFmt, pretty: boolean): void {
  if (pretty && output !== "json") {
    throw new Error("--pretty requires --output json");
  }
}

function readOutput(raw: string | undefined): OutputFmt {
  if (raw === undefined) {
    return "table";
  }
  if (raw === "json" || raw === "csv" || raw === "plain" || raw === "table") {
    return raw;
  }
  throw new Error("invalid --output: json|csv|plain");
}

function readSec(raw: string | undefined, defSec: number): number {
  if (raw === undefined) {
    return defSec;
  }
  const value = Number.parseFloat(raw);
  if (!Number.isFinite(value) || value <= 0) {
    throw new Error(`invalid timeout: ${raw}`);
  }
  return value;
}

function readLimit(raw: string | undefined): number | undefined {
  if (raw === undefined) {
    return undefined;
  }
  const value = Number.parseInt(raw, 10);
  if (!Number.isFinite(value) || value < 0) {
    throw new Error(`invalid --limit: ${raw}`);
  }
  return value;
}

/** 为什么: -c 指定配置文件, 必须从 argv 抽出再交给 loadFileCfg, 不能写死 XDG. */
export function cfgFlag(flags: Map<string, string>): string | undefined {
  return firstNonEmpty(flags.get("-c"), flags.get("--config"));
}

function attachCfg(cmd: CliCmd, flags: Map<string, string>): CliCmd {
  const config = cfgFlag(flags);
  if (config === undefined) {
    return cmd;
  }
  return { ...cmd, config };
}

function firstNonEmpty(...values: Array<string | undefined>): string | undefined {
  for (const value of values) {
    if (value !== undefined && value.trim() !== "") {
      return value;
    }
  }
  return undefined;
}
