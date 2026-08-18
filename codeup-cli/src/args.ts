import type { Audience, CliCmd } from "./types";

const VALUE_FLAGS = new Set([
  "-p",
  "--profile",
  "--repo",
  "--search",
  "--page",
  "--per-page",
  "--state",
  "--source",
  "--target",
  "--title",
  "--body",
  "--body-file",
  "--remote",
]);
const BOOL_FLAGS = new Set(["-h", "--help", "--show-secrets"]);

/** 为什么: 自己解析 argv, Audience 前缀和 -p 才能前后插, 不绑框架. */
export function parseArgs(argv: string[]): CliCmd {
  if (argv[0] === "completion") {
    if (argv[1] === "fish") {
      return { kind: "completion-fish" };
    }
    throw new Error("usage: codeup-cli completion fish");
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
  if (head === "init") {
    return { kind: "init", audience, profile };
  }
  if (head === "repos") {
    return parseRepos(flags, audience, profile);
  }
  if (head === "push") {
    return parsePush(rest.slice(1), flags, audience, profile);
  }
  if (head === "cr") {
    return parseCr(rest.slice(1), flags, audience, profile);
  }
  if (head === "webhook") {
    return parseWebhook(rest.slice(1), flags, audience, profile);
  }
  throw new Error(`unknown command: ${head}`);
}

export function helpText(topic?: string): string {
  if (topic === "cr") {
    return `codeup-cli cr

Usage:
  codeup-cli cr list [--repo group/project] [--state opened|merged|closed|all]
  codeup-cli cr get <localId> [--repo group/project]
  codeup-cli cr create --source <branch> --title <title> [--target <branch>]
`;
  }
  if (topic === "webhook") {
    return `codeup-cli webhook

Usage:
  codeup-cli webhook list [--repo group/project] [--show-secrets]
`;
  }
  if (topic === "agent") {
    return `codeup-cli agent

Usage:
  codeup-cli agent init
  codeup-cli agent repos
  codeup-cli agent push
  codeup-cli agent cr list|get|create
  codeup-cli agent webhook list
`;
  }
  return `codeup-cli

Usage:
  codeup-cli [-p profile] [agent|human] <command>

Commands:
  init
  repos [--search S]
  push [--remote origin] [branch]
  cr list|get|create
  webhook list
  completion fish
`;
}

function parseRepos(
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  return {
    kind: "repos",
    audience,
    profile,
    search: flags.get("--search"),
    page: readInt(flags.get("--page"), 1),
    perPage: readInt(flags.get("--per-page"), 50),
  };
}

function parsePush(
  argv: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  return {
    kind: "push",
    audience,
    profile,
    remote: flags.get("--remote") ?? "origin",
    branch: argv[0],
  };
}

function parseCr(
  argv: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help") {
    return { kind: "help", topic: "cr" };
  }
  if (sub === "list") {
    return {
      kind: "cr-list",
      audience,
      profile,
      repo: firstNonEmpty(flags.get("--repo"), argv[1]),
      state: flags.get("--state") ?? "opened",
      source: flags.get("--source"),
      target: flags.get("--target"),
      search: flags.get("--search"),
      page: readInt(flags.get("--page"), 1),
      perPage: readInt(flags.get("--per-page"), 20),
    };
  }
  if (sub === "get") {
    const localId = need(argv[1], "cr get: missing <localId>");
    return {
      kind: "cr-get",
      audience,
      profile,
      repo: flags.get("--repo"),
      localId,
    };
  }
  if (sub === "create") {
    return {
      kind: "cr-create",
      audience,
      profile,
      repo: flags.get("--repo"),
      source: need(flags.get("--source"), "cr create: missing --source"),
      target: flags.get("--target"),
      title: need(flags.get("--title"), "cr create: missing --title"),
      body: flags.get("--body"),
      bodyFile: flags.get("--body-file"),
    };
  }
  throw new Error(`unknown command: cr ${sub}`);
}

function parseWebhook(
  argv: string[],
  flags: Map<string, string>,
  audience: Audience,
  profile?: string,
): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help") {
    return { kind: "help", topic: "webhook" };
  }
  if (sub === "list") {
    return {
      kind: "webhook-list",
      audience,
      profile,
      repo: firstNonEmpty(flags.get("--repo"), argv[1]),
      showSecrets: flags.has("--show-secrets"),
      page: readInt(flags.get("--page"), 1),
      perPage: readInt(flags.get("--per-page"), 50),
    };
  }
  throw new Error(`unknown command: webhook ${sub}`);
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
