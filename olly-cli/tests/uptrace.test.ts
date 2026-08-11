import { describe, expect, test } from "bun:test";
import { HELP_TEXT, PROMETHEUS_HELP_TEXT, UPTRACE_HELP_TEXT, helpTextFor, shouldShowHelp } from "../src/help";
import { extractTraceSpans } from "../src/uptrace/api";
import { normalizeRawConfig } from "../src/uptrace/config";
import { buildContext, formatContextAgent } from "../src/uptrace/context";
import { buildGroupStatsRequest, buildGroupsRequest, buildSpansRequest, buildTraceRequest, parseTimeRange } from "../src/uptrace/query";
import { parseTraceInput } from "../src/uptrace/trace-id";
import { buildTraceTree } from "../src/uptrace/trace-tree";
import type { UptraceConfig, UptraceSpan } from "../src/uptrace/types";

const config: UptraceConfig = {
  baseUrl: "https://uptrace.example.com",
  webBaseUrl: "https://uptrace.example.com",
  projectId: 2,
  authToken: "token",
  defaultEnv: "loadtest",
  defaultTimeDurSeconds: 10800,
};

describe("help", () => {
  test("shows help before config is required", () => {
    expect(shouldShowHelp([])).toBe(true);
    expect(shouldShowHelp(["--help"])).toBe(true);
    expect(shouldShowHelp(["help"])).toBe(true);
    expect(shouldShowHelp(["uptrace", "-h"])).toBe(true);
    expect(shouldShowHelp(["prometheus", "-h"])).toBe(true);
    expect(shouldShowHelp(["prometheus", "query", "instant", "-h"])).toBe(false);
    expect(HELP_TEXT).toContain("uptrace groups");
    expect(HELP_TEXT).toContain("prometheus query");
    expect(HELP_TEXT).toContain("fish");
  });

  test("selects root command specific help", () => {
    expect(helpTextFor(["prom", "-h"])).toBe(PROMETHEUS_HELP_TEXT);
    expect(helpTextFor(["prometheus", "--help"])).toBe(PROMETHEUS_HELP_TEXT);
    expect(helpTextFor(["uptrace", "-h"])).toBe(UPTRACE_HELP_TEXT);
    expect(helpTextFor(["--help"])).toBe(HELP_TEXT);
    expect(PROMETHEUS_HELP_TEXT).toContain("prometheus query range");
    expect(PROMETHEUS_HELP_TEXT).not.toContain("Uptrace internal API");
  });
});

describe("parseTraceInput", () => {
  test("parses a bare trace id", () => {
    expect(parseTraceInput("e4f1e0bcd6a1ac296661a3e6ea5507c9")).toEqual({
      traceId: "e4f1e0bcd6a1ac296661a3e6ea5507c9",
    });
  });

  test("parses an Uptrace trace url with project id", () => {
    expect(parseTraceInput("https://uptrace.example.com/traces/2/e4f1e0bcd6a1ac296661a3e6ea5507c9")).toEqual({
      projectId: 2,
      traceId: "e4f1e0bcd6a1ac296661a3e6ea5507c9",
    });
  });

  test("parses trace id embedded in log text", () => {
    expect(parseTraceInput("trace_id=e4f1e0bcd6a1ac296661a3e6ea5507c9 cost=1200ms")).toEqual({
      traceId: "e4f1e0bcd6a1ac296661a3e6ea5507c9",
    });
  });

  test("rejects invalid trace input", () => {
    expect(() => parseTraceInput("trace_id=bad")).toThrow("trace id");
  });
});

describe("Uptrace request builders", () => {
  test("converts time_gte and time_dur to API time_start and time_end", () => {
    expect(parseTimeRange({ timeGte: "20260429T060000", timeDurSeconds: 10800 })).toEqual({
      timeGte: "2026-04-29T06:00:00.000Z",
      timeLt: "2026-04-29T09:00:00.000Z",
    });
  });

  test("builds groups request with default service UQL", () => {
    const request = buildGroupsRequest(config, {
      service: "app-gw",
      env: "loadtest",
      timeGte: "20260429T060000",
      timeDurSeconds: 10800,
      limit: 100,
    });

    expect(request.url).toBe("https://uptrace.example.com/internal/v1/tracing/2/groups");
    expect(request.query.get("time_gte")).toBe("2026-04-29T06:00:00.000Z");
    expect(request.query.get("time_lt")).toBe("2026-04-29T09:00:00.000Z");
    expect(request.query.get("system[]")).toBe("all");
    expect(request.query.get("limit")).toBe("100");
    expect(request.query.get("query")).toContain('where service_name = "app-gw"');
    expect(request.headers.Authorization).toBe("Bearer token");
  });

  test("builds internal group stats request with default metric columns", () => {
    const request = buildGroupStatsRequest(config, {
      service: "app-gw",
      env: "loadtest",
      timeGte: "20260429T060000",
    });

    expect(request.url).toBe("https://uptrace.example.com/internal/v1/tracing/2/group-stats");
    expect(request.query.getAll("column[]")).toEqual([
      "per_min(sum(_count))",
      "_error_rate",
      "p50(_duration)",
      "p90(_duration)",
      "p99(_duration)",
    ]);
  });

  test("uses jwt token as Uptrace cookie auth", () => {
    const request = buildGroupsRequest({ ...config, authToken: undefined, jwtToken: "jwt-value" }, {
      service: "app-gw",
      env: "loadtest",
      timeGte: "20260429T060000",
    });

    expect(request.headers.Cookie).toBe("token=jwt-value");
    expect(request.headers.Authorization).toBeUndefined();
  });

  test("normalizes config with jwt token without requiring api auth token", () => {
    const appConfig = normalizeRawConfig({
      uptrace: {
        base_url: "https://uptrace.example.com",
        project_id: 2,
        jwt_token: "jwt-value",
      },
    });

    expect(appConfig.uptrace?.jwtToken).toBe("jwt-value");
    expect(appConfig.uptrace?.authToken).toBeUndefined();
  });

  test("builds spans request for trace id and duration filter", () => {
    const request = buildSpansRequest(config, {
      traceId: "e4f1e0bcd6a1ac296661a3e6ea5507c9",
      durationGte: 10000,
      limit: 50,
      timeStart: "2026-04-29T06:00:00.000Z",
      timeEnd: "2026-04-29T09:00:00.000Z",
    });

    expect(request.url).toBe("https://uptrace.example.com/internal/v1/tracing/2/spans");
    expect(request.query.get("time_gte")).toBe("2026-04-29T06:00:00.000Z");
    expect(request.query.get("time_lt")).toBe("2026-04-29T09:00:00.000Z");
    expect(request.query.get("trace_id")).toBe("e4f1e0bcd6a1ac296661a3e6ea5507c9");
    expect(request.query.get("duration_gte")).toBe("10000");
  });

  test("builds internal trace detail request", () => {
    const request = buildTraceRequest(config, "e4f1e0bcd6a1ac296661a3e6ea5507c9");

    expect(request.url).toBe("https://uptrace.example.com/internal/v1/tracing/2/traces/e4f1e0bcd6a1ac296661a3e6ea5507c9");
    expect(request.query.toString()).toBe("");
  });

  test("flattens internal trace response root tree", () => {
    const spans = extractTraceSpans({
      root: {
        id: "root",
        traceId: "e4f1e0bcd6a1ac296661a3e6ea5507c9",
        name: "GET /*",
        duration: 100,
        children: [
          {
            id: "child",
            parentId: "root",
            traceId: "e4f1e0bcd6a1ac296661a3e6ea5507c9",
            name: "GET /api",
            duration: 80,
            children: [],
          },
        ],
      },
    });

    expect(spans.map((span) => span.id)).toEqual(["root", "child"]);
    expect(spans[0]?.children).toBeUndefined();
  });
});

describe("trace tree and agent context", () => {
  const spans: UptraceSpan[] = [
    {
      id: 1,
      traceId: "e4f1e0bcd6a1ac296661a3e6ea5507c9",
      parentId: 0,
      name: "GET /api/order",
      kind: "server",
      time: "2026-04-29T06:00:00.000Z",
      duration: 500000,
      attrs: {
        service_name: "app-gw",
        http_route: "/api/order",
      },
    },
    {
      id: 2,
      traceId: "e4f1e0bcd6a1ac296661a3e6ea5507c9",
      parentId: 1,
      name: "POST /inventory/reserve",
      kind: "client",
      time: "2026-04-29T06:00:00.100Z",
      duration: 350000,
      attrs: {
        service_name: "app-gw",
        peer_service: "ld-inventory",
      },
    },
    {
      id: 3,
      traceId: "e4f1e0bcd6a1ac296661a3e6ea5507c9",
      parentId: 2,
      name: "SELECT inventory",
      kind: "db",
      time: "2026-04-29T06:00:00.200Z",
      duration: 280000,
      statusCode: "error",
      attrs: {
        service_name: "ld-inventory",
        db_statement: "select * from inventory where sku_id = ?",
      },
    },
  ];

  test("builds parent-child tree and marks slow/error spans", () => {
    const tree = buildTraceTree(spans);

    expect(tree.roots).toHaveLength(1);
    expect(tree.roots[0]?.children[0]?.children[0]?.span.name).toBe("SELECT inventory");
    expect(tree.topSlowSpans[0]?.span.id).toBe(1);
    expect(tree.errorSpans.map((item) => item.span.id)).toEqual([3]);
  });

  test("formats compact context for LLM", () => {
    const context = buildContext({
      config,
      traceInput: "e4f1e0bcd6a1ac296661a3e6ea5507c9",
      spans,
      query: {
        service: "app-gw",
        env: "loadtest",
      },
    });

    const text = formatContextAgent(context);

    expect(text).toContain("trace_id: e4f1e0bcd6a1ac296661a3e6ea5507c9");
    expect(text).toContain("GET /api/order");
    expect(text).toContain("SELECT inventory");
    expect(text).toContain("source_hints:");
    expect(text).toContain("db_statement: select * from inventory where sku_id = ?");
  });
});
