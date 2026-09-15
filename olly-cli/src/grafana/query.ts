import type {
  DashRef,
  GrafanaConfig,
  GrafanaFrame,
  GrafanaPanel,
  GrafanaQueryBody,
  GrafanaQueryItem,
  GrafanaQueryResponse,
  GrafanaQuerySpec,
  GrafanaTarget,
  SeriesSummary,
  TargetSummary,
} from "./types";

export const DEFAULT_INTERVAL_MS = 15000;
export const DEFAULT_MAX_PTS = 1100;

/** 为什么：公网 Grafana 只用来抽 uid/时间窗，真正请求必须打配置里的内网地址。 */
export function parseDashRef(source: string): DashRef {
  const trimmed = source.trim();
  if (trimmed === "") {
    throw new Error("grafana dashboard url or uid is required");
  }

  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    if (trimmed.includes("/") || /\s/.test(trimmed)) {
      throw new Error(`grafana dashboard ref is invalid: ${trimmed}`);
    }
    return { uid: trimmed };
  }

  const match = url.pathname.match(/\/d\/([^/]+)/) ?? url.pathname.match(/\/dashboard\/d\/([^/]+)/);
  if (!match?.[1]) {
    throw new Error(`grafana url missing dashboard uid: ${trimmed}`);
  }

  return {
    uid: decodeURIComponent(match[1]),
    orgId: parseOrgId(url.searchParams.get("orgId")),
    from: url.searchParams.get("from") ?? undefined,
    to: url.searchParams.get("to") ?? undefined,
  };
}

/** 为什么：人要打开公网看板，链接必须用粘贴 URL 的 origin 或 web_base_url，不能把内网 API 地址当入口。 */
export function buildDashUrl(source: string, webBaseUrl: string, path: string | undefined, uid: string, orgId: number | undefined, from: string, to: string): string {
  const origin = sourceOrigin(source) ?? webBaseUrl;
  const dashPath = path && path.length > 0 ? path : `/d/${uid}`;
  const url = new URL(dashPath, `${trimSlash(origin)}/`);
  if (orgId !== undefined) {
    url.searchParams.set("orgId", String(orgId));
  }
  url.searchParams.set("from", from);
  url.searchParams.set("to", to);
  return url.toString();
}

function sourceOrigin(source: string): string | undefined {
  try {
    return new URL(source).origin;
  } catch {
    return undefined;
  }
}

function trimSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

/** 为什么：折叠 row 把 panel 藏在 nested 数组里，不展开就读不到值。 */
export function collectPanels(panels: GrafanaPanel[] | undefined): GrafanaPanel[] {
  const result: GrafanaPanel[] = [];
  for (const panel of panels ?? []) {
    if (panel.type === "row") {
      result.push(...collectPanels(panel.panels));
      continue;
    }
    result.push(panel);
  }
  return result;
}

export function visibleTargets(panel: GrafanaPanel): GrafanaTarget[] {
  return (panel.targets ?? []).filter((target) => target.hide !== true);
}

export function buildQueryBody(panel: GrafanaPanel, from: string, to: string): GrafanaQueryBody {
  const targets = visibleTargets(panel);
  if (targets.length === 0) {
    throw new Error(`grafana panel ${panelLabel(panel)} has no query`);
  }

  return {
    from,
    to,
    queries: targets.map((target, index) => toQueryItem(panel, target, index)),
  };
}

export function buildPanelQuery(config: GrafanaConfig, panel: GrafanaPanel, from: string, to: string, orgId?: number): GrafanaQuerySpec {
  return {
    method: "POST",
    url: `${config.baseUrl}/api/ds/query`,
    orgId,
    body: buildQueryBody(panel, from, to),
  };
}

export function readTargetSummaries(body: GrafanaQueryBody, response: GrafanaQueryResponse): TargetSummary[] {
  const results = response.results;
  if (!results) {
    throw new Error("grafana query response missing results");
  }

  return body.queries.map((query) => {
    const result = results[query.refId];
    if (!result) {
      throw new Error(`grafana query ${query.refId} missing result`);
    }
    if (result.error) {
      throw new Error(`grafana query ${query.refId}: ${result.error}`);
    }
    if (result.status !== undefined && result.status >= 400) {
      throw new Error(`grafana query ${query.refId} status ${result.status}`);
    }
    const series = summarizeFrames(result.frames ?? []);
    return {
      refId: query.refId,
      expr: query.expr,
      series_count: series.length,
      series,
    };
  });
}

export function summarizeFrames(frames: GrafanaFrame[]): SeriesSummary[] {
  const series: SeriesSummary[] = [];
  for (const frame of frames) {
    const fields = frame.schema?.fields ?? [];
    const values = frame.data?.values ?? [];
    const timeIndex = fields.findIndex((field) => field.type === "time");
    const timeValues = timeIndex >= 0 ? values[timeIndex] : undefined;

    fields.forEach((field, index) => {
      if (field.type !== "number") {
        return;
      }
      const points = values[index] ?? [];
      const summary = summarizePoints(points, timeValues);
      const labels = field.labels ?? {};
      series.push({
        name: field.config?.displayNameFromDS ?? frame.schema?.name ?? formatLabels(labels),
        labels,
        sample_count: summary.sample_count,
        last: summary.last,
        last_time: summary.last_time,
        max: summary.max,
      });
    });
  }
  return series;
}

function toQueryItem(panel: GrafanaPanel, target: GrafanaTarget, index: number): GrafanaQueryItem {
  const expr = target.expr?.trim();
  if (!expr) {
    throw new Error(`grafana panel ${panelLabel(panel)} target missing expr`);
  }

  const ds = promDs(panel, target);
  const range = target.range === true || (target.instant !== true && isRangePanel(panel.type));
  return {
    refId: target.refId?.trim() || String.fromCharCode(65 + index),
    datasource: ds,
    expr,
    range,
    instant: target.instant === true || !range,
    intervalMs: DEFAULT_INTERVAL_MS,
    maxDataPoints: panel.maxDataPoints ?? DEFAULT_MAX_PTS,
    legendFormat: target.legendFormat,
  };
}

function promDs(panel: GrafanaPanel, target: GrafanaTarget): { type: "prometheus"; uid: string } {
  const ds = target.datasource ?? panel.datasource;
  if (!ds || typeof ds === "string" || ds.type !== "prometheus" || !ds.uid) {
    throw new Error(`grafana panel ${panelLabel(panel)} datasource is not prometheus`);
  }
  return { type: "prometheus", uid: ds.uid };
}

function isRangePanel(type: string | undefined): boolean {
  return type === "timeseries" || type === "graph" || type === "heatmap" || type === "bargauge";
}

function summarizePoints(points: unknown[], timeValues: unknown[] | undefined): Pick<SeriesSummary, "sample_count" | "last" | "last_time" | "max"> {
  let sampleCount = 0;
  let last: number | undefined;
  let lastTime: number | undefined;
  let max: number | undefined;

  points.forEach((point, index) => {
    if (typeof point !== "number" || !Number.isFinite(point)) {
      return;
    }
    sampleCount += 1;
    last = point;
    const time = timeValues?.[index];
    if (typeof time === "number" && Number.isFinite(time)) {
      lastTime = time;
    }
    max = max === undefined ? point : Math.max(max, point);
  });

  return {
    sample_count: sampleCount,
    last,
    last_time: Number.isFinite(lastTime) ? lastTime : undefined,
    max,
  };
}

function formatLabels(labels: Record<string, string>): string {
  const pairs = Object.entries(labels).map(([key, value]) => `${key}="${value}"`);
  return pairs.length === 0 ? "{}" : `{${pairs.join(",")}}`;
}

function panelLabel(panel: GrafanaPanel): string {
  if (panel.id !== undefined) {
    return String(panel.id);
  }
  return panel.title ?? "<unknown>";
}

function parseOrgId(value: string | null): number | undefined {
  if (!value) {
    return undefined;
  }
  const orgId = Number(value);
  if (!Number.isInteger(orgId)) {
    throw new Error(`grafana orgId is invalid: ${value}`);
  }
  return orgId;
}
