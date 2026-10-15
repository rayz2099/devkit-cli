import type { OutputMode } from "../uptrace/types";

export type GrafanaOutputMode = OutputMode;

export interface GrafanaConfig {
  baseUrl: string;
  webBaseUrl?: string | undefined;
  username: string;
  password: string;
}

export interface RawGrafanaConfig {
  base_url?: string | undefined;
  web_base_url?: string | undefined;
  username?: string | undefined;
  password?: string | undefined;
}

export interface DashRef {
  uid: string;
  orgId?: number | undefined;
  from?: string | undefined;
  to?: string | undefined;
}

export interface AnalyzeInput {
  source: string;
  webBaseUrl: string;
  from?: string | undefined;
  to?: string | undefined;
  orgId?: number | undefined;
}

export interface GrafanaDs {
  type?: string | undefined;
  uid?: string | undefined;
}

export interface GrafanaTarget {
  refId?: string | undefined;
  expr?: string | undefined;
  hide?: boolean | undefined;
  range?: boolean | undefined;
  instant?: boolean | undefined;
  legendFormat?: string | undefined;
  datasource?: GrafanaDs | string | undefined;
}

export interface GrafanaPanel {
  id?: number | undefined;
  title?: string | undefined;
  type?: string | undefined;
  datasource?: GrafanaDs | string | undefined;
  targets?: GrafanaTarget[] | undefined;
  panels?: GrafanaPanel[] | undefined;
  maxDataPoints?: number | undefined;
  interval?: string | undefined;
}

export interface GrafanaDash {
  title?: string | undefined;
  uid?: string | undefined;
  time?: { from?: string | undefined; to?: string | undefined } | undefined;
  panels?: GrafanaPanel[] | undefined;
}

export interface GrafanaDashPayload {
  meta?: { url?: string | undefined } | undefined;
  dashboard: GrafanaDash;
}

export interface GrafanaQuerySpec {
  method: "POST";
  url: string;
  orgId?: number | undefined;
  body: GrafanaQueryBody;
}

export interface GrafanaQueryBody {
  from: string;
  to: string;
  queries: GrafanaQueryItem[];
}

export interface GrafanaQueryItem {
  refId: string;
  datasource: { type: "prometheus"; uid: string };
  expr: string;
  range: boolean;
  instant: boolean;
  intervalMs: number;
  maxDataPoints: number;
  legendFormat?: string | undefined;
}

export interface GrafanaGetSpec {
  method: "GET";
  url: string;
  orgId?: number | undefined;
}

export interface SeriesSummary {
  name: string;
  labels: Record<string, string>;
  sample_count: number;
  last?: number | undefined;
  last_time?: number | undefined;
  max?: number | undefined;
}

export interface TargetSummary {
  refId: string;
  expr: string;
  series_count: number;
  series: SeriesSummary[];
}

export interface PanelSummary {
  id?: number | undefined;
  title: string;
  type: string;
  targets: TargetSummary[];
}

export interface AnalyzeResult {
  title: string;
  uid: string;
  url?: string | undefined;
  orgId?: number | undefined;
  time: { from: string; to: string };
  panels: PanelSummary[];
  dashboard: GrafanaDashPayload;
  queryBodies: GrafanaQueryBody[];
}

export interface GrafanaFrameField {
  name?: string | undefined;
  type?: string | undefined;
  labels?: Record<string, string> | undefined;
  config?: { displayNameFromDS?: string | undefined } | undefined;
}

export interface GrafanaFrame {
  schema?: {
    name?: string | undefined;
    refId?: string | undefined;
    fields?: GrafanaFrameField[] | undefined;
  } | undefined;
  data?: { values?: unknown[][] | undefined } | undefined;
}

export interface GrafanaQueryResponse {
  results?: Record<string, { status?: number | undefined; error?: string | undefined; frames?: GrafanaFrame[] | undefined }> | undefined;
}
