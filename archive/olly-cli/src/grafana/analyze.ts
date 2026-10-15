import type { GrafanaClient } from "./api";
import { buildDashUrl, buildQueryBody, collectPanels, parseDashRef, readTargetSummaries } from "./query";
import type { AnalyzeInput, AnalyzeResult, GrafanaQueryBody, PanelSummary } from "./types";

/** 为什么：Agent 要看看板当前值，必须拉定义并执行 panel 查询，而不是只返回 PromQL。 */
export async function analyzeDashboard(client: GrafanaClient, input: AnalyzeInput): Promise<AnalyzeResult> {
  const ref = parseDashRef(input.source);
  const orgId = input.orgId ?? ref.orgId;
  const payload = await client.getDashboard(ref.uid, orgId);
  const dash = payload.dashboard;
  const from = input.from ?? ref.from ?? dash.time?.from;
  const to = input.to ?? ref.to ?? dash.time?.to;
  if (!from || !to) {
    throw new Error("grafana dashboard time range is required");
  }
  if (!dash.uid) {
    throw new Error("grafana dashboard missing uid");
  }

  const queryBodies: GrafanaQueryBody[] = [];
  const panels: PanelSummary[] = [];
  for (const panel of collectPanels(dash.panels)) {
    const body = buildQueryBody(panel, from, to);
    const response = await client.queryPanel(panel, from, to, orgId);
    queryBodies.push(body);
    panels.push({
      id: panel.id,
      title: panel.title ?? `panel-${panel.id ?? "unknown"}`,
      type: panel.type ?? "unknown",
      targets: readTargetSummaries(body, response),
    });
  }

  const dashPath = payload.meta?.url;
  const url = buildDashUrl(input.source, input.webBaseUrl, dashPath, dash.uid, orgId, from, to);
  return {
    title: dash.title ?? dash.uid,
    uid: dash.uid,
    url,
    orgId,
    time: { from, to },
    panels,
    dashboard: payload,
    queryBodies,
  };
}
