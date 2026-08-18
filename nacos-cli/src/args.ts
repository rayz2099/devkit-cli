import type { CliCmd, GlobalFlags, OutputFmt, SearchMode } from "./types";

const DEFAULT_CFG_GROUP = "COMMON";
const DEFAULT_NAMING_GROUP = "DEFAULT_GROUP";
const DEFAULT_CLUSTER = "DEFAULT";

/** 为什么: 自己解析 argv, 才能复刻 cobra persistent flags 的前后可插位置. */
export function parseArgs(argv: string[]): CliCmd {
  if (argv.length === 0 || argv[0] === "help" || argv[0] === "-h" || argv[0] === "--help") {
    return { kind: "help" };
  }

  if (argv[0] === "__complete") {
    return parseComplete(argv.slice(1), { dev: argv.includes("--dev") });
  }

  const { global, rest } = takeGlobals(argv);
  const head = rest[0];
  if (head === undefined || head === "help" || head === "-h" || head === "--help") {
    return { kind: "help" };
  }

  if (head === "completion") {
    if (rest[1] === "fish") {
      return { kind: "completion-fish" };
    }
    throw new Error("usage: nacos-cli completion fish");
  }

  if (head === "__fish_complete") {
    return parseFish(rest.slice(1), global);
  }

  if (head === "config") {
    return parseConfig(rest.slice(1), global);
  }
  if (head === "naming") {
    return parseNaming(rest.slice(1), global);
  }

  throw new Error(`unknown command: ${head}`);
}

export function helpText(topic?: string): string {
  if (topic === "config") {
    return `nacos-cli config

Usage:
  nacos-cli config get [data-id] [group]
  nacos-cli config put --data-id <id> --group <group> --content <text>
  nacos-cli config delete --data-id <id> --group <group>
  nacos-cli config list [--search blur|accurate] [--data-id <id>] [--group <group>]
`;
  }
  if (topic === "naming") {
    return `nacos-cli naming

Usage:
  nacos-cli naming register --service <name> --ip <ip> --port <port>
  nacos-cli naming deregister --service <name> --ip <ip> --port <port>
  nacos-cli naming instances --service <name> [--clusters a,b] [--healthy-only]
`;
  }
  return `nacos-cli

Usage:
  nacos-cli [global] config <get|put|delete|list>
  nacos-cli [global] naming <register|deregister|instances>
  nacos-cli completion fish

Global:
  --server-addr <addr>   nacos server address
  --username <name>      nacos username
  --password <pass>      nacos password
  --namespace <id>       nacos namespace
  -o, --output text|json output format
  --dev                  write HTTP debug log
`;
}

function parseComplete(argv: string[], global: GlobalFlags): CliCmd {
  let current: string | undefined;
  const tokens: string[] = [];
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (token === "--to-complete") {
      current = argv[index + 1] ?? "";
      index += 1;
      continue;
    }
    if (token !== undefined) {
      tokens.push(token);
    }
  }
  if (current === undefined) {
    current = tokens.length > 0 ? (tokens[tokens.length - 1] ?? "") : "";
    tokens.pop();
  }
  return { kind: "complete", tokens, current, global };
}

function parseFish(argv: string[], global: GlobalFlags): CliCmd {
  const kind = argv[0];
  const flags = takePairs(argv.slice(1));
  const prefix = flags.get("--prefix") ?? "";
  if (kind === "namespaces") {
    return { kind: "fish-namespaces", prefix };
  }
  if (kind === "config-data-ids") {
    return { kind: "fish-data-ids", prefix, global };
  }
  if (kind === "config-groups") {
    return {
      kind: "fish-groups",
      dataId: flags.get("--data-id") ?? "",
      prefix,
      global,
    };
  }
  throw new Error(`unknown fish completion: ${kind ?? ""}`);
}

function parseConfig(argv: string[], global: GlobalFlags): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help" || sub === "-h" || sub === "--help" || argv.includes("-h") || argv.includes("--help")) {
    return { kind: "help", topic: "config" };
  }

  if (sub === "get") {
    const { flags, positionals } = takeCmd(argv.slice(1));
    const dataId = firstNonEmpty(flags.get("--data-id"), positionals[0]);
    const group = firstNonEmpty(flags.get("--group"), positionals[1], DEFAULT_CFG_GROUP);
    if (dataId === "") {
      throw new Error("data-id is required");
    }
    if (group === "") {
      throw new Error("group is required");
    }
    return { kind: "config-get", dataId, group, global };
  }

  if (sub === "put") {
    const { flags } = takeCmd(argv.slice(1));
    const dataId = (flags.get("--data-id") ?? "").trim();
    const group = (flags.get("--group") ?? "").trim();
    const content = flags.get("--content");
    if (dataId === "") {
      throw new Error("data-id is required");
    }
    if (group === "") {
      throw new Error("group is required");
    }
    if (content === undefined) {
      throw new Error("content is required");
    }
    return { kind: "config-put", dataId, group, content, global };
  }

  if (sub === "delete") {
    const { flags } = takeCmd(argv.slice(1));
    const dataId = (flags.get("--data-id") ?? "").trim();
    const group = (flags.get("--group") ?? "").trim();
    if (dataId === "") {
      throw new Error("data-id is required");
    }
    if (group === "") {
      throw new Error("group is required");
    }
    return { kind: "config-delete", dataId, group, global };
  }

  if (sub === "list") {
    const { flags } = takeCmd(argv.slice(1));
    const search = (flags.get("--search") ?? "blur") as SearchMode;
    if (search !== "accurate" && search !== "blur") {
      throw new Error("search must be one of: accurate, blur");
    }
    return {
      kind: "config-list",
      search,
      dataId: flags.get("--data-id") ?? "",
      group: flags.get("--group") ?? "",
      pageNo: parseIntFlag(flags.get("--page-no"), 1, "page-no"),
      pageSize: parseIntFlag(flags.get("--page-size"), 10, "page-size"),
      global,
    };
  }

  throw new Error(`unknown config command: ${sub}`);
}

function parseNaming(argv: string[], global: GlobalFlags): CliCmd {
  const sub = argv[0];
  if (sub === undefined || sub === "help" || sub === "-h" || sub === "--help" || argv.includes("-h") || argv.includes("--help")) {
    return { kind: "help", topic: "naming" };
  }

  if (sub === "register") {
    const { flags } = takeCmd(argv.slice(1));
    return {
      kind: "naming-register",
      service: requireFlag(flags, "--service"),
      ip: requireFlag(flags, "--ip"),
      port: requirePort(flags.get("--port")),
      group: flags.get("--group") ?? DEFAULT_NAMING_GROUP,
      cluster: flags.get("--cluster") ?? DEFAULT_CLUSTER,
      weight: parseFloatFlag(flags.get("--weight"), 1, "weight"),
      ephemeral: parseBoolFlag(flags.get("--ephemeral"), true),
      global,
    };
  }

  if (sub === "deregister") {
    const { flags } = takeCmd(argv.slice(1));
    return {
      kind: "naming-deregister",
      service: requireFlag(flags, "--service"),
      ip: requireFlag(flags, "--ip"),
      port: requirePort(flags.get("--port")),
      group: flags.get("--group") ?? DEFAULT_NAMING_GROUP,
      cluster: flags.get("--cluster") ?? DEFAULT_CLUSTER,
      ephemeral: parseBoolFlag(flags.get("--ephemeral"), true),
      global,
    };
  }

  if (sub === "instances") {
    const { flags } = takeCmd(argv.slice(1));
    return {
      kind: "naming-instances",
      service: requireFlag(flags, "--service"),
      group: flags.get("--group") ?? DEFAULT_NAMING_GROUP,
      clusters: splitClusters(flags.get("--clusters") ?? ""),
      healthyOnly: parseBoolFlag(flags.get("--healthy-only"), true),
      global,
    };
  }

  throw new Error(`unknown naming command: ${sub}`);
}

function takeGlobals(argv: string[]): { global: GlobalFlags; rest: string[] } {
  const global: GlobalFlags = { dev: false };
  const rest: string[] = [];

  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (token === undefined) {
      continue;
    }

    if (token === "--dev") {
      global.dev = true;
      continue;
    }
    if (token === "-o" || token === "--output") {
      global.output = readOutput(readValue(argv, index, token));
      index += 1;
      continue;
    }
    if (token.startsWith("-o") && token.length > 2 && !token.startsWith("--")) {
      global.output = readOutput(token.slice(2));
      continue;
    }
    if (token === "--server-addr") {
      global.serverAddr = readValue(argv, index, token);
      index += 1;
      continue;
    }
    if (token === "--username") {
      global.username = readValue(argv, index, token);
      index += 1;
      continue;
    }
    if (token === "--password") {
      global.password = readValue(argv, index, token);
      index += 1;
      continue;
    }
    if (token === "--namespace") {
      global.namespace = readValue(argv, index, token);
      index += 1;
      continue;
    }

    rest.push(token);
  }

  return { global, rest };
}

function takeCmd(argv: string[]): { flags: Map<string, string>; positionals: string[] } {
  const flags = new Map<string, string>();
  const positionals: string[] = [];

  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (token === undefined) {
      continue;
    }
    if (token === "--") {
      positionals.push(...argv.slice(index + 1));
      break;
    }
    if (token.startsWith("--") && token.includes("=")) {
      const eq = token.indexOf("=");
      flags.set(token.slice(0, eq), token.slice(eq + 1));
      continue;
    }
    if (token.startsWith("--")) {
      const next = argv[index + 1];
      if (next === undefined || next.startsWith("-")) {
        flags.set(token, "true");
        continue;
      }
      flags.set(token, next);
      index += 1;
      continue;
    }
    positionals.push(token);
  }

  return { flags, positionals };
}

function takePairs(argv: string[]): Map<string, string> {
  return takeCmd(argv).flags;
}

function readValue(argv: string[], index: number, flag: string): string {
  const value = argv[index + 1];
  if (value === undefined) {
    throw new Error(`${flag} requires a value`);
  }
  return value;
}

function readOutput(value: string): OutputFmt {
  const output = value.toLowerCase();
  if (output !== "text" && output !== "json") {
    throw new Error(`invalid output: ${value}`);
  }
  return output;
}

function requireFlag(flags: Map<string, string>, name: string): string {
  const value = (flags.get(name) ?? "").trim();
  if (value === "") {
    throw new Error(`${name.slice(2)} is required`);
  }
  return value;
}

function requirePort(raw: string | undefined): number {
  if (raw === undefined || raw.trim() === "") {
    throw new Error("port is required");
  }
  const port = Number(raw);
  if (!Number.isInteger(port) || port <= 0) {
    throw new Error("port is required");
  }
  return port;
}

function parseIntFlag(raw: string | undefined, defaultValue: number, name: string): number {
  if (raw === undefined) {
    return defaultValue;
  }
  const value = Number(raw);
  if (!Number.isInteger(value) || value <= 0) {
    throw new Error(`invalid ${name}: ${raw}`);
  }
  return value;
}

function parseFloatFlag(raw: string | undefined, defaultValue: number, name: string): number {
  if (raw === undefined) {
    return defaultValue;
  }
  const value = Number(raw);
  if (!Number.isFinite(value)) {
    throw new Error(`invalid ${name}: ${raw}`);
  }
  return value;
}

function parseBoolFlag(raw: string | undefined, defaultValue: boolean): boolean {
  if (raw === undefined) {
    return defaultValue;
  }
  if (raw === "true") {
    return true;
  }
  if (raw === "false") {
    return false;
  }
  throw new Error(`invalid boolean: ${raw}`);
}

function firstNonEmpty(...values: Array<string | undefined>): string {
  for (const value of values) {
    if (value !== undefined && value.trim() !== "") {
      return value.trim();
    }
  }
  return "";
}

function splitClusters(raw: string): string[] {
  if (raw.trim() === "") {
    return [];
  }
  return raw
    .split(",")
    .map((item) => item.trim())
    .filter((item) => item !== "");
}
