import type { Audience, CliCmd } from "./types";
import { asInt } from "./util";

const VALUE_FLAGS = new Set([
  "-c",
  "--config",
  "-p",
  "--profile",
  "--project",
  "--event",
  "--id",
  "--analysis",
  "--quota",
  "--group",
  "--qp",
  "--recent",
  "--from",
  "--to",
  "--particle",
  "--model",
  "--timeout",
]);

const BOOL_FLAGS = new Set(["-h", "--help"]);

const WRITE_WORDS = new Set([
  "save",
  "delete",
  "del",
  "update",
  "create",
  "edit",
  "import",
  "upload",
  "download",
  "remove",
]);

/** 为什么: Audience 前缀和 -p 要能前后插, 不绑命令框架. */
export function parseArgs(argv: string[]): CliCmd {
  if (argv[0] === "completion") {
    if (argv[1] === "fish") {
      return { kind: "completion-fish" };
    }
    throw new Error("usage: td-cli completion fish");
  }
  if (argv[0] === "__complete") {
    return parseComplete(argv.slice(1));
  }
  const { flags, pos } = takeCmd(argv);
  return attachCfg(parseCmd(flags, pos), flags);
}

export function helpText(topic?: string): string {
  if (topic === "auth") {
    return `td-cli auth

Usage:
  td-cli auth login
  td-cli auth status

账号在 profile 的 url、username、password 里. login 只验证, 不把令牌写回配置.
`;
  }
  if (topic === "query") {
    return `td-cli query

Usage:
  td-cli query event --project ID [--event NAME] [--analysis A100] [--recent 1-7] [--from TIME --to TIME] [--particle T1] [--group COL] [--qp FILE]
  td-cli query property --project ID [--analysis A101] [--quota PROP] [--group COL] [--qp FILE]
  td-cli query run --project ID --model N --qp FILE
  td-cli query report --project ID --id REPORT [--model N]

Models: 0 event, 1 retention, 2 funnel, 3 property, 4 trace, 7 scatter, 12 interval, 14 attribution
--group 用 name, event:name 或 user:name
--qp 给出页面 QP JSON, 此时不能再带其它查询条件
`;
  }
  return `td-cli

Usage:
  td-cli [-c config.json] [-p profile] [agent|human] <command>

Commands:
  auth login|status
  project ls
  meta events|props --project ID [--event NAME]
  report ls|get --project ID [--id REPORT]
  query event|property|run|report
  completion fish

只读. 每次用 profile 的 username 和 password 登录, 不走设备码, 不保存令牌.
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
      if (value === undefined || value.startsWith("-")) {
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

function parseCmd(flags: Map<string, string>, pos: string[]): CliCmd {
  if (flags.has("-h") || flags.has("--help") || pos[0] === "help") {
    return { kind: "help", audience: "human", topic: pos[0] === "help" ? pos[1] : undefined };
  }
  let audience: Audience = "human";
  const rest = [...pos];
  if (rest[0] === "agent" || rest[0] === "human") {
    audience = rest[0];
    rest.shift();
  }
  const profile = flag(flags, "-p", "--profile");
  const head = rest[0];
  if (head === undefined) {
    return { kind: "help", audience };
  }
  if (WRITE_WORDS.has(head)) {
    throw new Error("read-only cli, write is disabled");
  }
  const scope = { audience, profile };
  if (head === "auth") {
    return parseAuth(rest.slice(1), scope);
  }
  if (head === "project") {
    return parseProject(rest.slice(1), scope);
  }
  if (head === "meta") {
    return parseMeta(rest.slice(1), flags, scope);
  }
  if (head === "report") {
    return parseReport(rest.slice(1), flags, scope);
  }
  if (head === "query") {
    return parseQuery(rest.slice(1), flags, scope);
  }
  throw new Error(`unknown command: ${head}`);
}

function parseAuth(
  argv: string[],
  scope: { audience: Audience; profile?: string },
): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help") {
    return { kind: "help", audience: scope.audience, topic: "auth" };
  }
  if (WRITE_WORDS.has(sub)) {
    throw new Error("read-only cli, write is disabled");
  }
  if (sub === "login") {
    return { kind: "auth-login", ...scope };
  }
  if (sub === "status") {
    return { kind: "auth-status", ...scope };
  }
  throw new Error(`unknown command: auth ${sub}`);
}

function parseProject(
  argv: string[],
  scope: { audience: Audience; profile?: string },
): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help") {
    return { kind: "help", audience: scope.audience };
  }
  if (WRITE_WORDS.has(sub)) {
    throw new Error("read-only cli, write is disabled");
  }
  if (sub !== "ls") {
    throw new Error(`unknown command: project ${sub}`);
  }
  return { kind: "project-ls", ...scope };
}

function parseMeta(
  argv: string[],
  flags: Map<string, string>,
  scope: { audience: Audience; profile?: string },
): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help") {
    return { kind: "help", audience: scope.audience };
  }
  if (WRITE_WORDS.has(sub)) {
    throw new Error("read-only cli, write is disabled");
  }
  const project = need(flag(flags, "--project"), "missing --project");
  if (sub === "events") {
    return { kind: "meta-events", ...scope, project };
  }
  if (sub === "props") {
    return { kind: "meta-props", ...scope, project, event: flag(flags, "--event") };
  }
  throw new Error(`unknown command: meta ${sub}`);
}

function parseReport(
  argv: string[],
  flags: Map<string, string>,
  scope: { audience: Audience; profile?: string },
): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help") {
    return { kind: "help", audience: scope.audience };
  }
  if (WRITE_WORDS.has(sub)) {
    throw new Error("read-only cli, write is disabled");
  }
  const project = need(flag(flags, "--project"), "missing --project");
  if (sub === "ls") {
    return { kind: "report-ls", ...scope, project };
  }
  if (sub === "get") {
    return {
      kind: "report-get",
      ...scope,
      project,
      reportId: need(flag(flags, "--id"), "missing --id"),
    };
  }
  throw new Error(`unknown command: report ${sub}`);
}

function parseQuery(
  argv: string[],
  flags: Map<string, string>,
  scope: { audience: Audience; profile?: string },
): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help") {
    return { kind: "help", audience: scope.audience, topic: "query" };
  }
  if (WRITE_WORDS.has(sub)) {
    throw new Error("read-only cli, write is disabled");
  }
  const project = need(flag(flags, "--project"), "missing --project");
  const timeoutMs = readTimeout(flag(flags, "--timeout"));
  const qp = flag(flags, "--qp");
  if (sub === "event") {
    rejectQpMix(flags, qp);
    return {
      kind: "query-event",
      ...scope,
      project,
      eventName: flag(flags, "--event") ?? "anyEvent",
      analysis: flag(flags, "--analysis") ?? "A100",
      recent: flag(flags, "--recent"),
      from: flag(flags, "--from"),
      to: flag(flags, "--to"),
      particle: flag(flags, "--particle") ?? "T1",
      group: flag(flags, "--group"),
      qp,
      timeoutMs,
    };
  }
  if (sub === "property") {
    rejectQpMix(flags, qp);
    return {
      kind: "query-property",
      ...scope,
      project,
      analysis: flag(flags, "--analysis") ?? "A101",
      quota: flag(flags, "--quota") ?? "",
      group: flag(flags, "--group"),
      recent: flag(flags, "--recent"),
      from: flag(flags, "--from"),
      to: flag(flags, "--to"),
      qp,
      timeoutMs,
    };
  }
  if (sub === "run") {
    return {
      kind: "query-run",
      ...scope,
      project,
      model: asInt(need(flag(flags, "--model"), "missing --model"), "--model"),
      qp: need(qp, "missing --qp"),
      timeoutMs,
    };
  }
  if (sub === "report") {
    const modelRaw = flag(flags, "--model");
    return {
      kind: "query-report",
      ...scope,
      project,
      reportId: need(flag(flags, "--id"), "missing --id"),
      model: modelRaw === undefined ? undefined : asInt(modelRaw, "--model"),
      timeoutMs,
    };
  }
  throw new Error(`unknown command: query ${sub}`);
}

function rejectQpMix(flags: Map<string, string>, qp: string | undefined): void {
  if (qp === undefined) {
    return;
  }
  const mixed = ["--event", "--analysis", "--quota", "--group", "--recent", "--from", "--to", "--particle"];
  for (const name of mixed) {
    if (flags.has(name)) {
      throw new Error("--qp replaces other query flags");
    }
  }
}

function readTimeout(raw: string | undefined): number {
  if (raw === undefined) {
    return 120_000;
  }
  const sec = asInt(raw, "--timeout");
  if (sec <= 0) {
    throw new Error("--timeout must be > 0");
  }
  return sec * 1000;
}

function attachCfg(cmd: CliCmd, flags: Map<string, string>): CliCmd {
  const config = flag(flags, "-c", "--config");
  if (config === undefined || cmd.kind === "completion-fish" || cmd.kind === "complete") {
    return cmd;
  }
  return { ...cmd, config };
}

function parseComplete(argv: string[]): CliCmd {
  const tokens = [...argv];
  const current = tokens.length > 0 ? (tokens.pop() ?? "") : "";
  return { kind: "complete", tokens, current };
}

function flag(flags: Map<string, string>, ...names: string[]): string | undefined {
  for (const name of names) {
    const value = flags.get(name);
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
