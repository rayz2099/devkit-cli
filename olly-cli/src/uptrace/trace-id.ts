const TRACE_ID_PATTERN = /\b[a-fA-F0-9]{32}\b/;

export interface ParsedTraceInput {
  traceId: string;
  projectId?: number;
}

/** 为什么：压测日志、浏览器 URL、裸 trace-id 都应收敛成同一个查询入口。 */
export function parseTraceInput(input: string): ParsedTraceInput {
  const trimmed = input.trim();
  const urlParsed = parseTraceUrl(trimmed);
  if (urlParsed) {
    return urlParsed;
  }

  const match = trimmed.match(TRACE_ID_PATTERN);
  if (!match) {
    throw new Error("invalid trace id input: expected 32 hex trace id or Uptrace trace URL");
  }

  return { traceId: match[0].toLowerCase() };
}

/** 为什么：Uptrace 页面 URL 里带 project id，CLI 应避免用户重复输入。 */
function parseTraceUrl(input: string): ParsedTraceInput | undefined {
  let url: URL;
  try {
    url = new URL(input);
  } catch {
    return undefined;
  }

  const parts = url.pathname.split("/").filter(Boolean);
  const tracesIndex = parts.indexOf("traces");
  if (tracesIndex < 0) {
    return undefined;
  }

  const projectPart = parts[tracesIndex + 1];
  const tracePart = parts[tracesIndex + 2];
  if (!projectPart || !tracePart || !TRACE_ID_PATTERN.test(tracePart)) {
    throw new Error("invalid Uptrace trace URL: expected /traces/{project_id}/{trace_id}");
  }

  const projectId = Number(projectPart);
  if (!Number.isInteger(projectId)) {
    throw new Error("invalid Uptrace trace URL: project id must be an integer");
  }

  return {
    projectId,
    traceId: tracePart.toLowerCase(),
  };
}
