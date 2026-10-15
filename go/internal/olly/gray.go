package olly

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultGrayRelative = 28800.0
	defaultGrayLimit    = 50.0
	defaultGraySort     = "timestamp:desc"
)

// GraySource 是命令行和粘贴 URL 混在一起的原始输入。nil 表示用户没传。
type GraySource struct {
	Source   *string
	Relative *float64
	From     *string
	To       *string
	Keyword  *string
	Limit    *float64
	Sort     *string
	Fields   *string
	Filter   *string
	Offset   *float64
	Decorate *bool
}

// GrayInput 是已经判定好时间类型的搜索条件。
type GrayInput struct {
	Query     string
	RangeType string
	Relative  *float64
	From      *string
	To        *string
	Keyword   *string
	Limit     *float64
	Sort      *string
	Fields    *string
	Filter    *string
	Offset    *float64
	Decorate  *bool
}

// GrayAgg 是服务端 pivot 聚合。limit 作用在分组上，不是日志条数。
type GrayAgg struct {
	Query     string
	RangeType string
	Field     string
	Relative  *float64
	From      *string
	To        *string
	Keyword   *string
	Limit     *float64
}

// GraySpec 是 Graylog GET。
type GraySpec struct {
	Method string
	URL    string
	Params form
}

// GrayPost 是聚合用的 POST。
type GrayPost struct {
	URL    string
	Params form
	Body   any
}

// ToGrayInput 把查询文本或浏览器 URL 收成 API 参数。时间类型由显式 flag 和 URL 的 rangetype 一起决定。
func ToGrayInput(src GraySource) (GrayInput, error) {
	parsed := parseGrayURL(strOr(src.Source))
	query := strOr(src.Source)
	if parsed.Query != "" {
		query = parsed.Query
	}
	if query == "" {
		return GrayInput{}, errors.New("graylog logs requires query text or Graylog search URL")
	}
	from := src.From
	if from == nil {
		from = parsed.From
	}
	to := src.To
	if to == nil {
		to = parsed.To
	}
	keyword := src.Keyword
	if keyword == nil {
		keyword = parsed.Keyword
	}
	relative := src.Relative
	if relative == nil {
		relative = parsed.Relative
	}
	rangeType := "relative"
	if (src.From != nil && *src.From != "") || (src.To != nil && *src.To != "") || parsed.RangeType == "absolute" {
		rangeType = "absolute"
	} else if (src.Keyword != nil && *src.Keyword != "") || parsed.RangeType == "keyword" {
		rangeType = "keyword"
	}
	if rangeType == "absolute" {
		if from == nil || *from == "" {
			return GrayInput{}, errors.New("graylog --from is required")
		}
		if to == nil || *to == "" {
			return GrayInput{}, errors.New("graylog --to is required")
		}
	}
	if rangeType == "keyword" {
		if keyword == nil || *keyword == "" {
			return GrayInput{}, errors.New("graylog --keyword is required")
		}
	}
	in := GrayInput{
		Query:     query,
		RangeType: rangeType,
		From:      from,
		To:        to,
		Keyword:   keyword,
		Limit:     src.Limit,
		Fields:    src.Fields,
		Filter:    src.Filter,
		Offset:    src.Offset,
		Decorate:  src.Decorate,
	}
	if src.Sort != nil {
		in.Sort = src.Sort
	} else {
		in.Sort = parsed.Sort
	}
	if src.Fields == nil {
		in.Fields = parsed.Fields
	}
	if src.Filter == nil {
		in.Filter = parsed.Filter
	}
	if rangeType == "relative" {
		rel := defaultGrayRelative
		if relative != nil {
			rel = *relative
		}
		in.Relative = &rel
	}
	return in, nil
}

type parsedGray struct {
	IsURL     bool
	Query     string
	RangeType string
	Relative  *float64
	From      *string
	To        *string
	Keyword   *string
	Sort      *string
	Fields    *string
	Filter    *string
}

func parseGrayURL(source string) parsedGray {
	if source == "" {
		return parsedGray{}
	}
	u, err := url.Parse(source)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return parsedGray{}
	}
	q := u.Query()
	out := parsedGray{IsURL: true}
	if text := firstQuery(q, "q", "query"); text != "" {
		out.Query = text
	}
	if text := firstQuery(q, "rangetype", "range_type"); text == "relative" || text == "absolute" || text == "keyword" {
		out.RangeType = text
	}
	if text := firstQuery(q, "relative", "range"); text != "" {
		if num, err := parseNum(text); err == nil {
			out.Relative = &num
		}
	}
	if text := q.Get("sort"); text != "" {
		out.Sort = &text
	}
	if text := q.Get("from"); text != "" {
		out.From = &text
	}
	if text := q.Get("to"); text != "" {
		out.To = &text
	}
	if text := q.Get("keyword"); text != "" {
		out.Keyword = &text
	}
	if text := q.Get("fields"); text != "" {
		out.Fields = &text
	}
	if text := q.Get("filter"); text != "" {
		out.Filter = &text
	}
	return out
}

func firstQuery(q url.Values, keys ...string) string {
	for _, key := range keys {
		if text := q.Get(key); text != "" {
			return text
		}
	}
	return ""
}

// BuildSearchRequest 打 /api/search/universal/{relative|absolute|keyword}。
func BuildSearchRequest(cfg GraylogConfig, in GrayInput) GraySpec {
	params := form{}
	params.set("query", in.Query)
	limit := defaultGrayLimit
	if in.Limit != nil {
		limit = *in.Limit
	}
	params.set("limit", jsNum(limit))
	sortKey := defaultGraySort
	if in.Sort != nil {
		sortKey = *in.Sort
	}
	params.set("sort", sortKey)
	addStr(&params, "fields", in.Fields)
	addStr(&params, "filter", in.Filter)
	addNum(&params, "offset", in.Offset)
	if in.Decorate != nil {
		params.set("decorate", strconvBool(*in.Decorate))
	}
	appendGrayRange(&params, in)
	return GraySpec{
		Method: http.MethodGet,
		URL:    cfg.BaseURL + "/api/search/universal/" + in.RangeType,
		Params: params,
	}
}

func appendGrayRange(params *form, in GrayInput) {
	switch in.RangeType {
	case "absolute":
		from, to := "", ""
		if in.From != nil {
			from = *in.From
		}
		if in.To != nil {
			to = *in.To
		}
		params.set("from", from)
		params.set("to", to)
	case "keyword":
		keyword := ""
		if in.Keyword != nil {
			keyword = *in.Keyword
		}
		params.set("keyword", keyword)
	default:
		rel := defaultGrayRelative
		if in.Relative != nil {
			rel = *in.Relative
		}
		params.set("range", jsNum(rel))
	}
}

// BuildAggRequest 对齐 Graylog 页面的 count() by field，走 views search sync。
func BuildAggRequest(cfg GraylogConfig, in GrayAgg) GrayPost {
	params := form{}
	params.set("timeout", "60000")
	limit := 10.0
	if in.Limit != nil {
		limit = *in.Limit
	}
	return GrayPost{
		URL:    cfg.BaseURL + "/api/views/search/sync",
		Params: params,
		Body: map[string]any{
			"queries": []any{
				map[string]any{
					"id": "query-1",
					"query": map[string]any{
						"type":         "elasticsearch",
						"query_string": in.Query,
					},
					"timerange": aggRange(in),
					"filter": map[string]any{
						"type":    "or",
						"filters": []any{},
					},
					"search_types": []any{
						map[string]any{
							"id":   "pivot-1",
							"type": "pivot",
							"row_groups": []any{
								map[string]any{
									"type":  "values",
									"field": in.Field,
									"limit": limit,
								},
							},
							"series": []any{
								map[string]any{"type": "count", "id": "count()"},
							},
							"column_groups": []any{},
							"rollup":        true,
						},
					},
				},
			},
		},
	}
}

func aggRange(in GrayAgg) map[string]any {
	switch in.RangeType {
	case "absolute":
		from, to := "", ""
		if in.From != nil {
			from = *in.From
		}
		if in.To != nil {
			to = *in.To
		}
		return map[string]any{"type": "absolute", "from": from, "to": to}
	case "keyword":
		keyword := ""
		if in.Keyword != nil {
			keyword = *in.Keyword
		}
		return map[string]any{"type": "keyword", "keyword": keyword}
	default:
		rel := defaultGrayRelative
		if in.Relative != nil {
			rel = *in.Relative
		}
		return map[string]any{"type": "relative", "range": rel}
	}
}

// GrayGet 发搜索请求。鉴权是 basic，并且必须带 X-Requested-By，否则 Graylog 会拒。
func GrayGet(ctx context.Context, client *http.Client, cfg GraylogConfig, spec GraySpec) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, spec.Method, spec.URL+"?"+spec.Params.encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", BasicAuth(cfg.Username, cfg.Password))
	req.Header.Set("X-Requested-By", "olly-cli")
	return grayJSON(ctx, client, req)
}

// GrayPostJSON 发聚合请求。
func GrayPostJSON(ctx context.Context, client *http.Client, cfg GraylogConfig, spec GrayPost) (any, error) {
	buf, err := marshalCompact(spec.Body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, spec.URL+"?"+spec.Params.encode(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", BasicAuth(cfg.Username, cfg.Password))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-By", "olly-cli")
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("graylog http %d: %s", resp.StatusCode, string(body))
	}
	val, err := decodeJSON(body)
	if err != nil {
		return nil, fmt.Errorf("graylog response is not json: %s", string(body))
	}
	return val, nil
}

func grayJSON(ctx context.Context, client *http.Client, req *http.Request) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("graylog http %d: %s", resp.StatusCode, string(body))
	}
	val, err := decodeJSON(body)
	if err != nil {
		return nil, fmt.Errorf("graylog response is not json: %s", string(body))
	}
	rec, ok := isRecord(val)
	if !ok {
		return map[string]any{}, nil
	}
	return rec, nil
}

func marshalCompact(val any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(val); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// GrayAggResult 是 pivot 叶子行。non-leaf 合计行不能混进结果。
type GrayAggResult struct {
	Query     string       `json:"query"`
	Field     string       `json:"field"`
	Total     float64      `json:"total"`
	Rows      []GrayAggRow `json:"rows"`
	Effective any          `json:"effective_timerange,omitempty"`
}

// GrayAggRow 是一个分组键和它的 count。
type GrayAggRow struct {
	Key   string  `json:"key"`
	Count float64 `json:"count"`
}

// ReadAgg 从 views 响应里取出 pivot-1 的叶子行。
func ReadAgg(body any, in GrayAgg) GrayAggResult {
	out := GrayAggResult{Query: in.Query, Field: in.Field, Rows: []GrayAggRow{}}
	rec, _ := isRecord(body)
	results, _ := isRecord(rec["results"])
	query, _ := isRecord(results["query-1"])
	types, _ := isRecord(query["search_types"])
	pivot, _ := isRecord(types["pivot-1"])
	if total, ok := asFloat(pivot["total"]); ok {
		out.Total = total
	}
	if eff, ok := pivot["effective_timerange"]; ok {
		out.Effective = eff
	}
	rows, _ := pivot["rows"].([]any)
	for _, row := range rows {
		item, ok := isRecord(row)
		if !ok || item["source"] != "leaf" {
			continue
		}
		keys, _ := item["key"].([]any)
		if len(keys) == 0 {
			continue
		}
		key, ok := keys[0].(string)
		if !ok {
			continue
		}
		vals, _ := item["values"].([]any)
		if len(vals) == 0 {
			continue
		}
		first, _ := isRecord(vals[0])
		count, ok := asFloat(first["value"])
		if !ok {
			continue
		}
		out.Rows = append(out.Rows, GrayAggRow{Key: key, Count: count})
	}
	return out
}

// GrayCtx 是格式化时要带回的查询窗口，不是 Graylog 响应的一部分。
type GrayCtx struct {
	Query     string
	RangeType string
	Relative  *float64
	From      *string
	To        *string
	Keyword   *string
	GroupBy   *string
}

// FormatGrayResult 默认打日志列表。--group-by 才按字段分组，并且标明这只是返回集上的抽样。
func FormatGrayResult(body map[string]any, mode string, ctx GrayCtx) (string, error) {
	if mode == "plain" {
		return pretty(body)
	}
	messages := toSamples(body)
	loggers := summarizeField(body, "logger_name")
	var groups []loggerSum
	grouped := ctx.GroupBy != nil && *ctx.GroupBy != ""
	if grouped {
		groups = summarizeField(body, *ctx.GroupBy)
	}
	if mode == "agent" {
		return formatGrayAgent(body, ctx, messages, loggers, groups, grouped)
	}
	if grouped {
		return formatGroupedHuman(body, *ctx.GroupBy, groups, len(messages)), nil
	}
	return formatLogsHuman(body, messages), nil
}

func formatGrayAgent(body map[string]any, ctx GrayCtx, messages []msgSample, loggerSum, groups []loggerSum, grouped bool) (string, error) {
	total := float64(len(messages))
	if num, ok := asFloat(body["total_results"]); ok {
		total = num
	}
	var ids []string
	seen := map[string]struct{}{}
	for _, msg := range messages {
		if msg.TraceID == "" {
			continue
		}
		if _, ok := seen[msg.TraceID]; ok {
			continue
		}
		seen[msg.TraceID] = struct{}{}
		ids = append(ids, msg.TraceID)
	}
	if ids == nil {
		ids = []string{}
	}
	payload := map[string]any{
		"query":             ctx.Query,
		"range":             formatRange(ctx),
		"total_results":     total,
		"returned_messages": len(messages),
		"trace_ids":         ids,
		"logger_summary":    namedGroups("logger_name", loggerSum),
		"messages":          samplesAny(messages),
	}
	if ctx.GroupBy != nil {
		payload["group_by"] = *ctx.GroupBy
	}
	if grouped {
		payload["groups"] = namedGroups(*ctx.GroupBy, groups)
	}
	return pretty(payload)
}

func formatRange(ctx GrayCtx) map[string]any {
	out := map[string]any{"type": ctx.RangeType}
	if ctx.Relative != nil {
		out["relative"] = *ctx.Relative
	}
	if ctx.From != nil {
		out["from"] = *ctx.From
	}
	if ctx.To != nil {
		out["to"] = *ctx.To
	}
	if ctx.Keyword != nil {
		out["keyword"] = *ctx.Keyword
	}
	return out
}

// FormatGrayFields 列出这次返回里能用来 group-by 的字段。
func FormatGrayFields(body map[string]any, mode string, ctx GrayCtx) (string, error) {
	fields := fieldNames(body)
	if mode == "plain" {
		return pretty(map[string]any{"fields": fields})
	}
	if mode == "agent" {
		return pretty(map[string]any{
			"query":              ctx.Query,
			"range":              formatRange(ctx),
			"field_count":        len(fields),
			"fields":             fields,
			"suggested_group_by": filterSuggested(fields),
		})
	}
	lines := make([]string, len(fields))
	for i, field := range fields {
		lines[i] = "- " + field
	}
	return fmt.Sprintf("Graylog fields: count=%d\n%s\n", len(fields), strings.Join(lines, "\n")), nil
}

// FormatGrayAgg 把聚合打成 key 和 count。人和 agent 都能直接接着反查。
func FormatGrayAgg(result GrayAggResult, mode string) (string, error) {
	if mode == "plain" || mode == "agent" {
		return pretty(result)
	}
	lines := []string{fmt.Sprintf("Graylog aggregate: field=%s total=%s rows=%d", result.Field, jsNum(result.Total), len(result.Rows))}
	for _, row := range result.Rows {
		lines = append(lines, row.Key+"\t"+jsNum(row.Count))
	}
	return strings.Join(lines, "\n") + "\n", nil
}

type msgSample struct {
	Timestamp string
	Source    string
	Level     any
	HasLevel  bool
	Message   string
	HasMsg    bool
	TraceID   string
	HasTrace  bool
	Logger    string
}

func formatLogsHuman(body map[string]any, messages []msgSample) string {
	if len(messages) == 0 {
		return "No logs.\n"
	}
	total := "-"
	if _, ok := body["total_results"]; ok {
		if num, isNum := asFloat(body["total_results"]); isNum {
			total = jsNum(num)
		}
	}
	lines := []string{fmt.Sprintf("Graylog logs: total=%s returned=%d", total, len(messages))}
	for _, sample := range messages {
		trace := ""
		if sample.TraceID != "" {
			trace = " trace=" + sample.TraceID
		}
		level := "-"
		if sample.HasLevel {
			level = jsFmt(sample.Level)
		}
		lines = append(lines, fmt.Sprintf("%s %s [%s] %s%s", dash(sample.Timestamp), dash(sample.Source), level, sample.Logger, trace))
		msg := ""
		if sample.HasMsg {
			msg = sample.Message
		}
		lines = append(lines, "  "+msg)
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatGroupedHuman(body map[string]any, field string, sums []loggerSum, returned int) string {
	if len(sums) == 0 {
		return "No logs.\n"
	}
	totalText := "-"
	sampled := false
	if num, ok := asFloat(body["total_results"]); ok {
		totalText = jsNum(num)
		if num > float64(returned) {
			sampled = true
		}
	}
	missing := 0
	missKey := "<missing_" + field + ">"
	for _, sum := range sums {
		if sum.Value == missKey {
			missing += sum.Count
		}
	}
	label := "Graylog groups"
	if sampled {
		label = "Graylog groups(sampled)"
	}
	lines := []string{fmt.Sprintf("%s: total=%s returned=%d group_by=%s groups=%d", label, totalText, returned, field, len(sums))}
	if sampled {
		lines = append(lines, "warning: grouped counts are computed from returned messages only, not full-result aggregation.")
	}
	if missing > 0 {
		lines = append(lines, fmt.Sprintf("warning: group field missing in returned messages: field=%s missing=%d.", field, missing))
	}
	for _, sum := range sums {
		ids := "-"
		if len(sum.TraceIDs) > 0 {
			n := len(sum.TraceIDs)
			if n > 5 {
				n = 5
			}
			ids = strings.Join(sum.TraceIDs[:n], ",")
		}
		lines = append(lines, fmt.Sprintf("- %s=%s count=%d trace_ids=%s", field, sum.Value, sum.Count, ids))
		n := len(sum.Samples)
		if n > 2 {
			n = 2
		}
		for _, sample := range sum.Samples[:n] {
			msg := ""
			if sample.HasMsg {
				msg = sample.Message
			}
			lines = append(lines, fmt.Sprintf("  %s %s %s", dash(sample.Timestamp), dash(sample.Source), msg))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func dash(val string) string {
	if val == "" {
		return "-"
	}
	return val
}

type loggerSum struct {
	Value    string
	Count    int
	TraceIDs []string
	Samples  []msgSample
}

func summarizeField(body map[string]any, field string) []loggerSum {
	order := []string{}
	index := map[string]*loggerSum{}
	for _, item := range messageMaps(body) {
		msg := map[string]any{}
		if rec, ok := isRecord(item["message"]); ok {
			msg = rec
		}
		value := ""
		if text, ok := stringish(msg[field]); ok {
			value = text
		} else if field == "logger_name" {
			value = loggerName(msg)
		} else {
			value = "<missing_" + field + ">"
		}
		sum := index[value]
		if sum == nil {
			sum = &loggerSum{Value: value}
			index[value] = sum
			order = append(order, value)
		}
		sum.Count++
		if trace := traceOf(msg); trace != "" && !containsStr(sum.TraceIDs, trace) {
			sum.TraceIDs = append(sum.TraceIDs, trace)
		}
		if len(sum.Samples) < 5 {
			sum.Samples = append(sum.Samples, toSample(msg, traceOf(msg)))
		}
	}
	out := make([]loggerSum, 0, len(order))
	for _, key := range order {
		out = append(out, *index[key])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

func namedGroups(field string, sums []loggerSum) []any {
	if sums == nil {
		sums = []loggerSum{}
	}
	out := make([]any, 0, len(sums))
	for _, sum := range sums {
		out = append(out, map[string]any{
			field:       sum.Value,
			"count":     sum.Count,
			"trace_ids": nonNilStr(sum.TraceIDs),
			"samples":   samplesAny(sum.Samples),
		})
	}
	return out
}

func samplesAny(list []msgSample) []any {
	if list == nil {
		return []any{}
	}
	out := make([]any, 0, len(list))
	for _, sample := range list {
		item := map[string]any{}
		if sample.Timestamp != "" {
			item["timestamp"] = sample.Timestamp
		}
		if sample.Source != "" {
			item["source"] = sample.Source
		}
		if sample.HasLevel {
			item["level"] = sample.Level
		}
		if sample.HasMsg {
			item["message"] = sample.Message
		}
		if sample.HasTrace {
			item["trace_id"] = sample.TraceID
		}
		item["logger_name"] = sample.Logger
		out = append(out, item)
	}
	return out
}

func toSamples(body map[string]any) []msgSample {
	items := messageMaps(body)
	out := make([]msgSample, 0, len(items))
	for _, item := range items {
		msg := map[string]any{}
		if rec, ok := isRecord(item["message"]); ok {
			msg = rec
		}
		out = append(out, toSample(msg, traceOf(msg)))
	}
	return out
}

func toSample(msg map[string]any, trace string) msgSample {
	sample := msgSample{Logger: loggerName(msg)}
	if text, ok := msg["timestamp"].(string); ok {
		sample.Timestamp = text
	}
	if text, ok := msg["source"].(string); ok {
		sample.Source = text
	}
	if _, ok := msg["level"]; ok && msg["level"] != nil {
		sample.Level = numOrRaw(msg["level"])
		sample.HasLevel = true
	}
	if text, ok := msg["message"].(string); ok {
		sample.Message = text
		sample.HasMsg = true
	}
	if trace != "" {
		sample.TraceID = trace
		sample.HasTrace = true
	} else if _, ok := msg["trace_id"]; ok || msg["traceId"] != nil {
		sample.HasTrace = msg["trace_id"] != nil || msg["traceId"] != nil
		sample.TraceID = trace
	}
	return sample
}

func numOrRaw(val any) any {
	if num, ok := asFloat(val); ok {
		return num
	}
	return val
}

func loggerName(msg map[string]any) string {
	if text, ok := msg["logger_name"].(string); ok {
		return text
	}
	if text, ok := msg["loggerName"].(string); ok {
		return text
	}
	return "<missing_logger_name>"
}

func traceOf(msg map[string]any) string {
	if text, ok := msg["trace_id"].(string); ok {
		return text
	}
	if text, ok := msg["traceId"].(string); ok {
		return text
	}
	return ""
}

func messageMaps(body map[string]any) []map[string]any {
	list, _ := body["messages"].([]any)
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if rec, ok := isRecord(item); ok {
			out = append(out, rec)
		}
	}
	return out
}

func fieldNames(body map[string]any) []string {
	set := map[string]struct{}{}
	if list, ok := body["fields"].([]any); ok {
		for _, item := range list {
			if text, ok := item.(string); ok {
				set[text] = struct{}{}
			}
		}
	}
	for _, item := range messageMaps(body) {
		msg, _ := isRecord(item["message"])
		for key := range msg {
			set[key] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func filterSuggested(fields []string) []string {
	want := map[string]struct{}{
		"app": {}, "facility": {}, "level": {}, "logger_name": {},
		"source": {}, "thread_name": {}, "trace_id": {},
	}
	var out []string
	for _, field := range fields {
		if _, ok := want[field]; ok {
			out = append(out, field)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func stringish(val any) (string, bool) {
	switch typed := val.(type) {
	case string:
		return typed, true
	case float64:
		return jsNum(typed), true
	case json.Number:
		return typed.String(), true
	case bool:
		return strconvBool(typed), true
	default:
		return "", false
	}
}

func containsStr(list []string, val string) bool {
	for _, item := range list {
		if item == val {
			return true
		}
	}
	return false
}

func nonNilStr(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func parseNum(text string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(text), 64)
}
