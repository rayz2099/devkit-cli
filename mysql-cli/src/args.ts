import type { CliCommand, OutputFormat } from "./types";

const outputFormats = new Set(["json", "csv"]);

// 参数白名单让工具行为稳定, 避免 mysql 原生命令语义泄漏进包装器.
export function parseCliArgs(argv: string[]): CliCommand {
  if (argv.length === 0 || argv[0] === "help" || argv[0] === "--help" || argv[0] === "-h") {
    return { kind: "help" };
  }

  if (argv[0] === "__fish_complete_profiles") {
    return { kind: "completion" };
  }

  let profile: string | undefined;
  let execute: string | undefined;
  let output: OutputFormat | undefined;

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];

    if (arg === "-p") {
      profile = readValue(argv, index, "-p");
      index += 1;
      continue;
    }

    if (arg === "-e") {
      execute = readValue(argv, index, "-e");
      index += 1;
      continue;
    }

    if (arg === "--output") {
      const value = readValue(argv, index, "--output");
      if (!outputFormats.has(value)) {
        throw new Error(`Unsupported --output: ${value}. Expected json or csv`);
      }
      output = value as OutputFormat;
      index += 1;
      continue;
    }

    throw new Error(`Unsupported argument: ${arg}`);
  }

  if (!profile) {
    throw new Error("Missing required -p <profile>");
  }

  if (output && !execute) {
    throw new Error("--output requires -e");
  }

  return {
    kind: "run",
    profile,
    execute,
    output,
  };
}

function readValue(argv: string[], index: number, flag: string): string {
  const value = argv[index + 1];
  if (!value) {
    throw new Error(`${flag} requires a value`);
  }
  return value;
}

export function helpText(): string {
  return `mysql-cli

Usage:
  mysql-cli -p <profile>
  mysql-cli -p <profile> -e <sql>
  mysql-cli -p <profile> -e <sql> --output json|csv
  mysql-cli help

Options:
  -p <profile>       Profile name from ~/.config/mysql-cli/config.json
  -e <sql>           Execute SQL and exit
  --output json|csv  Format -e result
`;
}
