import type { OutputMode } from "../uptrace/types";
import type { GraylogAggregateResult, GraylogFormatContext, GraylogMessage, GraylogSearchResponse } from "./types";

interface LoggerSummary {
  value: string;
  count: number;
  trace_ids: string[];
  samples: MessageSample[];
}

interface MessageSample {
  timestamp?: string | undefined;
  source?: string | undefined;
  level?: number | string | undefined;
  message?: string | undefined;
  trace_id?: string | undefined;
  logger_name?: string | undefined;
}

/** 为什么：日志查询既要给人快速扫错误现场，也要给 Agent 稳定字段继续串 Uptrace。 */
export function formatGraylogResult(response: GraylogSearchResponse, mode: OutputMode, context: GraylogFormatContext): string {
  if (mode === "plain") {
    return `${JSON.stringify(response, null, 2)}\n`;
  }
  const messages = toSamples(response);
  const loggerSummary = summarizeByField(response, "logger_name");
  const groups = context.groupBy ? summarizeByField(response, context.groupBy) : undefined;
  if (mode === "agent") {
    return `${JSON.stringify(
      {
        query: context.query,
        range: formatRange(context),
        total_results: response.total_results ?? messages.length,
        returned_messages: messages.length,
        trace_ids: unique(messages.map((item) => item.trace_id).filter(isString)),
        logger_summary: loggerSummary.map((item) => toNamedGroup("logger_name", item)),
        group_by: context.groupBy,
        groups: groups?.map((item) => toNamedGroup(context.groupBy ?? "group", item)),
        messages,
      },
      null,
      2,
    )}\n`;
  }
  return groups ? formatGroupedHuman(response, context.groupBy ?? "group", groups, messages.length) : formatLogsHuman(response, messages);
}

/** 为什么：排障时先知道可统计字段, Agent 才能选择合适的 group-by 维度继续收敛。 */
export function formatGraylogFields(response: GraylogSearchResponse, mode: OutputMode, context: GraylogFormatContext): string {
  const fields = fieldNames(response);
  const suggested = fields.filter(isSuggestedGroupField);
  if (mode === "plain") {
    return `${JSON.stringify({ fields }, null, 2)}\n`;
  }
  if (mode === "agent") {
    return `${JSON.stringify(
      {
        query: context.query,
        range: formatRange(context),
        field_count: fields.length,
        fields,
        suggested_group_by: suggested,
      },
      null,
      2,
    )}\n`;
  }
  return `Graylog fields: count=${fields.length}\n${fields.map((field) => `- ${field}`).join("\n")}\n`;
}

export function formatGraylogAggregate(result: GraylogAggregateResult, mode: OutputMode): string {
  if (mode === "plain" || mode === "agent") {
    return `${JSON.stringify(result, null, 2)}\n`;
  }
  const lines = [`Graylog aggregate: field=${result.field} total=${result.total} rows=${result.rows.length}`];
  for (const row of result.rows) {
    lines.push(`${row.key}\t${row.count}`);
  }
  return `${lines.join("\n")}\n`;
}

function formatLogsHuman(response: GraylogSearchResponse, messages: MessageSample[]): string {
  if (messages.length === 0) {
    return "No logs.\n";
  }
  const lines = [`Graylog logs: total=${response.total_results ?? "-"} returned=${messages.length}`];
  for (const sample of messages) {
    const traceText = sample.trace_id ? ` trace=${sample.trace_id}` : "";
    lines.push(`${sample.timestamp ?? "-"} ${sample.source ?? "-"} [${sample.level ?? "-"}] ${sample.logger_name ?? "-"}${traceText}`);
    lines.push(`  ${sample.message ?? ""}`);
  }
  return `${lines.join("\n")}\n`;
}

function formatGroupedHuman(response: GraylogSearchResponse, field: string, summaries: LoggerSummary[], returned: number): string {
  if (summaries.length === 0) {
    return "No logs.\n";
  }
  const sampled = typeof response.total_results === "number" && response.total_results > returned;
  const missing = summaries.filter((summary) => summary.value === `<missing_${field}>`).reduce((sum, item) => sum + item.count, 0);
  const label = sampled ? "Graylog groups(sampled)" : "Graylog groups";
  const lines = [`${label}: total=${response.total_results ?? "-"} returned=${returned} group_by=${field} groups=${summaries.length}`];
  if (sampled) {
    lines.push("warning: grouped counts are computed from returned messages only, not full-result aggregation.");
  }
  if (missing > 0) {
    lines.push(`warning: group field missing in returned messages: field=${field} missing=${missing}.`);
  }
  for (const summary of summaries) {
    lines.push(`- ${field}=${summary.value} count=${summary.count} trace_ids=${summary.trace_ids.slice(0, 5).join(",") || "-"}`);
    for (const sample of summary.samples.slice(0, 2)) {
      lines.push(`  ${sample.timestamp ?? "-"} ${sample.source ?? "-"} ${sample.message ?? ""}`);
    }
  }
  return `${lines.join("\n")}\n`;
}

function summarizeByField(response: GraylogSearchResponse, field: string): LoggerSummary[] {
  const map = new Map<string, LoggerSummary>();
  for (const item of response.messages ?? []) {
    const message = item.message ?? {};
    const value = stringValue(message[field]) ?? (field === "logger_name" ? loggerNameOf(message) : `<missing_${field}>`);
    const traceId = traceIdOf(message);
    const summary = map.get(value) ?? {
      value,
      count: 0,
      trace_ids: [],
      samples: [],
    };
    summary.count += 1;
    if (traceId && !summary.trace_ids.includes(traceId)) {
      summary.trace_ids.push(traceId);
    }
    if (summary.samples.length < 5) {
      summary.samples.push(toSample(message, traceId));
    }
    map.set(value, summary);
  }
  return [...map.values()].sort((left, right) => right.count - left.count);
}

function loggerNameOf(message: GraylogMessage): string {
  return message.logger_name ?? message.loggerName ?? "<missing_logger_name>";
}

function traceIdOf(message: GraylogMessage): string | undefined {
  return message.trace_id ?? message.traceId;
}

function toSample(message: GraylogMessage, traceId: string | undefined): MessageSample {
  return {
    timestamp: message.timestamp,
    source: message.source,
    level: message.level,
    message: message.message,
    trace_id: traceId,
    logger_name: loggerNameOf(message),
  };
}

function toSamples(response: GraylogSearchResponse): MessageSample[] {
  return (response.messages ?? []).map((item) => {
    const message = item.message ?? {};
    return toSample(message, traceIdOf(message));
  });
}

function toNamedGroup(field: string, summary: LoggerSummary): Record<string, unknown> {
  return {
    [field]: summary.value,
    count: summary.count,
    trace_ids: summary.trace_ids,
    samples: summary.samples,
  };
}

function formatRange(context: GraylogFormatContext): Record<string, string | number | undefined> {
  return {
    type: context.rangeType,
    relative: context.relative,
    from: context.from,
    to: context.to,
    keyword: context.keyword,
  };
}

function unique(values: string[]): string[] {
  return [...new Set(values)];
}

function isString(value: unknown): value is string {
  return typeof value === "string";
}

function stringValue(value: unknown): string | undefined {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return undefined;
}

function fieldNames(response: GraylogSearchResponse): string[] {
  const set = new Set<string>(response.fields ?? []);
  for (const item of response.messages ?? []) {
    const message = item.message ?? {};
    for (const key of Object.keys(message)) {
      set.add(key);
    }
  }
  return [...set].sort();
}

function isSuggestedGroupField(field: string): boolean {
  return [
    "app",
    "facility",
    "level",
    "logger_name",
    "source",
    "thread_name",
    "trace_id",
  ].includes(field);
}
