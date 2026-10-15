import { buildPanelQuery } from "./query";
import type {
  GrafanaConfig,
  GrafanaDashPayload,
  GrafanaGetSpec,
  GrafanaPanel,
  GrafanaQueryResponse,
  GrafanaQuerySpec,
} from "./types";

export interface GrafanaClient {
  getDashboard(uid: string, orgId?: number): Promise<GrafanaDashPayload>;
  queryPanel(panel: GrafanaPanel, from: string, to: string, orgId?: number): Promise<GrafanaQueryResponse>;
}

type FetchLike = (input: string, init?: RequestInit) => Promise<Response>;

/** 为什么：Grafana client 只封装鉴权和 HTTP，方便测试替换 fetch。 */
export function createGrafanaClient(config: GrafanaConfig, fetcher: FetchLike = fetch): GrafanaClient {
  return {
    async getDashboard(uid, orgId) {
      const spec: GrafanaGetSpec = {
        method: "GET",
        url: `${config.baseUrl}/api/dashboards/uid/${encodeURIComponent(uid)}`,
        orgId,
      };
      return await requestJson<GrafanaDashPayload>(config, fetcher, spec);
    },
    async queryPanel(panel, from, to, orgId) {
      const spec = buildPanelQuery(config, panel, from, to, orgId);
      return await requestJson<GrafanaQueryResponse>(config, fetcher, spec);
    },
  };
}

async function requestJson<T>(config: GrafanaConfig, fetcher: FetchLike, spec: GrafanaGetSpec | GrafanaQuerySpec): Promise<T> {
  const headers: Record<string, string> = {
    Accept: "application/json",
    Authorization: `Basic ${btoa(`${config.username}:${config.password}`)}`,
  };
  if (spec.orgId !== undefined) {
    headers["X-Grafana-Org-Id"] = String(spec.orgId);
  }

  const init: RequestInit = spec.method === "POST"
    ? {
        method: spec.method,
        headers: { ...headers, "Content-Type": "application/json" },
        body: JSON.stringify(spec.body),
      }
    : { method: spec.method, headers };

  const response = await fetcher(spec.url, init);
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`grafana http ${response.status}: ${text}`);
  }
  return parseJson<T>(text);
}

function parseJson<T>(text: string): T {
  try {
    return JSON.parse(text) as T;
  } catch {
    throw new Error(`grafana response is not json: ${text}`);
  }
}
