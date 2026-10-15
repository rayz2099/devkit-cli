import { describe, expect, test } from "bun:test";
import { analyzeDashboard } from "../src/grafana/analyze";
import { createGrafanaClient } from "../src/grafana/api";
import { normalizeGrafanaConfig } from "../src/grafana/config";
import { formatGrafanaResult } from "../src/grafana/format";
import {
  buildDashUrl,
  buildQueryBody,
  collectPanels,
  parseDashRef,
  readTargetSummaries,
  summarizeFrames,
} from "../src/grafana/query";
import type { AnalyzeResult, GrafanaConfig, GrafanaDashPayload, GrafanaPanel } from "../src/grafana/types";
import { normalizeRawConfig } from "../src/uptrace/config";

const config: GrafanaConfig = {
  baseUrl: "http://10.0.48.10:3000",
  username: "admin",
  password: "secret",
};

const promDs = { type: "prometheus", uid: "prom-uid" };

describe("Grafana config", () => {
  test("normalizes grafana config and adds http scheme", () => {
    const appConfig = normalizeRawConfig({
      grafana: {
        base_url: "10.0.48.10:3000",
        username: "admin",
        password: "secret",
      },
    });

    expect(appConfig.grafana).toEqual({ ...config, webBaseUrl: undefined });
  });

  test("rejects incomplete grafana config", () => {
    expect(() => normalizeGrafanaConfig({ base_url: "http://10.0.48.10:3000", username: "admin" })).toThrow("grafana.password");
  });
});

describe("Grafana dashboard ref", () => {
  test("parses public dashboard URL uid orgId and time", () => {
    const ref = parseDashRef("https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/5Lia5Yqh5o6l5Y-j?orgId=1&from=now-1h&to=now");

    expect(ref).toEqual({
      uid: "c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a",
      orgId: 1,
      from: "now-1h",
      to: "now",
    });
  });

  test("parses bare uid", () => {
    expect(parseDashRef("c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a")).toEqual({
      uid: "c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a",
    });
  });

  test("rejects URL without dashboard uid", () => {
    expect(() => parseDashRef("https://grafana.dtactivity.cn/dashboards")).toThrow("missing dashboard uid");
  });

  test("builds public dashboard URL from pasted origin", () => {
    const url = buildDashUrl(
      "https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz?orgId=1",
      "http://10.0.48.10:3000",
      "/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz",
      "c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a",
      1,
      "now-6h",
      "now",
    );
    expect(url).toBe("https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz?orgId=1&from=now-6h&to=now");
  });
});

describe("Grafana query builders", () => {
  test("collects nested row panels", () => {
    const panels = collectPanels([
      { type: "row", title: "row", panels: [{ id: 2, type: "timeseries", title: "nested" }] },
      { id: 1, type: "timeseries", title: "top" },
    ]);

    expect(panels.map((panel) => panel.title)).toEqual(["nested", "top"]);
  });

  test("builds prometheus query and skips hidden targets", () => {
    const body = buildQueryBody(samplePanel(), "now-6h", "now");

    expect(body.from).toBe("now-6h");
    expect(body.to).toBe("now");
    expect(body.queries).toEqual([
      {
        refId: "A",
        datasource: { type: "prometheus", uid: "prom-uid" },
        expr: "sum(irate(http_biz_count_total[5m])) by (path)",
        range: true,
        instant: false,
        intervalMs: 15000,
        maxDataPoints: 1100,
        legendFormat: "{{path}}",
      },
    ]);
  });

  test("rejects non-prometheus datasource", () => {
    const panel: GrafanaPanel = {
      id: 9,
      type: "timeseries",
      title: "mysql",
      datasource: { type: "mysql", uid: "mysql" },
      targets: [{ refId: "A", expr: "select 1", range: true }],
    };

    expect(() => buildQueryBody(panel, "now-6h", "now")).toThrow("datasource is not prometheus");
  });
});

describe("Grafana frames", () => {
  test("summarizes last and max and skips nulls", () => {
    const series = summarizeFrames([
      {
        schema: {
          name: "/napi/blog/detail/-DEFAULT",
          fields: [
            { name: "Time", type: "time" },
            { name: "Value", type: "number", labels: { path: "/napi/blog/detail/", biz: "DEFAULT" }, config: { displayNameFromDS: "/napi/blog/detail/-DEFAULT" } },
          ],
        },
        data: { values: [[1, 2, 3], [10, null, 25]] },
      },
    ]);

    expect(series).toEqual([
      {
        name: "/napi/blog/detail/-DEFAULT",
        labels: { path: "/napi/blog/detail/", biz: "DEFAULT" },
        sample_count: 2,
        last: 25,
        last_time: 3,
        max: 25,
      },
    ]);
  });

  test("fails when query result has error", () => {
    expect(() =>
      readTargetSummaries(
        { from: "now-6h", to: "now", queries: [{ refId: "A", datasource: { type: "prometheus", uid: "prom-uid" }, expr: "up", range: true, instant: false, intervalMs: 15000, maxDataPoints: 1100 }] },
        { results: { A: { error: "timeout", status: 500 } } },
      ),
    ).toThrow("grafana query A: timeout");
  });
});

describe("Grafana analyze", () => {
  test("loads dashboard and queries prometheus panels", async () => {
    const payload = sampleDash();
    const client = createGrafanaClient(config, async (url, init) => {
      if (url === "http://10.0.48.10:3000/api/dashboards/uid/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a") {
        expect(init?.headers && (init.headers as Record<string, string>)["X-Grafana-Org-Id"]).toBe("1");
        return jsonResponse(payload);
      }
      if (url === "http://10.0.48.10:3000/api/ds/query") {
        const body = JSON.parse(String(init?.body)) as { queries: { refId: string }[] };
        expect(body.queries.map((item) => item.refId)).toEqual(["A"]);
        return jsonResponse({
          results: {
            A: {
              status: 200,
              frames: [
                {
                  schema: {
                    name: "biz",
                    fields: [
                      { name: "Time", type: "time" },
                      { name: "Value", type: "number", labels: { path: "/x" }, config: { displayNameFromDS: "/x" } },
                    ],
                  },
                  data: { values: [[1000, 2000], [1.5, 4]] },
                },
              ],
            },
          },
        });
      }
      throw new Error(url);
    });

    const result = await analyzeDashboard(client, {
      source: "https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz?orgId=1",
      webBaseUrl: "http://10.0.48.10:3000",
    });

    expect(result.title).toBe("业务接口");
    expect(result.uid).toBe("c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a");
    expect(result.url).toBe("https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz?orgId=1&from=now-6h&to=now");
    expect(result.time).toEqual({ from: "now-6h", to: "now" });
    expect(result.panels[0]?.targets[0]?.series[0]).toEqual({
      name: "/x",
      labels: { path: "/x" },
      sample_count: 2,
      last: 4,
      last_time: 2000,
      max: 4,
    });
  });

  test("flag time overrides URL and dashboard time", async () => {
    const client = createGrafanaClient(config, async (url) => {
      if (url.includes("/api/dashboards/uid/")) {
        return jsonResponse(sampleDash());
      }
      return jsonResponse({ results: { A: { status: 200, frames: [] } } });
    });

    const result = await analyzeDashboard(client, {
      source: "c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a",
      webBaseUrl: "http://10.0.48.10:3000",
      from: "now-15m",
      to: "now",
    });

    expect(result.time).toEqual({ from: "now-15m", to: "now" });
  });
});

describe("Grafana format", () => {
  test("formats agent output without raw points", () => {
    const text = formatGrafanaResult(sampleResult(), "agent");
    expect(text).toContain('"title": "业务接口"');
    expect(text).toContain('"last": 4');
    expect(text).toContain('"max": 8');
    expect(text).not.toContain("values");
  });

  test("formats human summary", () => {
    const text = formatGrafanaResult(sampleResult(), "human");
    expect(text).toContain("Grafana: 业务接口");
    expect(text).toContain("URL: https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz?orgId=1&from=now-6h&to=now");
    expect(text).toContain("Time: now-6h → now");
    expect(text).toContain("#1 业务接口看板");
    expect(text).toContain("last=4 max=8 samples=3");
  });

  test("human output ranks top series by max", () => {
    const result = sampleResult();
    result.panels[0]!.targets[0]!.series_count = 12;
    result.panels[0]!.targets[0]!.series = Array.from({ length: 12 }, (_, index) => ({
      name: `s${index}`,
      labels: {},
      sample_count: 1,
      last: index,
      max: index,
    }));
    const text = formatGrafanaResult(result, "human");
    expect(text).toContain("top 10 by max:");
    expect(text).toContain("s11  last=11 max=11 samples=1");
    expect(text).not.toContain("s0  last=0");
  });
});

function samplePanel(): GrafanaPanel {
  return {
    id: 1,
    type: "timeseries",
    title: "业务接口看板",
    datasource: promDs,
    targets: [
      { refId: "A", expr: "sum(irate(http_biz_count_total[5m])) by (path)", range: true, legendFormat: "{{path}}", datasource: promDs },
      { refId: "B", expr: "sum(irate(http_biz_count_total[5m])) by (biz)", range: true, hide: true, datasource: promDs },
    ],
  };
}

function sampleDash(): GrafanaDashPayload {
  return {
    meta: { url: "/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz" },
    dashboard: {
      title: "业务接口",
      uid: "c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a",
      time: { from: "now-6h", to: "now" },
      panels: [samplePanel()],
    },
  };
}

function sampleResult(): AnalyzeResult {
  return {
    title: "业务接口",
    uid: "c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a",
    url: "https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz?orgId=1&from=now-6h&to=now",
    orgId: 1,
    time: { from: "now-6h", to: "now" },
    panels: [
      {
        id: 1,
        title: "业务接口看板",
        type: "timeseries",
        targets: [
          {
            refId: "A",
            expr: "up",
            series_count: 1,
            series: [{ name: "/x", labels: { path: "/x" }, sample_count: 3, last: 4, last_time: 3, max: 8 }],
          },
        ],
      },
    ],
    dashboard: sampleDash(),
    queryBodies: [],
  };
}

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" } });
}
