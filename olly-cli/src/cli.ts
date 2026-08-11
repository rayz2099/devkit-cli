#!/usr/bin/env bun
import { createUptraceClient } from "./uptrace/api";
import { helpTextFor, shouldShowHelp } from "./help";
import { createPrometheusClient } from "./prometheus/api";
import { createGraylogClient } from "./graylog/api";
import { formatGraylogAggregate, formatGraylogFields, formatGraylogResult } from "./graylog/format";
import { toGraylogSearchInput } from "./graylog/query";
import {
  buildBuildInfoRequest,
  buildHealthyRequest,
  buildLabelValuesRequest,
  buildLabelsRequest,
  buildQueryRangeRequest,
  buildQueryRequest,
  buildReadyRequest,
  buildRuntimeInfoRequest,
  buildSeriesRequest,
} from "./prometheus/query";
import { formatPrometheusResult } from "./prometheus/format";
import { normalizePrometheusConfig } from "./prometheus/config";
import { loadConfig, loadConfigIfExists, requireGraylogConfig, requireUptraceConfig } from "./uptrace/config";
import { buildContext, formatContextAgent, formatContextHuman } from "./uptrace/context";
import { formatGroups, formatJson } from "./uptrace/format";
import { parseTraceInput } from "./uptrace/trace-id";
import type { PrometheusEnvelope, PrometheusMetadataInput, PrometheusQueryInput, PrometheusQueryRangeInput } from "./prometheus/types";
import type { GroupsQueryInput, OutputMode, SpansQueryInput, UptraceConfig } from "./uptrace/types";

interface ParsedArgs {
  configPath?: string | undefined;
  output: OutputMode;
  command: string[];
  flags: Record<string, string | boolean>;
  positionals: string[];
}

interface PrometheusParsedArgs {
  configPath?: string | undefined;
  output: OutputMode;
  command: string[];
  flags: Record<string, string | boolean>;
  multiFlags: Record<string, string[]>;
  positionals: string[];
}

/** 为什么：入口只负责参数编排，Uptrace 能力放在独立模块里便于测试和复用。 */
export async function main(argv: string[] = Bun.argv.slice(2)): Promise<void> {
  if (shouldShowHelp(argv)) {
    process.stdout.write(helpTextFor(argv));
    return;
  }

  const prometheusArgs = extractPrometheusArgs(argv);
  if (prometheusArgs) {
    await runPrometheus(prometheusArgs, argv);
    return;
  }

  const args = parseArgs(argv);
  if (args.command[0] === "logs" || args.command[0] === "graylog") {
    await runGraylog(args);
    return;
  }

  if (args.command[0] !== "uptrace") {
    throw new Error("usage: olly-cli [-f config.json] [--output human|agent|plain] {uptrace|logs|prometheus} <command>. Run olly-cli --help for details.");
  }

  const appConfig = await loadConfig(args.configPath);
  const uptraceConfig = requireUptraceConfig(appConfig);
  const subcommand = args.command[1];
  const client = createUptraceClient(uptraceConfig);

  if (subcommand === "groups") {
    const body = await client.listGroups(toGroupsInput(args, uptraceConfig));
    process.stdout.write(formatGroups(body, args.output));
    return;
  }

  if (subcommand === "group-stats") {
    const body = await client.listGroupStats(toGroupsInput(args, uptraceConfig));
    process.stdout.write(formatGroups(body, args.output));
    return;
  }

  if (subcommand === "spans") {
    const body = await client.listSpans(toSpansInput(args));
    process.stdout.write(args.output === "plain" ? formatJson({ spans: body }) : formatJson({ spans: body }));
    return;
  }

  if (subcommand === "trace" || subcommand === "context") {
    const traceInput = firstTraceInput(args);
    const parsed = parseTraceInput(traceInput);
    const config = withProject(uptraceConfig, parsed.projectId);
    const traceClient = createUptraceClient(config);
    const spans = await traceClient.getTrace(parsed.traceId);
    if (args.output === "plain") {
      process.stdout.write(formatJson({ spans }));
      return;
    }
    const context = buildContext({
      config,
      traceInput,
      spans,
      query: {
        service: stringFlag(args, "service"),
        env: stringFlag(args, "env") ?? config.defaultEnv,
        uri: stringFlag(args, "uri"),
      },
    });
    process.stdout.write(args.output === "human" && subcommand === "trace" ? formatContextHuman(context) : formatContextAgent(context));
    return;
  }

  if (subcommand === "diagnose") {
    await runDiagnose(uptraceConfig, args);
    return;
  }

  throw new Error(`unknown uptrace command: ${subcommand ?? "<missing>"}`);
}

async function runGraylog(args: ParsedArgs): Promise<void> {
  const appConfig = await loadConfig(args.configPath);
  const config = requireGraylogConfig(appConfig);
  const client = createGraylogClient(config);
  if (args.command[1] === "aggregate" || args.command[1] === "agg") {
    const input = toGraylogSearchInput({
      source: firstAggregateQueryInput(args),
      relative: numberFlag(args, "relative"),
      from: stringFlag(args, "from"),
      to: stringFlag(args, "to"),
      keyword: stringFlag(args, "keyword"),
      limit: numberFlag(args, "limit"),
    });
    const field = stringFlag(args, "field");
    if (!field) {
      throw new Error("logs aggregate requires --field");
    }
    const body = await client.aggregate({
      query: input.query,
      rangeType: input.rangeType,
      relative: input.relative,
      from: input.from,
      to: input.to,
      keyword: input.keyword,
      field,
      limit: input.limit,
    });
    process.stdout.write(formatGraylogAggregate(body, args.output));
    return;
  }
  const input = toGraylogSearchInput({
    source: firstLogInput(args),
    relative: numberFlag(args, "relative"),
    from: stringFlag(args, "from"),
    to: stringFlag(args, "to"),
    keyword: stringFlag(args, "keyword"),
    limit: numberFlag(args, "limit"),
    sort: stringFlag(args, "sort"),
    fields: stringFlag(args, "fields"),
    filter: stringFlag(args, "filter"),
    offset: numberFlag(args, "offset"),
    decorate: booleanFlag(args, "decorate"),
  });
  const body = await client.search(input);
  const context = {
    query: input.query,
    rangeType: input.rangeType,
    relative: input.relative,
    from: input.from,
    to: input.to,
    keyword: input.keyword,
    groupBy: stringFlag(args, "groupBy"),
  };
  process.stdout.write(booleanFlag(args, "showFields") ? formatGraylogFields(body, args.output, context) : formatGraylogResult(body, args.output, context));
}

async function runPrometheus(prometheusArgv: string[], rootArgv: string[]): Promise<void> {
  const args = parsePrometheusArgs(prometheusArgv, rootArgv);
  const fileConfig = await loadConfigIfExists(args.configPath);
  const config = {
    ...(fileConfig?.prometheus ?? normalizePrometheusConfig(undefined)),
    baseUrl: stringFlagLike(args, "baseUrl") ?? fileConfig?.prometheus.baseUrl ?? normalizePrometheusConfig(undefined).baseUrl,
  };
  const client = createPrometheusClient(config);
  const [first, second] = args.command;

  if (first === "query" && second === "instant") {
    const input = toPrometheusQueryInput(args);
    const spec = buildQueryRequest(config, input);
    const envelope = args.output === "plain" ? await client.raw(spec) : successEnvelope(await client.query(input));
    process.stdout.write(formatPrometheusResult(envelope, args.output, { command: "query instant", query: input.query, includeValues: booleanFlagLike(args, "includeValues") }));
    return;
  }

  if (first === "query" && second === "range") {
    const input = toPrometheusQueryRangeInput(args);
    const spec = buildQueryRangeRequest(config, input);
    const envelope = args.output === "plain" ? await client.raw(spec) : successEnvelope(await client.queryRange(input));
    process.stdout.write(formatPrometheusResult(envelope, args.output, { command: "query range", query: input.query, includeValues: booleanFlagLike(args, "includeValues") }));
    return;
  }

  if (first === "query" && second === "labels") {
    const labelName = args.positionals[0];
    const metadata = toPrometheusMetadataInput(args);
    const spec = labelName ? buildLabelValuesRequest(config, labelName, metadata) : buildLabelsRequest(config, metadata);
    const data = args.output === "plain" ? await client.raw(spec) : successEnvelope(labelName ? await client.labelValues(labelName, metadata) : await client.labels(metadata));
    process.stdout.write(formatPrometheusResult(data, args.output, { command: "query labels" }));
    return;
  }

  if (first === "query" && second === "series") {
    const input = toPrometheusMetadataInput(args);
    const spec = buildSeriesRequest(config, input);
    const envelope = args.output === "plain" ? await client.raw(spec) : successEnvelope(await client.series(input));
    process.stdout.write(formatPrometheusResult(envelope, args.output, { command: "query series" }));
    return;
  }

  if (first === "ready") {
    process.stdout.write(await client.status(buildReadyRequest(config)));
    return;
  }

  if (first === "healthy") {
    process.stdout.write(await client.status(buildHealthyRequest(config)));
    return;
  }

  if (first === "build-info") {
    const spec = buildBuildInfoRequest(config);
    const envelope = args.output === "plain" ? await client.raw(spec) : successEnvelope(await client.buildInfo());
    process.stdout.write(formatPrometheusResult(envelope, args.output, { command: "build-info" }));
    return;
  }

  if (first === "runtime-info") {
    const spec = buildRuntimeInfoRequest(config);
    const envelope = args.output === "plain" ? await client.raw(spec) : successEnvelope(await client.runtimeInfo());
    process.stdout.write(formatPrometheusResult(envelope, args.output, { command: "runtime-info" }));
    return;
  }

  throw new Error(`unknown prometheus command: ${args.command.join(" ") || "<missing>"}`);
}

/** 为什么：Prometheus query 自己也有 -o 等参数，分支必须保留子命令后的原始 argv。 */
function extractPrometheusArgs(argv: string[]): string[] | undefined {
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token) {
      continue;
    }
    if (token === "-f") {
      index += 1;
      continue;
    }
    if (token === "prometheus" || token === "prom") {
      return argv.slice(index + 1);
    }
    return undefined;
  }
  return undefined;
}

function parsePrometheusArgs(argv: string[], rootArgv: string[]): PrometheusParsedArgs {
  const flags: Record<string, string | boolean> = {};
  const multiFlags: Record<string, string[]> = {};
  const command: string[] = [];
  const positionals: string[] = [];
  let configPath: string | undefined;
  let output: OutputMode = "human";

  for (let index = 0; index < rootArgv.length; index += 1) {
    const token = rootArgv[index];
    if (token === "-f") {
      configPath = requireValue(rootArgv, index, "-f");
      index += 1;
      continue;
    }
    if (token === "--output" || token === "-o") {
      output = parseOutputMode(requireValue(rootArgv, index, token));
      index += 1;
    }
    if (token === "prometheus" || token === "prom") {
      break;
    }
  }

  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token) {
      continue;
    }
    if (token === "--output" || token === "-o") {
      output = parseOutputMode(requireValue(argv, index, token));
      index += 1;
      continue;
    }
    if (token === "-f") {
      configPath = requireValue(argv, index, "-f");
      index += 1;
      continue;
    }
    if (token.startsWith("--")) {
      const [rawKey, inlineValue] = token.slice(2).split("=", 2);
      const key = rawKey ? camelCase(rawKey) : "";
      if (!key) {
        throw new Error(`invalid flag: ${token}`);
      }
      const value = inlineValue ?? argv[index + 1];
      if (inlineValue === undefined && value && !value.startsWith("-")) {
        index += 1;
      }
      const normalizedValue = inlineValue !== undefined ? inlineValue : value && !value.startsWith("-") ? value : true;
      if (key === "match" && typeof normalizedValue === "string") {
        multiFlags[key] = [...(multiFlags[key] ?? []), normalizedValue];
      } else {
        flags[key] = normalizedValue;
      }
      continue;
    }
    if (command.length < 2) {
      command.push(token);
    } else {
      positionals.push(token);
    }
  }

  return { configPath, output, command, flags, multiFlags, positionals };
}

function parseArgs(argv: string[]): ParsedArgs {
  const flags: Record<string, string | boolean> = {};
  const command: string[] = [];
  const positionals: string[] = [];
  let configPath: string | undefined;
  let output: OutputMode = "human";

  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token) {
      continue;
    }
    if (token === "-f") {
      configPath = requireValue(argv, index, "-f");
      index += 1;
      continue;
    }
    if (token === "--output" || token === "-o") {
      output = parseOutputMode(requireValue(argv, index, token));
      index += 1;
      continue;
    }
    if (token.startsWith("--")) {
      const [rawKey, inlineValue] = token.slice(2).split("=", 2);
      const key = rawKey ? camelCase(rawKey) : "";
      if (!key) {
        throw new Error(`invalid flag: ${token}`);
      }
      if (inlineValue !== undefined) {
        flags[key] = inlineValue;
        continue;
      }
      const next = argv[index + 1];
      if (next && !next.startsWith("-")) {
        flags[key] = next;
        index += 1;
      } else {
        flags[key] = true;
      }
      continue;
    }
    if (command.length < 2) {
      command.push(token);
    } else {
      positionals.push(token);
    }
  }

  return { configPath, output, command, flags, positionals };
}

function toGroupsInput(args: ParsedArgs, config: UptraceConfig): GroupsQueryInput {
  return {
    service: stringFlag(args, "service"),
    env: stringFlag(args, "env") ?? config.defaultEnv,
    query: stringFlag(args, "query"),
    search: stringFlag(args, "search"),
    limit: numberFlag(args, "limit"),
    durationGte: numberFlag(args, "durationGte"),
    durationLt: numberFlag(args, "durationLt"),
    timeStart: stringFlag(args, "timeStart"),
    timeEnd: stringFlag(args, "timeEnd"),
    timeGte: stringFlag(args, "timeGte"),
    timeLt: stringFlag(args, "timeLt"),
    timeDurSeconds: numberFlag(args, "timeDur") ?? numberFlag(args, "timeDurSeconds"),
  };
}

function booleanFlag(args: ParsedArgs, key: string): boolean | undefined {
  const value = args.flags[key];
  if (value === undefined) {
    return undefined;
  }
  if (value === true) {
    return true;
  }
  if (value === "true") {
    return true;
  }
  if (value === "false") {
    return false;
  }
  throw new Error(`flag --${key} must be true or false`);
}

function toSpansInput(args: ParsedArgs): SpansQueryInput {
  return {
    traceId: stringFlag(args, "traceId"),
    spanId: numberFlag(args, "spanId"),
    parentId: numberFlag(args, "parentId"),
    limit: numberFlag(args, "limit"),
    durationGte: numberFlag(args, "durationGte"),
    durationLt: numberFlag(args, "durationLt"),
    query: stringFlag(args, "query"),
    sortBy: stringFlag(args, "sortBy"),
    sortDesc: booleanFlag(args, "sortDesc"),
    page: numberFlag(args, "page"),
    timeStart: stringFlag(args, "timeStart"),
    timeEnd: stringFlag(args, "timeEnd"),
    timeGte: stringFlag(args, "timeGte"),
    timeLt: stringFlag(args, "timeLt"),
    timeDurSeconds: numberFlag(args, "timeDur") ?? numberFlag(args, "timeDurSeconds"),
  };
}

async function runDiagnose(config: UptraceConfig, args: ParsedArgs): Promise<void> {
  const client = createUptraceClient(config);
  const groups = await client.listGroups(toGroupsInput(args, config));
  if (args.output === "plain") {
    process.stdout.write(formatJson(groups));
    return;
  }
  process.stdout.write(formatGroups(groups, args.output));
}

function toPrometheusQueryInput(args: PrometheusParsedArgs): PrometheusQueryInput {
  return {
    query: firstPromqlInput(args),
    time: stringFlagLike(args, "time"),
    timeout: stringFlagLike(args, "timeout"),
    limit: numberFlagLike(args, "limit"),
  };
}

function toPrometheusQueryRangeInput(args: PrometheusParsedArgs): PrometheusQueryRangeInput {
  return {
    query: firstPromqlInput(args),
    start: stringFlagLike(args, "start"),
    end: stringFlagLike(args, "end"),
    step: stringFlagLike(args, "step"),
    timeout: stringFlagLike(args, "timeout"),
    limit: numberFlagLike(args, "limit"),
  };
}

function toPrometheusMetadataInput(args: PrometheusParsedArgs): PrometheusMetadataInput {
  return {
    matches: args.multiFlags.match,
    start: stringFlagLike(args, "start"),
    end: stringFlagLike(args, "end"),
    limit: numberFlagLike(args, "limit"),
  };
}

function firstPromqlInput(args: PrometheusParsedArgs): string {
  const value = args.positionals[0];
  if (!value) {
    throw new Error("prometheus query requires promql");
  }
  return value;
}

function firstTraceInput(args: ParsedArgs): string {
  const value = stringFlag(args, "traceId") ?? args.positionals[0];
  if (!value) {
    throw new Error("trace command requires trace id, trace URL, or --trace-id");
  }
  return value;
}

function firstLogInput(args: ParsedArgs): string {
  const value = stringFlag(args, "query") ?? args.command[1] ?? args.positionals[0];
  if (!value) {
    throw new Error("logs command requires query text, Graylog search URL, or --query");
  }
  return value;
}

function firstAggregateQueryInput(args: ParsedArgs): string {
  const value = stringFlag(args, "query") ?? args.positionals[0];
  if (!value) {
    throw new Error("logs aggregate requires query text, Graylog search URL, or --query");
  }
  return value;
}

function withProject(config: UptraceConfig, projectId: number | undefined): UptraceConfig {
  return projectId ? { ...config, projectId } : config;
}

function stringFlag(args: ParsedArgs, key: string): string | undefined {
  const value = args.flags[key];
  return typeof value === "string" ? value : undefined;
}

function numberFlag(args: ParsedArgs, key: string): number | undefined {
  const value = stringFlag(args, key);
  if (value === undefined) {
    return undefined;
  }
  const number = Number(value);
  if (!Number.isFinite(number)) {
    throw new Error(`flag --${key} must be a number`);
  }
  return number;
}

function stringFlagLike(args: PrometheusParsedArgs, key: string): string | undefined {
  const value = args.flags[key];
  return typeof value === "string" ? value : undefined;
}

function numberFlagLike(args: PrometheusParsedArgs, key: string): number | undefined {
  const value = stringFlagLike(args, key);
  if (value === undefined) {
    return undefined;
  }
  const number = Number(value);
  if (!Number.isFinite(number)) {
    throw new Error(`flag --${key} must be a number`);
  }
  return number;
}

function booleanFlagLike(args: PrometheusParsedArgs, key: string): boolean | undefined {
  const value = args.flags[key];
  if (value === undefined) {
    return undefined;
  }
  if (value === true || value === "true") {
    return true;
  }
  if (value === "false") {
    return false;
  }
  throw new Error(`flag --${key} must be true or false`);
}

function successEnvelope<T>(data: T): PrometheusEnvelope<T> {
  return { status: "success", data };
}

function requireValue(argv: string[], index: number, flag: string): string {
  const value = argv[index + 1];
  if (!value) {
    throw new Error(`${flag} requires a value`);
  }
  return value;
}

function parseOutputMode(value: string): OutputMode {
  if (value === "human" || value === "agent" || value === "plain") {
    return value;
  }
  throw new Error("--output must be human, agent, or plain");
}

function camelCase(value: string): string {
  return value.replace(/-([a-z])/g, (_, char: string) => char.toUpperCase());
}

if (import.meta.main) {
  main().catch((error: unknown) => {
    const message = error instanceof Error ? error.message : String(error);
    process.stderr.write(`${message}\n`);
    process.exit(1);
  });
}
