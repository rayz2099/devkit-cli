import { buildPrefix } from "./uri";
import type { HarHeader, NormalizedEntry } from "./types";

export interface SelectionSpec {
  kind: "http" | "scalar" | "fields";
  value?: "path" | "url" | "prefix";
  fields?: string[];
}

/**
 * 解析 -s/--select 参数，支持快捷值和逗号分隔字段列表。
 */
export function parseSelection(select: string): SelectionSpec {
  const normalized = select.trim();

  if (normalized === "" || normalized === "http") {
    return { kind: "http" };
  }

  if (normalized === "path" || normalized === "url" || normalized === "prefix") {
    return { kind: "scalar", value: normalized };
  }

  const fields = normalized.split(",").map((item) => item.trim()).filter(Boolean);
  if (fields.length === 0) {
    return { kind: "http" };
  }

  return {
    kind: "fields",
    fields,
  };
}

/**
 * 生成默认 text 输出的 HTTP 明文块。
 */
export function renderHttpExchange(entry: NormalizedEntry): string {
  return [
    "[Request]",
    renderRequestText(entry),
    "",
    "[Response]",
    renderResponseText(entry),
  ].join("\n");
}

/**
 * 构造字段投影所需的标准化上下文。
 */
export function buildProjectionSource(entry: NormalizedEntry, prefixDepth: number): Record<string, unknown> {
  return {
    sourceFile: entry.sourceFile,
    path: entry.path,
    url: entry.url,
    prefix: buildPrefix(entry.path, prefixDepth),
    http: {
      method: entry.method,
      status: entry.httpStatus,
    },
    request: {
      method: entry.method,
      url: entry.url,
      path: entry.path,
      line: buildRequestLine(entry),
      httpVersion: entry.requestHttpVersion,
      headers: entry.requestHeaders,
      body: entry.requestBodyText,
      raw: renderRequestText(entry),
    },
    response: {
      status: entry.httpStatus,
      statusText: entry.statusText,
      line: buildResponseLine(entry),
      httpVersion: entry.responseHttpVersion,
      mimeType: entry.mimeType,
      headers: entry.responseHeaders,
      body: entry.responseBodyText,
      raw: renderResponseText(entry),
    },
    body: entry.body,
  };
}

/**
 * 将字段列表映射到一个保留指定路径的新对象。
 */
export function projectFields(source: Record<string, unknown>, fields: string[]): Record<string, unknown> {
  const projected: Record<string, unknown> = {};

  for (const field of fields) {
    const value = resolvePath(source, field);
    if (value === undefined) {
      continue;
    }

    setPath(projected, field, value);
  }

  return projected;
}

function resolvePath(source: Record<string, unknown>, path: string): unknown {
  return path.split(".").reduce<unknown>((current, segment) => {
    if (current === null || current === undefined || typeof current !== "object") {
      return undefined;
    }

    return (current as Record<string, unknown>)[segment];
  }, source);
}

function setPath(target: Record<string, unknown>, path: string, value: unknown): void {
  const segments = path.split(".");
  let current: Record<string, unknown> = target;

  for (let index = 0; index < segments.length - 1; index += 1) {
    const segment = segments[index];
    const next = current[segment];

    if (next === undefined || next === null || typeof next !== "object" || Array.isArray(next)) {
      current[segment] = {};
    }

    current = current[segment] as Record<string, unknown>;
  }

  current[segments[segments.length - 1]] = value;
}

function buildRequestLine(entry: NormalizedEntry): string {
  return [entry.method, entry.pathWithQuery, entry.requestHttpVersion].filter(Boolean).join(" ");
}

function buildResponseLine(entry: NormalizedEntry): string {
  return [entry.responseHttpVersion, String(entry.httpStatus), entry.statusText].filter(Boolean).join(" ");
}

function renderRequestText(entry: NormalizedEntry): string {
  return renderHttpMessage(
    buildRequestLine(entry),
    entry.requestHeaders,
    entry.requestBodyText,
  );
}

function renderResponseText(entry: NormalizedEntry): string {
  return renderHttpMessage(
    buildResponseLine(entry),
    entry.responseHeaders,
    entry.responseBodyText,
  );
}

function renderHttpMessage(startLine: string, headers: HarHeader[], body?: string): string {
  const lines = [startLine, ...headers.map((header) => `${header.name}: ${header.value}`)];

  if (body !== undefined && body !== "") {
    lines.push("", body);
  }

  return lines.join("\n");
}
