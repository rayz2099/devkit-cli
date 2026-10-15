import type { GraylogAggregateInput, GraylogConfig, GraylogPostSpec, GraylogRequestSpec, GraylogSearchInput, GraylogSearchSource } from "./types";

export const DEFAULT_GRAYLOG_RELATIVE_SECONDS = 28800;
export const DEFAULT_GRAYLOG_LIMIT = 50;
const DEFAULT_GRAYLOG_SORT = "timestamp:desc";

/** 为什么：用户常从浏览器复制 Graylog search URL，CLI 要把它降级成可复用 API 查询。 */
export function toGraylogSearchInput(source: GraylogSearchSource): GraylogSearchInput {
  const parsed = parseSource(source.source);
  const query = parsed.query ?? source.source;
  if (!query || query.length === 0) {
    throw new Error("graylog logs requires query text or Graylog search URL");
  }
  const from = source.from ?? parsed.from;
  const to = source.to ?? parsed.to;
  const keyword = source.keyword ?? parsed.keyword;
  const relative = source.relative ?? parsed.relative;
  const rangeType = source.from || source.to || parsed.rangeType === "absolute"
    ? "absolute"
    : source.keyword || parsed.rangeType === "keyword"
      ? "keyword"
      : "relative";
  if (rangeType === "absolute") {
    requireNonEmpty(from, "--from");
    requireNonEmpty(to, "--to");
  }
  if (rangeType === "keyword") {
    requireNonEmpty(keyword, "--keyword");
  }

  return {
    query,
    rangeType,
    relative: rangeType === "relative" ? relative ?? DEFAULT_GRAYLOG_RELATIVE_SECONDS : undefined,
    from,
    to,
    keyword,
    limit: source.limit,
    sort: source.sort ?? parsed.sort,
    fields: source.fields ?? parsed.fields,
    filter: source.filter ?? parsed.filter,
    offset: source.offset,
    decorate: source.decorate,
  };
}

export function buildSearchRequest(config: GraylogConfig, input: GraylogSearchInput): GraylogRequestSpec {
  const params = new URLSearchParams();
  params.set("query", input.query);
  params.set("limit", String(input.limit ?? DEFAULT_GRAYLOG_LIMIT));
  params.set("sort", input.sort ?? DEFAULT_GRAYLOG_SORT);
  appendOptional(params, "fields", input.fields);
  appendOptional(params, "filter", input.filter);
  appendOptionalNumber(params, "offset", input.offset);
  appendOptionalBool(params, "decorate", input.decorate);
  appendRange(params, input);

  return {
    method: "GET",
    url: `${config.baseUrl}/api/search/universal/${input.rangeType}`,
    params,
  };
}

export function buildAggregateRequest(config: GraylogConfig, input: GraylogAggregateInput): GraylogPostSpec {
  const params = new URLSearchParams();
  params.set("timeout", "60000");
  return {
    method: "POST",
    url: `${config.baseUrl}/api/views/search/sync`,
    params,
    body: {
      queries: [
        {
          id: "query-1",
          query: {
            type: "elasticsearch",
            query_string: input.query,
          },
          timerange: aggregateTimerange(input),
          filter: {
            type: "or",
            filters: [],
          },
          search_types: [
            {
              id: "pivot-1",
              type: "pivot",
              row_groups: [
                {
                  type: "values",
                  field: input.field,
                  limit: input.limit ?? 10,
                },
              ],
              series: [
                {
                  type: "count",
                  id: "count()",
                },
              ],
              column_groups: [],
              rollup: true,
            },
          ],
        },
      ],
    },
  };
}

function parseSource(source: string | undefined): Partial<GraylogSearchInput> {
  if (!source) {
    return {};
  }
  let url: URL;
  try {
    url = new URL(source);
  } catch {
    return {};
  }

  const result: Partial<GraylogSearchInput> = {};
  const query = url.searchParams.get("q") ?? url.searchParams.get("query");
  const rangeType = url.searchParams.get("rangetype") ?? url.searchParams.get("range_type");
  const relativeText = url.searchParams.get("relative") ?? url.searchParams.get("range") ?? undefined;
  const relative = relativeText ? Number(relativeText) : Number.NaN;
  const sort = url.searchParams.get("sort");
  const from = url.searchParams.get("from");
  const to = url.searchParams.get("to");
  const keyword = url.searchParams.get("keyword");
  const fields = url.searchParams.get("fields");
  const filter = url.searchParams.get("filter");
  if (query) {
    result.query = query;
  }
  if (rangeType === "relative" || rangeType === "absolute" || rangeType === "keyword") {
    result.rangeType = rangeType;
  }
  if (Number.isFinite(relative)) {
    result.relative = relative;
  }
  if (sort) {
    result.sort = sort;
  }
  if (from) {
    result.from = from;
  }
  if (to) {
    result.to = to;
  }
  if (keyword) {
    result.keyword = keyword;
  }
  if (fields) {
    result.fields = fields;
  }
  if (filter) {
    result.filter = filter;
  }
  return result;
}

function appendRange(params: URLSearchParams, input: GraylogSearchInput): void {
  if (input.rangeType === "relative") {
    params.set("range", String(input.relative ?? DEFAULT_GRAYLOG_RELATIVE_SECONDS));
    return;
  }
  if (input.rangeType === "absolute") {
    params.set("from", input.from ?? "");
    params.set("to", input.to ?? "");
    return;
  }
  params.set("keyword", input.keyword ?? "");
}

function aggregateTimerange(input: GraylogAggregateInput): Record<string, string | number> {
  if (input.rangeType === "absolute") {
    return {
      type: "absolute",
      from: input.from ?? "",
      to: input.to ?? "",
    };
  }
  if (input.rangeType === "keyword") {
    return {
      type: "keyword",
      keyword: input.keyword ?? "",
    };
  }
  return {
    type: "relative",
    range: input.relative ?? DEFAULT_GRAYLOG_RELATIVE_SECONDS,
  };
}

function appendOptional(params: URLSearchParams, key: string, value: string | undefined): void {
  if (value !== undefined) {
    params.set(key, value);
  }
}

function appendOptionalNumber(params: URLSearchParams, key: string, value: number | undefined): void {
  if (value !== undefined) {
    params.set(key, String(value));
  }
}

function appendOptionalBool(params: URLSearchParams, key: string, value: boolean | undefined): void {
  if (value !== undefined) {
    params.set(key, String(value));
  }
}

function requireNonEmpty(value: string | undefined, name: string): asserts value is string {
  if (!value || value.length === 0) {
    throw new Error(`graylog ${name} is required`);
  }
}
