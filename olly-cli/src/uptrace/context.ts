import { parseTraceInput } from "./trace-id";
import { buildTraceTree, getAttrs, getDuration } from "./trace-tree";
import type { AgentContext, ContextInput, TraceNode, UptraceConfig, UptraceSpan } from "./types";

/** 为什么：context 是给 LLM 的稳定输入，不能让模型再猜 trace 结构。 */
export function buildContext(input: ContextInput): AgentContext {
  const parsed = parseTraceInput(input.traceInput);
  const projectId = parsed.projectId ?? input.config.projectId;
  const webBaseUrl = input.config.webBaseUrl ?? input.config.baseUrl;
  const context: AgentContext = {
    projectId,
    traceId: parsed.traceId,
    traceUrl: `${trimRightSlash(webBaseUrl)}/traces/${projectId}/${parsed.traceId}`,
    query: input.query ?? {},
    tree: buildTraceTree(input.spans),
    sourceHints: collectSourceHints(input.spans),
  };
  return context;
}

/** 为什么：agent 模式要降低噪音，保留排查需要的调用链和源码线索。 */
export function formatContextAgent(context: AgentContext): string {
  const lines: string[] = [
    `trace_id: ${context.traceId}`,
    `project_id: ${context.projectId}`,
    `trace_url: ${context.traceUrl}`,
  ];

  if (context.query.service || context.query.env || context.query.uri) {
    lines.push("query:");
    addOptionalLine(lines, "service", context.query.service, 2);
    addOptionalLine(lines, "env", context.query.env, 2);
    addOptionalLine(lines, "uri", context.query.uri, 2);
  }

  lines.push("summary:");
  lines.push(`  total_duration_us: ${Math.round(context.tree.totalDuration)}`);
  lines.push(`  span_count: ${context.tree.allNodes.length}`);
  lines.push(`  error_span_count: ${context.tree.errorSpans.length}`);
  lines.push("top_slow_spans:");
  for (const node of context.tree.topSlowSpans) {
    lines.push(`  - ${spanOneLine(node)}`);
  }

  lines.push("call_tree:");
  for (const root of context.tree.roots) {
    appendNode(lines, root);
  }

  if (context.sourceHints.length > 0) {
    lines.push("source_hints:");
    for (const hint of context.sourceHints) {
      lines.push(`  - ${hint}`);
    }
  }

  return `${lines.join("\n")}\n`;
}

/** 为什么：human 模式应能快速扫出入口、慢点和链接。 */
export function formatContextHuman(context: AgentContext): string {
  const lines = [
    `Trace: ${context.traceId}`,
    `Project: ${context.projectId}`,
    `URL: ${context.traceUrl}`,
    `Spans: ${context.tree.allNodes.length}, Errors: ${context.tree.errorSpans.length}, Duration(us): ${Math.round(context.tree.totalDuration)}`,
    "",
    "Top slow spans:",
  ];
  for (const node of context.tree.topSlowSpans) {
    lines.push(`- ${spanOneLine(node)}`);
  }
  lines.push("", "Call tree:");
  for (const root of context.tree.roots) {
    appendNode(lines, root);
  }
  return `${lines.join("\n")}\n`;
}

function appendNode(lines: string[], node: TraceNode): void {
  lines.push(`${"  ".repeat(node.depth)}- ${spanOneLine(node)}`);
  for (const child of node.children) {
    appendNode(lines, child);
  }
}

function spanOneLine(node: TraceNode): string {
  const span = node.span;
  const attrs = getAttrs(span);
  const service = valueToString(attrs.service_name ?? attrs["service.name"]);
  const route = valueToString(attrs.http_route ?? attrs["http.route"] ?? attrs.http_target ?? attrs["http.target"]);
  const status = node.isError ? " error=true" : "";
  const slow = node.isSlow ? " slow=true" : "";
  const suffix = [service ? ` service=${service}` : "", route ? ` route=${route}` : "", status, slow].join("");
  return `id=${span.id} name="${span.name ?? "<unnamed>"}" kind=${span.kind ?? "unknown"} duration_us=${Math.round(getDuration(span))}${suffix}`;
}

function collectSourceHints(spans: UptraceSpan[]): string[] {
  const hints = new Set<string>();
  for (const span of spans) {
    const attrs = getAttrs(span);
    addHint(hints, "service_name", attrs.service_name ?? attrs["service.name"]);
    addHint(hints, "http_route", attrs.http_route ?? attrs["http.route"]);
    addHint(hints, "http_target", attrs.http_target ?? attrs["http.target"]);
    addHint(hints, "rpc_method", attrs.rpc_method ?? attrs["rpc.method"]);
    addHint(hints, "peer_service", attrs.peer_service ?? attrs["peer.service"]);
    addHint(hints, "db_statement", attrs.db_statement ?? attrs["db.statement"]);
  }
  return [...hints];
}

function addHint(hints: Set<string>, key: string, value: unknown): void {
  const text = valueToString(value);
  if (text) {
    hints.add(`${key}: ${text}`);
  }
}

function addOptionalLine(lines: string[], key: string, value: string | undefined, indent: number): void {
  if (value) {
    lines.push(`${" ".repeat(indent)}${key}: ${value}`);
  }
}

function valueToString(value: unknown): string | undefined {
  if (typeof value === "string" && value.length > 0) {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return undefined;
}

function trimRightSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
