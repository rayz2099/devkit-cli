import { buildProjectionSource, parseSelection, projectFields, renderHttpExchange } from "./selection";
import { buildPrefix } from "./uri";
import type { NormalizedEntry, OutputFormat } from "./types";

/**
 * 将命中的 entry 渲染为 text/json 输出。
 */
export function formatEntries(
  entries: NormalizedEntry[],
  select: string,
  format: OutputFormat,
  dedupe: boolean,
  prefixDepth: number,
): string {
  const selection = parseSelection(select);

  if (format === "json") {
    const serialized = dedupe
      ? dedupeJsonEntries(entries, selection, prefixDepth)
      : entries.map((entry) => toJsonEntry(entry, selection, prefixDepth));
    return `${JSON.stringify(serialized, null, 2)}\n`;
  }

  const blocks = entries.map((entry) => selectText(entry, selection, prefixDepth));
  const rendered = dedupe ? Array.from(new Set(blocks)) : blocks;
  const separator = selection.kind === "http" ? "\n\n" : "\n";
  return rendered.length === 0 ? "" : `${rendered.join(separator)}\n`;
}

function dedupeJsonEntries(
  entries: NormalizedEntry[],
  selection: ReturnType<typeof parseSelection>,
  prefixDepth: number,
): Array<Record<string, unknown>> {
  const seen = new Set<string>();
  const results: Array<Record<string, unknown>> = [];

  for (const entry of entries) {
    const value = toJsonEntry(entry, selection, prefixDepth);
    const key = JSON.stringify(value);
    if (seen.has(key)) {
      continue;
    }

    seen.add(key);
    results.push(value);
  }

  return results;
}

function toJsonEntry(
  entry: NormalizedEntry,
  selection: ReturnType<typeof parseSelection>,
  prefixDepth: number,
): Record<string, unknown> {
  const source = buildProjectionSource(entry, prefixDepth);

  if (selection.kind === "fields") {
    return projectFields(source, selection.fields ?? []);
  }

  if (selection.kind === "scalar") {
    return {
      [selection.value ?? "path"]: selectScalar(entry, selection.value ?? "path", prefixDepth),
    };
  }

  return source;
}

function selectText(
  entry: NormalizedEntry,
  selection: ReturnType<typeof parseSelection>,
  prefixDepth: number,
): string {
  switch (selection.kind) {
    case "http":
      return renderHttpExchange(entry);
    case "scalar":
      return selectScalar(entry, selection.value ?? "path", prefixDepth);
    case "fields":
      return JSON.stringify(projectFields(buildProjectionSource(entry, prefixDepth), selection.fields ?? []));
    default:
      return entry.path;
  }
}

function selectScalar(entry: NormalizedEntry, select: "path" | "url" | "prefix", prefixDepth: number): string {
  switch (select) {
    case "url":
      return entry.url;
    case "prefix":
      return buildPrefix(entry.path, prefixDepth);
    case "path":
    default:
      return entry.path;
  }
}
