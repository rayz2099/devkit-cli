import { describe, expect, test } from "bun:test";
import { createGraylogClient } from "../src/graylog/api";
import { normalizeGraylogConfig } from "../src/graylog/config";
import { formatGraylogAggregate, formatGraylogFields, formatGraylogResult } from "../src/graylog/format";
import { buildAggregateRequest, buildSearchRequest, toGraylogSearchInput } from "../src/graylog/query";
import type { GraylogConfig, GraylogSearchResponse } from "../src/graylog/types";
import { normalizeRawConfig } from "../src/uptrace/config";

const config: GraylogConfig = {
  baseUrl: "http://127.0.0.1:9000",
  username: "admin",
  password: "admin",
};

describe("Graylog config", () => {
  test("normalizes graylog config and adds http scheme", () => {
    const appConfig = normalizeRawConfig({
      graylog: {
        base_url: "127.0.0.1:9000",
        username: "admin",
        password: "admin",
      },
    });

    expect(appConfig.graylog).toEqual(config);
  });

  test("rejects incomplete graylog config", () => {
    expect(() => normalizeGraylogConfig({ base_url: "127.0.0.1:9000", username: "admin" })).toThrow("graylog.password");
  });
});

describe("Graylog query", () => {
  test("builds relative search request from query text", () => {
    const input = toGraylogSearchInput({
      source: "app:billing AND level = 3",
      relative: 28800,
      limit: 20,
    });
    const request = buildSearchRequest(config, input);

    expect(request.url).toBe("http://127.0.0.1:9000/api/search/universal/relative");
    expect(request.params.get("query")).toBe("app:billing AND level = 3");
    expect(request.params.get("range")).toBe("28800");
    expect(request.params.get("limit")).toBe("20");
  });

  test("builds absolute search request with full graylog parameters", () => {
    const input = toGraylogSearchInput({
      source: "app:billing AND level:3",
      from: "2026-06-01T10:00:00+08:00",
      to: "2026-06-01T11:00:00+08:00",
      fields: "timestamp,source,logger_name,trace_id,message",
      filter: "streams:abc",
      offset: 10,
      decorate: true,
    });
    const request = buildSearchRequest(config, input);

    expect(request.url).toBe("http://127.0.0.1:9000/api/search/universal/absolute");
    expect(request.params.get("from")).toBe("2026-06-01T10:00:00+08:00");
    expect(request.params.get("to")).toBe("2026-06-01T11:00:00+08:00");
    expect(request.params.get("fields")).toBe("timestamp,source,logger_name,trace_id,message");
    expect(request.params.get("filter")).toBe("streams:abc");
    expect(request.params.get("offset")).toBe("10");
    expect(request.params.get("decorate")).toBe("true");
  });

  test("parses graylog search URL into query and relative range", () => {
    const input = toGraylogSearchInput({
      source: "https://log.example.com/search?q=app%3Abilling+AND+level%3A3&rangetype=relative&relative=28800",
      limit: 50,
    });

    expect(input.query).toBe("app:billing AND level:3");
    expect(input.relative).toBe(28800);
    expect(input.limit).toBe(50);
  });

  test("parses absolute graylog URL", () => {
    const input = toGraylogSearchInput({
      source: "https://log.example.com/search?q=app%3Abilling&rangetype=absolute&from=2026-06-01T10%3A00%3A00.000Z&to=2026-06-01T11%3A00%3A00.000Z",
    });

    expect(input.rangeType).toBe("absolute");
    expect(input.from).toBe("2026-06-01T10:00:00.000Z");
    expect(input.to).toBe("2026-06-01T11:00:00.000Z");
  });

  test("builds server side aggregate request", () => {
    const request = buildAggregateRequest(config, {
      query: "app:billing AND level:3",
      rangeType: "relative",
      relative: 28800,
      field: "USER_IP",
      limit: 10,
    });

    expect(request.url).toBe("http://127.0.0.1:9000/api/views/search/sync");
    expect(request.params?.get("timeout")).toBe("60000");
    expect(request.body).toMatchObject({
      queries: [
        {
          query: { query_string: "app:billing AND level:3" },
          timerange: { type: "relative", range: 28800 },
          search_types: [
            {
              type: "pivot",
              row_groups: [{ type: "values", field: "USER_IP", limit: 10 }],
              series: [{ type: "count", id: "count()" }],
              rollup: true,
            },
          ],
        },
      ],
    });
  });
});

describe("Graylog API and formatting", () => {
  test("client sends basic auth and returns response", async () => {
    const client = createGraylogClient(config, async (input, init) => {
      const url = new URL(input);
      expect(`${url.origin}${url.pathname}`).toBe("http://127.0.0.1:9000/api/search/universal/relative");
      expect(url.searchParams.get("query")).toBe("app:billing AND level = 3");
      expect(url.searchParams.get("range")).toBe("28800");
      expect(url.searchParams.get("limit")).toBe("10");
      expect(url.searchParams.get("sort")).toBe("timestamp:desc");
      expect(init?.method).toBe("GET");
      expect(init?.headers).toEqual({
        Accept: "application/json",
        Authorization: `Basic ${btoa("admin:admin")}`,
        "X-Requested-By": "olly-cli",
      });
      return new Response(JSON.stringify(sampleResponse), { status: 200 });
    });

    const input = toGraylogSearchInput({ source: "app:billing AND level = 3", relative: 28800, limit: 10 });
    await expect(client.search(input)).resolves.toEqual(sampleResponse);
  });

  test("client parses aggregate pivot rows", async () => {
    const client = createGraylogClient(config, async (input, init) => {
      expect(input).toBe("http://127.0.0.1:9000/api/views/search/sync?timeout=60000");
      expect(init?.method).toBe("POST");
      return new Response(JSON.stringify(sampleAggregateResponse), { status: 200 });
    });

    const result = await client.aggregate({
      query: "app:billing AND level:3",
      rangeType: "relative",
      relative: 28800,
      field: "USER_IP",
      limit: 2,
    });

    expect(result.rows).toEqual([
      { key: "220.181.51.116", count: 535 },
      { key: "220.181.51.113", count: 498 },
    ]);
    expect(result.total).toBe(102634);
  });

  test("formats agent output as log list by default", () => {
    const text = formatGraylogResult(sampleResponse, "agent", {
      query: "app:billing AND level = 3",
      rangeType: "relative",
      relative: 28800,
    });

    expect(text).toContain('"query": "app:billing AND level = 3"');
    expect(text).toContain('"total_results": 2');
    expect(text).toContain('"messages"');
    expect(text).toContain('"logger_summary"');
    expect(text).toContain('"trace_ids"');
    expect(text).toContain('"abc123"');
  });

  test("formats agent output grouped by requested field", () => {
    const text = formatGraylogResult(sampleResponse, "agent", {
      query: "app:billing AND level = 3",
      rangeType: "relative",
      relative: 28800,
      groupBy: "logger_name",
    });

    expect(text).toContain('"group_by": "logger_name"');
    expect(text).toContain('"groups"');
    expect(text).toContain('"logger_name": "com.example.OrderService"');
    expect(text).toContain('"count": 2');
    expect(text).toContain('"trace_ids"');
  });

  test("marks grouped human output as sampled when total is larger than returned messages", () => {
    const text = formatGraylogResult({ ...sampleResponse, total_results: 102638 }, "human", {
      query: "app:billing AND level = 3",
      rangeType: "relative",
      relative: 28800,
      groupBy: "USER_IP",
    });

    expect(text).toContain("Graylog groups(sampled)");
    expect(text).toContain("returned=2");
    expect(text).toContain("not full-result aggregation");
    expect(text).toContain("missing=2");
  });

  test("formats human output as readable logs by default", () => {
    const text = formatGraylogResult(sampleResponse, "human", {
      query: "app:billing AND level = 3",
      rangeType: "relative",
      relative: 28800,
    });

    expect(text).toContain("Graylog logs: total=2");
    expect(text).toContain("2026-06-01T10:00:00.000Z node-1 [3] com.example.OrderService trace=abc123");
    expect(text).toContain("RPC timeout");
  });

  test("formats human fields list", () => {
    const text = formatGraylogFields(sampleResponse, "human", {
      query: "app:billing AND level = 3",
      rangeType: "relative",
      relative: 28800,
    });

    expect(text).toContain("Graylog fields: count=8");
    expect(text).toContain("- logger_name");
    expect(text).toContain("- trace_id");
  });

  test("formats agent fields list with grouping hints", () => {
    const text = formatGraylogFields(sampleResponse, "agent", {
      query: "app:billing AND level = 3",
      rangeType: "relative",
      relative: 28800,
    });

    expect(text).toContain('"fields"');
    expect(text).toContain('"logger_name"');
    expect(text).toContain('"suggested_group_by"');
  });

  test("formats aggregate as key count rows", () => {
    const text = formatGraylogAggregate({
      query: "app:billing AND level:3",
      field: "USER_IP",
      total: 102634,
      rows: [
        { key: "220.181.51.116", count: 535 },
        { key: "220.181.51.113", count: 498 },
      ],
    }, "human");

    expect(text).toContain("Graylog aggregate: field=USER_IP total=102634 rows=2");
    expect(text).toContain("220.181.51.116\t535");
  });
});

const sampleResponse: GraylogSearchResponse = {
  total_results: 2,
  fields: ["timestamp", "source", "message", "level", "logger_name", "trace_id", "app", "facility"],
  messages: [
    {
      message: {
        timestamp: "2026-06-01T10:00:00.000Z",
        message: "RPC timeout",
        source: "node-1",
        logger_name: "com.example.OrderService",
        trace_id: "abc123",
        level: 3,
      },
    },
    {
      message: {
        timestamp: "2026-06-01T10:00:01.000Z",
        message: "RPC timeout again",
        source: "node-2",
        logger_name: "com.example.OrderService",
        trace_id: "def456",
        level: 3,
      },
    },
  ],
};

const sampleAggregateResponse = {
  results: {
    "query-1": {
      search_types: {
        "pivot-1": {
          total: 102634,
          rows: [
            {
              key: ["220.181.51.116"],
              values: [{ key: ["count()"], value: 535 }],
              source: "leaf",
            },
            {
              key: ["220.181.51.113"],
              values: [{ key: ["count()"], value: 498 }],
              source: "leaf",
            },
            {
              key: [],
              values: [{ key: ["count()"], value: 102634 }],
              source: "non-leaf",
            },
          ],
        },
      },
    },
  },
};
