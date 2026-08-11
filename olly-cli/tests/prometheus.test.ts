import { describe, expect, test } from "bun:test";
import { DEFAULT_PROMETHEUS_BASE_URL, normalizePrometheusConfig } from "../src/prometheus/config";
import { createPrometheusClient, readPrometheusEnvelope } from "../src/prometheus/api";
import { formatPrometheusResult } from "../src/prometheus/format";
import { buildBuildInfoRequest, buildLabelValuesRequest, buildLabelsRequest, buildQueryRangeRequest, buildQueryRequest, buildReadyRequest, buildSeriesRequest } from "../src/prometheus/query";
import type { PrometheusConfig, PrometheusEnvelope } from "../src/prometheus/types";
import { normalizeRawConfig } from "../src/uptrace/config";

const config: PrometheusConfig = {
  baseUrl: "http://127.0.0.1:9090",
  defaultStep: "60s",
  defaultTimeout: "30s",
};

describe("Prometheus config", () => {
  test("normalizes optional prometheus config without requiring uptrace", () => {
    const appConfig = normalizeRawConfig({
      prometheus: {
        base_url: "http://prometheus.example:9090",
        default_step: "15s",
      },
    });

    expect(appConfig.prometheus).toEqual({
      baseUrl: "http://prometheus.example:9090",
      defaultStep: "15s",
      defaultTimeout: undefined,
    });
    expect(appConfig.uptrace).toBeUndefined();
  });

  test("uses built-in prometheus defaults when config is missing", () => {
    expect(normalizePrometheusConfig(undefined)).toEqual({
      baseUrl: DEFAULT_PROMETHEUS_BASE_URL,
      defaultStep: undefined,
      defaultTimeout: undefined,
    });
  });
});

describe("Prometheus request builders", () => {
  test("builds query_range POST request with long promql and timezone timestamps", () => {
    const promql = `100 * (1 - node_memory_MemAvailable_bytes{job="node",instance="192.0.2.10"} /
    node_memory_MemTotal_bytes{job="node",instance="192.0.2.10"})`;
    const request = buildQueryRangeRequest(config, {
      query: promql,
      start: "2026-04-22T10:00:00+08:00",
      end: "2026-04-22T11:00:00+08:00",
      step: "60s",
      timeout: "45s",
      limit: 100,
    });

    expect(request.method).toBe("POST");
    expect(request.url).toBe("http://127.0.0.1:9090/api/v1/query_range");
    expect(request.body.get("query")).toBe(promql);
    expect(request.body.get("start")).toBe("2026-04-22T10:00:00+08:00");
    expect(request.body.get("end")).toBe("2026-04-22T11:00:00+08:00");
    expect(request.body.get("step")).toBe("60s");
    expect(request.body.get("timeout")).toBe("45s");
    expect(request.body.get("limit")).toBe("100");
  });

  test("builds instant query request and applies default timeout", () => {
    const request = buildQueryRequest(config, {
      query: "up",
      time: "2026-04-22T10:00:00+08:00",
    });

    expect(request.url).toBe("http://127.0.0.1:9090/api/v1/query");
    expect(request.body.get("query")).toBe("up");
    expect(request.body.get("time")).toBe("2026-04-22T10:00:00+08:00");
    expect(request.body.get("timeout")).toBe("30s");
  });

  test("builds label and series metadata requests", () => {
    const labels = buildLabelsRequest(config, { start: "1", end: "2", matches: ["up"] });
    expect(labels.url).toBe("http://127.0.0.1:9090/api/v1/labels");
    expect(labels.body.getAll("match[]")).toEqual(["up"]);

    const values = buildLabelValuesRequest(config, "job", { matches: ["up"] });
    expect(values.url).toBe("http://127.0.0.1:9090/api/v1/label/job/values");
    expect(values.body.getAll("match[]")).toEqual(["up"]);

    const series = buildSeriesRequest(config, { matches: ["up", "process_start_time_seconds{job=\"prometheus\"}"], limit: 10 });
    expect(series.url).toBe("http://127.0.0.1:9090/api/v1/series");
    expect(series.body.getAll("match[]")).toEqual(["up", "process_start_time_seconds{job=\"prometheus\"}"]);
    expect(series.body.get("limit")).toBe("10");
  });

  test("builds status requests", () => {
    expect(buildReadyRequest(config)).toEqual({
      method: "GET",
      url: "http://127.0.0.1:9090/-/ready",
    });
    expect(buildBuildInfoRequest(config)).toEqual({
      method: "GET",
      url: "http://127.0.0.1:9090/api/v1/status/buildinfo",
    });
  });

  test("rejects missing required range and series arguments", () => {
    expect(() => buildQueryRangeRequest({ baseUrl: config.baseUrl }, { query: "up", start: "1", end: "2" })).toThrow("--step");
    expect(() => buildSeriesRequest(config, { matches: [] })).toThrow("--match");
  });
});

describe("Prometheus API and formatting", () => {
  test("throws prometheus envelope errors with error type", () => {
    const envelope: PrometheusEnvelope<unknown> = {
      status: "error",
      errorType: "bad_data",
      error: "invalid parameter query",
    };

    expect(() => readPrometheusEnvelope(envelope)).toThrow("bad_data: invalid parameter query");
  });

  test("client posts form body and returns response data", async () => {
    const client = createPrometheusClient(config, async (input, init) => {
      expect(input).toBe("http://127.0.0.1:9090/api/v1/query");
      expect(init?.method).toBe("POST");
      expect(init?.headers).toEqual({ "Content-Type": "application/x-www-form-urlencoded" });
      expect(String(init?.body)).toContain("query=up");
      return new Response(JSON.stringify({ status: "success", data: { resultType: "vector", result: [] } }), { status: 200 });
    });

    const data = await client.query({ query: "up" });
    expect(data).toEqual({ resultType: "vector", result: [] });
  });

  test("client reads readiness text without prometheus envelope", async () => {
    const client = createPrometheusClient(config, async (input, init) => {
      expect(input).toBe("http://127.0.0.1:9090/-/ready");
      expect(init?.method).toBe("GET");
      return new Response("Prometheus Server is Ready.", { status: 200 });
    });

    await expect(client.ready()).resolves.toBe("Prometheus Server is Ready.\n");
  });

  test("formats range matrix summary for agent output", () => {
    const text = formatPrometheusResult(
      {
        status: "success",
        data: {
          resultType: "matrix",
          result: [
            {
              metric: { job: "node", instance: "192.0.2.10" },
              values: [
                [1776823200, "10"],
                [1776823260, "15"],
                [1776823320, "20"],
              ],
            },
          ],
        },
      },
      "agent",
      { query: "memory", command: "query range", includeValues: false },
    );

    expect(text).toContain('"series_count": 1');
    expect(text).toContain('"sample_count": 3');
    expect(text).toContain('"min": 10');
    expect(text).toContain('"max": 20');
    expect(text).toContain('"avg": 15');
    expect(text).not.toContain('"values"');
  });
});
