import type { OutputMode } from "../uptrace/types";
import type { AnalyzeResult, SeriesSummary, TargetSummary } from "./types";

const HUMAN_TOP_LIMIT = 10;

/** 为什么：原始 frame 点位会撑爆 Agent 上下文，默认只保留 last/max。 */
export function formatGrafanaResult(result: AnalyzeResult, mode: OutputMode): string {
  if (mode === "plain") {
    return `${JSON.stringify({ dashboard: result.dashboard, time: result.time, queries: result.queryBodies }, null, 2)}\n`;
  }
  if (mode === "agent") {
    return `${JSON.stringify(toAgentPayload(result), null, 2)}\n`;
  }
  return formatHuman(result);
}

function toAgentPayload(result: AnalyzeResult): unknown {
  return {
    title: result.title,
    uid: result.uid,
    url: result.url,
    orgId: result.orgId,
    time: result.time,
    panel_count: result.panels.length,
    panels: result.panels,
  };
}

/** 为什么：人要先拿到可点开的看板链接，再扫 top series，不能把几百条 path 全打出来。 */
function formatHuman(result: AnalyzeResult): string {
  const seriesCount = result.panels.reduce((sum, panel) => sum + panel.targets.reduce((inner, target) => inner + target.series_count, 0), 0);
  const lines = [
    `Grafana: ${result.title}`,
    `URL: ${result.url ?? "-"}`,
    `Time: ${result.time.from} → ${result.time.to}`,
    `Panels: ${result.panels.length}  Series: ${seriesCount}`,
  ];
  if (result.panels.length === 0) {
    lines.push("No panels.");
    return `${lines.join("\n")}\n`;
  }
  for (const panel of result.panels) {
    lines.push("", `#${panel.id ?? "-"} ${panel.title}`);
    for (const target of panel.targets) {
      lines.push(`  ${target.refId}  ${target.expr}  series=${target.series_count}`);
      const ranked = rankSeries(target);
      if (ranked.length === 0) {
        lines.push("    (no series)");
        continue;
      }
      if (target.series_count > HUMAN_TOP_LIMIT) {
        lines.push(`    top ${HUMAN_TOP_LIMIT} by max:`);
      }
      for (const series of ranked) {
        lines.push(`    ${formatSeries(series)}`);
      }
    }
  }
  return `${lines.join("\n")}\n`;
}

function rankSeries(target: TargetSummary): SeriesSummary[] {
  return [...target.series].sort((left, right) => (right.max ?? -Infinity) - (left.max ?? -Infinity)).slice(0, HUMAN_TOP_LIMIT);
}

function formatSeries(series: SeriesSummary): string {
  return `${series.name}  last=${fmtNum(series.last)} max=${fmtNum(series.max)} samples=${series.sample_count}`;
}

function fmtNum(value: number | undefined): string {
  if (value === undefined) {
    return "-";
  }
  return String(value);
}
