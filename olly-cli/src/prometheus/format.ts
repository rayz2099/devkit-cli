import type { OutputMode } from "../uptrace/types";
import type { PrometheusEnvelope, PrometheusFormatContext, PrometheusQueryData, PrometheusScalarValue, PrometheusSeries } from "./types";

interface SeriesSummary {
  metric: Record<string, string>;
  sample_count: number;
  first?: PrometheusScalarValue | undefined;
  last?: PrometheusScalarValue | undefined;
  min?: number | undefined;
  max?: number | undefined;
  avg?: number | undefined;
  values?: PrometheusScalarValue[] | undefined;
}

/** 为什么：Prometheus 原始点位容易撑爆 Agent 上下文，默认输出统计摘要。 */
export function formatPrometheusResult(envelope: PrometheusEnvelope<unknown>, mode: OutputMode, context: PrometheusFormatContext): string {
  if (mode === "plain") {
    return `${JSON.stringify(envelope, null, 2)}\n`;
  }
  const data = envelope.data;
  if (isQueryData(data)) {
    return mode === "agent" ? formatQueryAgent(data, context) : formatQueryHuman(data);
  }
  return formatMetadata(data, mode);
}

function formatQueryAgent(data: PrometheusQueryData, context: PrometheusFormatContext): string {
  const series = Array.isArray(data.result) ? data.result.filter(isSeries) : [];
  const summaries = series.map((item) => summarizeSeries(item, Boolean(context.includeValues)));
  return `${JSON.stringify(
    {
      command: context.command,
      query: context.query,
      result_type: data.resultType,
      series_count: summaries.length,
      sample_count: summaries.reduce((sum, item) => sum + item.sample_count, 0),
      series: summaries,
    },
    null,
    2,
  )}\n`;
}

function formatQueryHuman(data: PrometheusQueryData): string {
  if (!Array.isArray(data.result)) {
    return `${JSON.stringify(data.result)}\n`;
  }
  if (data.result.length === 0) {
    return "No result.\n";
  }

  const lines = [`Prometheus ${data.resultType}:`];
  for (const item of data.result) {
    if (!isSeries(item)) {
      lines.push(`- ${JSON.stringify(item)}`);
      continue;
    }
    const summary = summarizeSeries(item, false);
    const labelText = formatMetric(item.metric);
    if (item.value) {
      lines.push(`- ${labelText} value=${item.value[1]} time=${item.value[0]}`);
      continue;
    }
    lines.push(`- ${labelText} samples=${summary.sample_count} first=${formatSample(summary.first)} last=${formatSample(summary.last)} min=${summary.min ?? "-"} max=${summary.max ?? "-"} avg=${summary.avg ?? "-"}`);
  }
  return `${lines.join("\n")}\n`;
}

function formatMetadata(data: unknown, mode: OutputMode): string {
  if (!Array.isArray(data)) {
    return `${JSON.stringify(data, null, 2)}\n`;
  }
  if (mode === "agent") {
    return `${JSON.stringify({ count: data.length, values: data }, null, 2)}\n`;
  }
  return data.map((item) => (typeof item === "string" ? item : formatMetric(item as Record<string, string>))).join("\n") + "\n";
}

function summarizeSeries(series: PrometheusSeries, includeValues: boolean): SeriesSummary {
  const samples = series.values ?? (series.value ? [series.value] : []);
  const numbers = samples.map((sample) => Number(sample[1])).filter((value) => Number.isFinite(value));
  const min = numbers.length > 0 ? Math.min(...numbers) : undefined;
  const max = numbers.length > 0 ? Math.max(...numbers) : undefined;
  const avg = numbers.length > 0 ? numbers.reduce((sum, value) => sum + value, 0) / numbers.length : undefined;
  return {
    metric: series.metric,
    sample_count: samples.length,
    first: samples[0],
    last: samples[samples.length - 1],
    min,
    max,
    avg,
    values: includeValues ? samples : undefined,
  };
}

function isQueryData(value: unknown): value is PrometheusQueryData {
  return Boolean(value && typeof value === "object" && "resultType" in value && "result" in value);
}

function isSeries(value: unknown): value is PrometheusSeries {
  return Boolean(value && typeof value === "object" && "metric" in value);
}

function formatMetric(metric: Record<string, string>): string {
  const pairs = Object.entries(metric).map(([key, value]) => `${key}="${value}"`);
  return `{${pairs.join(",")}}`;
}

function formatSample(sample: PrometheusScalarValue | undefined): string {
  return sample ? `${sample[1]}@${sample[0]}` : "-";
}
