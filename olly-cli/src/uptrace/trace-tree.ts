import type { TraceNode, TraceTree, UptraceSpan } from "./types";

const SLOW_TOP_LIMIT = 10;

/** 为什么：Uptrace 返回的是 span 列表，LLM 和人排查都需要 parent-child 调用链。 */
export function buildTraceTree(spans: UptraceSpan[]): TraceTree {
  const totalDuration = Math.max(...spans.map((span) => getDuration(span)), 0);
  const nodes = spans.map((span) => createNode(span, totalDuration));
  const byId = new Map(nodes.map((node) => [String(node.span.id), node]));
  const roots: TraceNode[] = [];
  const orphans: TraceNode[] = [];

  for (const node of nodes) {
    const parentId = getParentId(node.span);
    if (!parentId || parentId === "0") {
      roots.push(node);
      continue;
    }

    const parent = byId.get(parentId);
    if (!parent) {
      orphans.push(node);
      roots.push(node);
      continue;
    }

    node.depth = parent.depth + 1;
    parent.children.push(node);
  }

  for (const root of roots) {
    assignDepth(root, 0);
    sortChildren(root);
  }

  const sortedByDuration = [...nodes].sort((left, right) => getDuration(right.span) - getDuration(left.span));
  const slowThreshold = sortedByDuration[Math.min(SLOW_TOP_LIMIT - 1, sortedByDuration.length - 1)]?.span;
  const threshold = slowThreshold ? getDuration(slowThreshold) : Number.POSITIVE_INFINITY;
  for (const node of nodes) {
    node.isSlow = getDuration(node.span) >= threshold && nodes.length > 0;
  }

  return {
    roots: roots.sort((left, right) => compareSpanTime(left.span, right.span)),
    orphans,
    allNodes: nodes,
    topSlowSpans: sortedByDuration.slice(0, SLOW_TOP_LIMIT),
    errorSpans: nodes.filter((node) => node.isError),
    totalDuration,
  };
}

export function getDuration(span: UptraceSpan): number {
  return typeof span.duration === "number" ? span.duration : 0;
}

export function getAttrs(span: UptraceSpan): Record<string, unknown> {
  return span.attrs ?? span.attributes ?? {};
}

export function getParentId(span: UptraceSpan): string | undefined {
  const value = span.parentId ?? span.parent_id;
  return value === undefined || value === null ? undefined : String(value);
}

function createNode(span: UptraceSpan, totalDuration: number): TraceNode {
  return {
    span,
    children: [],
    depth: 0,
    durationShare: totalDuration > 0 ? getDuration(span) / totalDuration : 0,
    isError: isErrorSpan(span),
    isSlow: false,
  };
}

function isErrorSpan(span: UptraceSpan): boolean {
  const status = String(span.statusCode ?? span.status_code ?? "").toLowerCase();
  const attrs = getAttrs(span);
  const attrStatus = String(attrs["otel.status_code"] ?? attrs["status_code"] ?? "").toLowerCase();
  return status === "error" || attrStatus === "error";
}

function assignDepth(node: TraceNode, depth: number): void {
  node.depth = depth;
  for (const child of node.children) {
    assignDepth(child, depth + 1);
  }
}

function sortChildren(node: TraceNode): void {
  node.children.sort((left, right) => compareSpanTime(left.span, right.span));
  for (const child of node.children) {
    sortChildren(child);
  }
}

function compareSpanTime(left: UptraceSpan, right: UptraceSpan): number {
  return new Date(left.time ?? 0).getTime() - new Date(right.time ?? 0).getTime();
}
