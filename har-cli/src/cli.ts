import { renderFishCompletion } from "./core/completions";
import { evaluateWhere } from "./core/expression";
import { formatEntries } from "./core/formatter";
import { readHarEntries } from "./core/har";
import { renderHelp } from "./core/help";
import type { CliResult, OutputFormat, ScanOptions } from "./core/types";

/**
 * 执行 CLI 主逻辑，测试和真实命令入口都复用该函数。
 */
export async function executeCli(argv: string[]): Promise<CliResult> {
  try {
    const parsed = parseArgs(argv);

    if (parsed.kind === "help") {
      return ok(renderHelp());
    }

    if (parsed.kind === "completions") {
      if (parsed.shell !== "fish") {
        return fail("Only fish completion is supported.\n");
      }

      return ok(`${renderFishCompletion()}\n`);
    }

    const entries = await readHarEntries(parsed.options.files);
    // 未指定 -f 时保留完整 HAR 结果，避免字段投影场景被权限白名单经验条件隐式裁剪。
    const matched = parsed.options.filter === undefined ? entries : entries.filter((entry) => evaluateWhere(parsed.options.filter ?? "", {
      body: entry.body,
      http: {
        method: entry.method,
        status: entry.httpStatus,
      },
      request: {
        path: entry.path,
        url: entry.url,
      },
      response: {
        mimeType: entry.mimeType,
      },
      uri: {
        path: entry.path,
        url: entry.url,
      },
    }));

    return ok(formatEntries(
      matched,
      parsed.options.select,
      parsed.options.format,
      parsed.options.dedupe,
      parsed.options.prefixDepth,
    ));
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return fail(`${message}\n`);
  }
}

/**
 * 解析单级命令参数，保留 help 与 completions 作为辅助入口。
 */
function parseArgs(argv: string[]): { kind: "help" } | { kind: "completions"; shell: string } | { kind: "scan"; options: ScanOptions } {
  if (argv.length === 0 || argv[0] === "help" || argv.includes("--help") || argv.includes("-h")) {
    return { kind: "help" };
  }

  if (argv[0] === "completions") {
    return {
      kind: "completions",
      shell: argv[1] ?? "",
    };
  }

  const options: ScanOptions = {
    files: [],
    select: "http",
    prefixDepth: 2,
    format: "text",
    dedupe: true,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const current = argv[index];

    if (!current.startsWith("-")) {
      options.files.push(current);
      continue;
    }

    switch (current) {
      case "--filter":
      case "-f":
        options.filter = requireValue(argv, ++index, current);
        break;
      case "--select":
      case "-s":
        options.select = requireValue(argv, ++index, current);
        break;
      case "--prefix-depth":
      case "-p":
        options.prefixDepth = parsePositiveInteger(requireValue(argv, ++index, current), current);
        break;
      case "--format":
      case "-o":
        options.format = parseOutputFormat(requireValue(argv, ++index, current));
        break;
      case "--no-dedupe":
        options.dedupe = false;
        break;
      default:
        throw new Error(`Unknown option: ${current}`);
    }
  }

  if (options.files.length === 0) {
    throw new Error("At least one HAR file is required.");
  }

  return { kind: "scan", options };
}

function requireValue(argv: string[], index: number, option: string): string {
  const value = argv[index];
  if (value === undefined || (value.startsWith("-") && value !== "")) {
    throw new Error(`Missing value for ${option}`);
  }
  return value;
}

function parseOutputFormat(value: string): OutputFormat {
  if (value === "text" || value === "json") {
    return value;
  }

  throw new Error(`Unsupported format: ${value}`);
}

function parsePositiveInteger(value: string, option: string): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) {
    throw new Error(`Invalid value for ${option}: ${value}`);
  }
  return parsed;
}

function ok(stdout: string): CliResult {
  return {
    stdout,
    stderr: "",
    exitCode: 0,
  };
}

function fail(stderr: string): CliResult {
  return {
    stdout: "",
    stderr,
    exitCode: 1,
  };
}

if (import.meta.main) {
  const result = await executeCli(process.argv.slice(2));
  if (result.stdout) {
    process.stdout.write(result.stdout);
  }
  if (result.stderr) {
    process.stderr.write(result.stderr);
  }
  process.exit(result.exitCode);
}
