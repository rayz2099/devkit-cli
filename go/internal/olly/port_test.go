package olly

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPromConfigDefaults(t *testing.T) {
	app, err := normalize([]byte(`{"prometheus":{"base_url":"http://prometheus.example:9090","default_step":"15s"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if app.Uptrace != nil || app.Prometheus.BaseURL != "http://prometheus.example:9090" || !app.Prometheus.HasStep || app.Prometheus.HasTimeout {
		t.Fatalf("%+v", app.Prometheus)
	}
	bare := normalizeProm(nil)
	if bare.BaseURL != DefaultPrometheusBase || bare.HasStep || bare.HasTimeout {
		t.Fatalf("%+v", bare)
	}
}

func TestPromBuilders(t *testing.T) {
	cfg := PrometheusConfig{BaseURL: "http://127.0.0.1:9090", DefaultStep: "60s", HasStep: true, DefaultTimeout: "30s", HasTimeout: true}
	promql := "100 * (1 - node_memory_MemAvailable_bytes{job=\"node\",instance=\"192.0.2.10\"} /\n    node_memory_MemTotal_bytes{job=\"node\",instance=\"192.0.2.10\"})"
	step := "60s"
	timeout := "45s"
	limit := 100.0
	start := "2026-04-22T10:00:00+08:00"
	end := "2026-04-22T11:00:00+08:00"
	req, err := BuildRangeRequest(cfg, PromRange{Query: promql, Start: &start, End: &end, Step: &step, Timeout: &timeout, Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost || req.URL != "http://127.0.0.1:9090/api/v1/query_range" {
		t.Fatalf("%+v", req)
	}
	body := parseForm(t, req.Body)
	for key, want := range map[string]string{"query": promql, "start": start, "end": end, "step": "60s", "timeout": "45s", "limit": "100"} {
		if one(body, key) != want {
			t.Fatalf("%s=%q", key, one(body, key))
		}
	}
	at := "2026-04-22T10:00:00+08:00"
	instant, err := BuildQueryRequest(cfg, PromQuery{Query: "up", Time: &at})
	if err != nil {
		t.Fatal(err)
	}
	got := parseForm(t, instant.Body)
	if one(got, "timeout") != "30s" || one(got, "query") != "up" {
		t.Fatalf("%+v", got.pairs)
	}
	labels := BuildLabelsRequest(cfg, PromMeta{Start: &start, End: &end, Matches: []string{"up"}})
	if labels.URL != "http://127.0.0.1:9090/api/v1/labels" || !same(parseForm(t, labels.Body).getAll("match[]"), []string{"up"}) {
		t.Fatal(labels)
	}
	values, err := BuildLabelValuesRequest(cfg, "job", PromMeta{Matches: []string{"up"}})
	if err != nil || values.URL != "http://127.0.0.1:9090/api/v1/label/job/values" {
		t.Fatal(err, values)
	}
	series, err := BuildSeriesRequest(cfg, PromMeta{Matches: []string{"up", "process_start_time_seconds{job=\"prometheus\"}"}, Limit: floatPtr(10)})
	if err != nil {
		t.Fatal(err)
	}
	form := parseForm(t, series.Body)
	if series.URL != "http://127.0.0.1:9090/api/v1/series" || one(form, "limit") != "10" || len(form.getAll("match[]")) != 2 {
		t.Fatal(series, form.pairs)
	}
	ready := BuildReadyRequest(cfg)
	if ready.Method != http.MethodGet || ready.URL != "http://127.0.0.1:9090/-/ready" {
		t.Fatal(ready)
	}
	info := BuildBuildInfoRequest(cfg)
	if info.URL != "http://127.0.0.1:9090/api/v1/status/buildinfo" {
		t.Fatal(info)
	}
	if _, err := BuildRangeRequest(PrometheusConfig{BaseURL: cfg.BaseURL}, PromRange{Query: "up", Start: &start, End: &end}); err == nil || !strings.Contains(err.Error(), "--step") {
		t.Fatal(err)
	}
	if _, err := BuildSeriesRequest(cfg, PromMeta{}); err == nil || !strings.Contains(err.Error(), "--match") {
		t.Fatal(err)
	}
}

func TestPromEnvelopeAndFormat(t *testing.T) {
	raw := []byte(`{"status":"error","errorType":"bad_data","error":"invalid parameter query"}`)
	if _, err := ReadPromData(200, raw); err == nil || err.Error() != "bad_data: invalid parameter query" {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/-/ready" {
			w.Write([]byte("Prometheus Server is Ready."))
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("bad request %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		buf, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(buf), "query=up") {
			t.Errorf("body %s", buf)
		}
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()
	cfg := PrometheusConfig{BaseURL: srv.URL}
	spec, err := BuildQueryRequest(cfg, PromQuery{Query: "up"})
	if err != nil {
		t.Fatal(err)
	}
	status, buf, err := PromDo(context.Background(), srv.Client(), spec)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ReadPromData(status, buf)
	if err != nil || !strings.Contains(string(data), `"vector"`) {
		t.Fatal(err, string(data))
	}
	text, err := PromText(context.Background(), srv.Client(), BuildReadyRequest(cfg))
	if err != nil || text != "Prometheus Server is Ready.\n" {
		t.Fatal(err, text)
	}
	matrix := []byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"job":"node","instance":"192.0.2.10"},"values":[[1776823200,"10"],[1776823260,"15"],[1776823320,"20"]]}]}}`)
	out, err := FormatPromResult(matrix, "agent", PromFormatCtx{Command: "query range", Query: "memory", HasQuery: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"series_count": 1`, `"sample_count": 3`, `"min": 10`, `"max": 20`, `"avg": 15`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
	if strings.Contains(out, `"values"`) {
		t.Fatal(out)
	}
}

func TestUptraceBuilders(t *testing.T) {
	cfg := UptraceConfig{BaseURL: "https://uptrace.example.com", WebBaseURL: "https://uptrace.example.com", HasWeb: true, ProjectID: 2, AuthToken: "token", HasAuth: true, DefaultEnv: "loadtest", HasDefaultEnv: true, DefaultTimeDur: floatPtr(10800)}
	rng, err := ParseTimeRange(TimeInput{TimeGte: strPtr("20260429T060000"), TimeDur: floatPtr(10800)}, time.Now())
	if err != nil || rng.Gte != "2026-04-29T06:00:00.000Z" || rng.Lt != "2026-04-29T09:00:00.000Z" {
		t.Fatal(err, rng)
	}
	svc, env := "app-gw", "loadtest"
	req, err := BuildGroupsRequest(cfg, GroupsInput{TimeInput: TimeInput{TimeGte: strPtr("20260429T060000"), TimeDur: floatPtr(10800)}, Service: &svc, Env: &env, Limit: floatPtr(100)})
	if err != nil {
		t.Fatal(err)
	}
	if req.URL != "https://uptrace.example.com/internal/v1/tracing/2/groups" || req.Headers["Authorization"] != "Bearer token" {
		t.Fatal(req.URL, req.Headers)
	}
	if one(req.Query, "time_gte") != "2026-04-29T06:00:00.000Z" || one(req.Query, "system[]") != "all" || one(req.Query, "limit") != "100" || !strings.Contains(one(req.Query, "query"), `where service_name = "app-gw"`) {
		t.Fatal(req.Query.pairs)
	}
	stats, err := BuildGroupStatsRequest(cfg, GroupsInput{TimeInput: TimeInput{TimeGte: strPtr("20260429T060000")}, Service: &svc, Env: &env})
	if err != nil {
		t.Fatal(err)
	}
	if stats.URL != "https://uptrace.example.com/internal/v1/tracing/2/group-stats" || strings.Join(stats.Query.getAll("column[]"), ",") != "per_min(sum(_count)),_error_rate,p50(_duration),p90(_duration),p99(_duration)" {
		t.Fatal(stats.Query.getAll("column[]"))
	}
	jwtCfg := cfg
	jwtCfg.HasAuth = false
	jwtCfg.AuthToken = ""
	jwtCfg.HasJWT = true
	jwtCfg.JWTToken = "jwt-value"
	jwtReq, err := BuildGroupsRequest(jwtCfg, GroupsInput{TimeInput: TimeInput{TimeGte: strPtr("20260429T060000")}, Service: &svc, Env: &env})
	if err != nil || jwtReq.Headers["Cookie"] != "token=jwt-value" || jwtReq.Headers["Authorization"] != "" {
		t.Fatal(err, jwtReq.Headers)
	}
	app, err := normalize([]byte(`{"uptrace":{"base_url":"https://uptrace.example.com","project_id":2,"jwt_token":"jwt-value"}}`))
	if err != nil || app.Uptrace.JWTToken != "jwt-value" || app.Uptrace.HasAuth {
		t.Fatal(err, app.Uptrace)
	}
	start, end := "2026-04-29T06:00:00.000Z", "2026-04-29T09:00:00.000Z"
	trace := "e4f1e0bcd6a1ac296661a3e6ea5507c9"
	spans, err := BuildSpansRequest(cfg, SpansInput{TimeInput: TimeInput{TimeStart: &start, TimeEnd: &end}, TraceID: &trace, DurationGte: floatPtr(10000), Limit: floatPtr(50)})
	if err != nil {
		t.Fatal(err)
	}
	if spans.URL != "https://uptrace.example.com/internal/v1/tracing/2/spans" || one(spans.Query, "trace_id") != trace || one(spans.Query, "duration_gte") != "10000" {
		t.Fatal(spans.Query.pairs)
	}
	detail, err := BuildTraceRequest(cfg, trace)
	if err != nil || detail.URL != "https://uptrace.example.com/internal/v1/tracing/2/traces/"+trace || detail.Query.encode() != "" {
		t.Fatal(err, detail)
	}
	body, err := decodeJSON([]byte(`{"root":{"id":"root","name":"GET /*","duration":100,"children":[{"id":"child","parentId":"root","name":"GET /api","duration":80,"children":[]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	flat := ExtractTraceSpans(body)
	if len(flat) != 2 {
		t.Fatal(len(flat))
	}
	first, _ := flat[0].(map[string]any)
	if _, ok := first["children"]; ok {
		t.Fatal("children kept")
	}
}

func TestTraceAndContext(t *testing.T) {
	id := "e4f1e0bcd6a1ac296661a3e6ea5507c9"
	if got, err := ParseTraceInput(id); err != nil || got.TraceID != id || got.ProjectID != nil {
		t.Fatal(err, got)
	}
	parsed, err := ParseTraceInput("https://uptrace.example.com/traces/2/" + id)
	if err != nil || parsed.TraceID != id || parsed.ProjectID == nil || *parsed.ProjectID != 2 {
		t.Fatal(err, parsed)
	}
	if got, err := ParseTraceInput("trace_id=" + id + " cost=1200ms"); err != nil || got.TraceID != id {
		t.Fatal(err, got)
	}
	if _, err := ParseTraceInput("trace_id=bad"); err == nil || !strings.Contains(err.Error(), "trace id") {
		t.Fatal(err)
	}
	spans := []map[string]any{
		{"id": 1.0, "traceId": id, "parentId": 0.0, "name": "GET /api/order", "kind": "server", "time": "2026-04-29T06:00:00.000Z", "duration": 500000.0, "attrs": map[string]any{"service_name": "app-gw", "http_route": "/api/order"}},
		{"id": 2.0, "parentId": 1.0, "name": "POST /inventory/reserve", "kind": "client", "time": "2026-04-29T06:00:00.100Z", "duration": 350000.0, "attrs": map[string]any{"service_name": "app-gw", "peer_service": "ld-inventory"}},
		{"id": 3.0, "parentId": 2.0, "name": "SELECT inventory", "kind": "db", "time": "2026-04-29T06:00:00.200Z", "duration": 280000.0, "statusCode": "error", "attrs": map[string]any{"service_name": "ld-inventory", "db_statement": "select * from inventory where sku_id = ?"}},
	}
	tree := BuildTraceTree(spans)
	if len(tree.Roots) != 1 || tree.Roots[0].Children[0].Children[0].Span["name"] != "SELECT inventory" || tree.TopSlow[0].Span["id"] != 1.0 {
		t.Fatal("tree")
	}
	if len(tree.Errors) != 1 || tree.Errors[0].Span["id"] != 3.0 {
		t.Fatal(tree.Errors)
	}
	cfg := UptraceConfig{BaseURL: "https://uptrace.example.com", WebBaseURL: "https://uptrace.example.com", HasWeb: true, ProjectID: 2}
	ctx, err := BuildContext(cfg, id, spans, AgentQuery{Service: "app-gw", Env: "loadtest"})
	if err != nil {
		t.Fatal(err)
	}
	text := FormatContextAgent(ctx)
	for _, want := range []string{"trace_id: " + id, "GET /api/order", "SELECT inventory", "source_hints:", "db_statement: select * from inventory where sku_id = ?"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s\n%s", want, text)
		}
	}
}

func TestGraylog(t *testing.T) {
	app, err := normalize([]byte(`{"graylog":{"base_url":"127.0.0.1:9000","username":"admin","password":"admin"}}`))
	if err != nil || app.Graylog.BaseURL != "http://127.0.0.1:9000" {
		t.Fatal(err, app.Graylog)
	}
	if _, err := normalizeGray([]byte(`{"base_url":"127.0.0.1:9000","username":"admin"}`)); err == nil || !strings.Contains(err.Error(), "graylog.password") {
		t.Fatal(err)
	}
	cfg := *app.Graylog
	in, err := ToGrayInput(GraySource{Source: strPtr("app:billing AND level = 3"), Relative: floatPtr(28800), Limit: floatPtr(20)})
	if err != nil {
		t.Fatal(err)
	}
	req := BuildSearchRequest(cfg, in)
	if req.URL != "http://127.0.0.1:9000/api/search/universal/relative" || one(req.Params, "range") != "28800" || one(req.Params, "limit") != "20" {
		t.Fatal(req)
	}
	abs, err := ToGrayInput(GraySource{Source: strPtr("app:billing AND level:3"), From: strPtr("2026-06-01T10:00:00+08:00"), To: strPtr("2026-06-01T11:00:00+08:00"), Fields: strPtr("timestamp,source,logger_name,trace_id,message"), Filter: strPtr("streams:abc"), Offset: floatPtr(10), Decorate: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	absReq := BuildSearchRequest(cfg, abs)
	if absReq.URL != "http://127.0.0.1:9000/api/search/universal/absolute" || one(absReq.Params, "decorate") != "true" || one(absReq.Params, "offset") != "10" || one(absReq.Params, "filter") != "streams:abc" {
		t.Fatal(absReq.Params.pairs)
	}
	link, err := ToGrayInput(GraySource{Source: strPtr("https://log.example.com/search?q=app%3Abilling+AND+level%3A3&rangetype=relative&relative=28800"), Limit: floatPtr(50)})
	if err != nil || link.Query != "app:billing AND level:3" || link.Relative == nil || *link.Relative != 28800 {
		t.Fatal(err, link)
	}
	absLink, err := ToGrayInput(GraySource{Source: strPtr("https://log.example.com/search?q=app%3Abilling&rangetype=absolute&from=2026-06-01T10%3A00%3A00.000Z&to=2026-06-01T11%3A00%3A00.000Z")})
	if err != nil || absLink.RangeType != "absolute" || strOr(absLink.From) != "2026-06-01T10:00:00.000Z" {
		t.Fatal(err, absLink)
	}
	agg := BuildAggRequest(cfg, GrayAgg{Query: "app:billing AND level:3", RangeType: "relative", Relative: floatPtr(28800), Field: "USER_IP", Limit: floatPtr(10)})
	if agg.URL != "http://127.0.0.1:9000/api/views/search/sync" || one(agg.Params, "timeout") != "60000" {
		t.Fatal(agg)
	}
	buf, _ := json.Marshal(agg.Body)
	if !strings.Contains(string(buf), `"query_string":"app:billing AND level:3"`) || !strings.Contains(string(buf), `"field":"USER_IP"`) || !strings.Contains(string(buf), `"rollup":true`) {
		t.Fatal(string(buf))
	}
	sample := graySample()
	text, err := FormatGrayResult(sample, "agent", GrayCtx{Query: "app:billing AND level = 3", RangeType: "relative", Relative: floatPtr(28800)})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"query": "app:billing AND level = 3"`, `"total_results": 2`, `"messages"`, `"logger_summary"`, `"trace_ids"`, `"abc123"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s\n%s", want, text)
		}
	}
	grouped, err := FormatGrayResult(sample, "agent", GrayCtx{Query: "q", RangeType: "relative", Relative: floatPtr(1), GroupBy: strPtr("logger_name")})
	if err != nil || !strings.Contains(grouped, `"group_by": "logger_name"`) || !strings.Contains(grouped, `"count": 2`) {
		t.Fatal(err, grouped)
	}
	big := cloneMap(sample)
	big["total_results"] = 102638.0
	human, err := FormatGrayResult(big, "human", GrayCtx{Query: "q", RangeType: "relative", GroupBy: strPtr("USER_IP")})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Graylog groups(sampled)", "returned=2", "not full-result aggregation", "missing=2"} {
		if !strings.Contains(human, want) {
			t.Fatal(human)
		}
	}
	logs, err := FormatGrayResult(sample, "human", GrayCtx{Query: "q", RangeType: "relative"})
	if err != nil || !strings.Contains(logs, "Graylog logs: total=2") || !strings.Contains(logs, "2026-06-01T10:00:00.000Z node-1 [3] com.example.OrderService trace=abc123") || !strings.Contains(logs, "RPC timeout") {
		t.Fatal(err, logs)
	}
	fields, err := FormatGrayFields(sample, "human", GrayCtx{Query: "q", RangeType: "relative"})
	if err != nil || !strings.Contains(fields, "Graylog fields: count=8") || !strings.Contains(fields, "- logger_name") {
		t.Fatal(err, fields)
	}
	agentFields, err := FormatGrayFields(sample, "agent", GrayCtx{Query: "q", RangeType: "relative"})
	if err != nil || !strings.Contains(agentFields, `"suggested_group_by"`) {
		t.Fatal(err, agentFields)
	}
	aggBody, err := decodeJSON([]byte(`{"results":{"query-1":{"search_types":{"pivot-1":{"total":102634,"rows":[{"key":["220.181.51.116"],"values":[{"key":["count()"],"value":535}],"source":"leaf"},{"key":["220.181.51.113"],"values":[{"key":["count()"],"value":498}],"source":"leaf"},{"key":[],"values":[{"key":["count()"],"value":102634}],"source":"non-leaf"}]}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	aggRows := ReadAgg(aggBody, GrayAgg{Query: "q", Field: "USER_IP"})
	if aggRows.Total != 102634 || len(aggRows.Rows) != 2 || aggRows.Rows[0].Key != "220.181.51.116" || aggRows.Rows[0].Count != 535 {
		t.Fatalf("%+v", aggRows)
	}
	aggText, err := FormatGrayAgg(GrayAggResult{Query: "q", Field: "USER_IP", Total: 102634, Rows: []GrayAggRow{{Key: "220.181.51.116", Count: 535}, {Key: "220.181.51.113", Count: 498}}}, "human")
	if err != nil || !strings.Contains(aggText, "Graylog aggregate: field=USER_IP total=102634 rows=2") || !strings.Contains(aggText, "220.181.51.116\t535") {
		t.Fatal(err, aggText)
	}
}

func TestGrafana(t *testing.T) {
	app, err := normalize([]byte(`{"grafana":{"base_url":"10.0.48.10:3000","username":"admin","password":"secret"}}`))
	if err != nil || app.Grafana.BaseURL != "http://10.0.48.10:3000" || app.Grafana.HasWeb {
		t.Fatal(err, app.Grafana)
	}
	if _, err := normalizeGraf([]byte(`{"base_url":"http://10.0.48.10:3000","username":"admin"}`)); err == nil || !strings.Contains(err.Error(), "grafana.password") {
		t.Fatal(err)
	}
	ref, err := ParseDashRef("https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/5Lia5Yqh5o6l5Y-j?orgId=1&from=now-1h&to=now")
	if err != nil || ref.UID != "c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a" || ref.OrgID == nil || *ref.OrgID != 1 || strOr(ref.From) != "now-1h" {
		t.Fatal(err, ref)
	}
	bare, err := ParseDashRef("c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a")
	if err != nil || bare.UID != "c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a" || bare.OrgID != nil {
		t.Fatal(err, bare)
	}
	if _, err := ParseDashRef("https://grafana.dtactivity.cn/dashboards"); err == nil || !strings.Contains(err.Error(), "missing dashboard uid") {
		t.Fatal(err)
	}
	link, err := BuildDashURL("https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz?orgId=1", "http://10.0.48.10:3000", "/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz", "c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a", intPtr(1), "now-6h", "now")
	if err != nil || link != "https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz?orgId=1&from=now-6h&to=now" {
		t.Fatal(err, link)
	}
	panels := CollectPanels([]Panel{{Type: "row", Title: "row", Panels: []Panel{{ID: intPtr(2), Type: "timeseries", Title: "nested"}}}, {ID: intPtr(1), Type: "timeseries", Title: "top"}})
	if panels[0].Title != "nested" || panels[1].Title != "top" {
		t.Fatal(panels)
	}
	body, err := BuildQueryBody(samplePanel(), "now-6h", "now")
	if err != nil || len(body.Queries) != 1 || body.Queries[0].RefID != "A" || !body.Queries[0].Range || body.Queries[0].Instant || body.Queries[0].Expr == "" || body.Queries[0].IntervalMs != 15000 {
		t.Fatal(err, body)
	}
	bad := Panel{ID: intPtr(9), Type: "timeseries", Title: "mysql", Datasource: &DsRef{Type: "mysql", UID: "mysql"}, Targets: []Target{{RefID: strPtr("A"), Expr: strPtr("select 1"), HasRange: true, Range: true}}}
	if _, err := BuildQueryBody(bad, "now-6h", "now"); err == nil || !strings.Contains(err.Error(), "datasource is not prometheus") {
		t.Fatal(err)
	}
	series := summarizeFrames([]any{map[string]any{
		"schema": map[string]any{"name": "/napi/blog/detail/-DEFAULT", "fields": []any{
			map[string]any{"name": "Time", "type": "time"},
			map[string]any{"name": "Value", "type": "number", "labels": map[string]any{"path": "/napi/blog/detail/", "biz": "DEFAULT"}, "config": map[string]any{"displayNameFromDS": "/napi/blog/detail/-DEFAULT"}},
		}},
		"data": map[string]any{"values": []any{[]any{1.0, 2.0, 3.0}, []any{10.0, nil, 25.0}}},
	}})
	if len(series) != 1 || series[0].Name != "/napi/blog/detail/-DEFAULT" || series[0].SampleCount != 2 || series[0].Last == nil || *series[0].Last != 25 || series[0].LastTime == nil || *series[0].LastTime != 3 || series[0].Max == nil || *series[0].Max != 25 {
		t.Fatalf("%+v", series)
	}
	if _, err := ReadTargetSums(QueryBody{Queries: []QueryItem{{RefID: "A", Expr: "up"}}}, map[string]any{"results": map[string]any{"A": map[string]any{"error": "timeout", "status": 500.0}}}); err == nil || err.Error() != "grafana query A: timeout" {
		t.Fatal(err)
	}
	human, err := FormatGrafana(sampleGrafResult(), "human")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Grafana: 业务接口", "URL: https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz?orgId=1&from=now-6h&to=now", "Time: now-6h → now", "#1 业务接口看板", "last=4 max=8 samples=3"} {
		if !strings.Contains(human, want) {
			t.Fatalf("missing %s\n%s", want, human)
		}
	}
	agent, err := FormatGrafana(sampleGrafResult(), "agent")
	if err != nil || !strings.Contains(agent, `"title": "业务接口"`) || !strings.Contains(agent, `"last": 4`) || strings.Contains(agent, "values") {
		t.Fatal(err, agent)
	}
	wide := sampleGrafResult()
	wide.Panels[0].Targets[0].SeriesCount = 12
	wide.Panels[0].Targets[0].Series = nil
	for i := 0; i < 12; i++ {
		n := float64(i)
		wide.Panels[0].Targets[0].Series = append(wide.Panels[0].Targets[0].Series, SeriesSum{Name: "s" + itoa(i), SampleCount: 1, Last: &n, Max: &n, Labels: map[string]string{}})
	}
	top, err := FormatGrafana(wide, "human")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(top, "top 10 by max:") || !strings.Contains(top, "s11  last=11 max=11 samples=1") || strings.Contains(top, "s0  last=0") {
		t.Fatal(top)
	}
}

func TestDoctor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	var buf strings.Builder
	err := Doctor("", &buf)
	if err == nil || !strings.Contains(buf.String(), "fail config:") || !strings.Contains(buf.String(), "pass prometheus\n") {
		t.Fatal(err, buf.String())
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(statErr) {
		t.Fatal("config dir created", statErr)
	}
	path := filepath.Join(home, "olly.json")
	os.WriteFile(path, []byte(`{"graylog":{"base_url":"127.0.0.1:9000","username":"admin"}}`), 0o644)
	buf.Reset()
	err = Doctor(path, &buf)
	if err == nil || !strings.Contains(buf.String(), "pass config\n") || !strings.Contains(buf.String(), "fail graylog:") || !strings.Contains(buf.String(), "pass prometheus\n") {
		t.Fatal(err, buf.String())
	}
	os.WriteFile(path, []byte(`{"uptrace":{"base_url":"https://uptrace.example.com","project_id":2,"jwt_token":"jwt"}}`), 0o644)
	buf.Reset()
	if err := Doctor(path, &buf); err != nil || !strings.Contains(buf.String(), "pass uptrace\n") || !strings.Contains(buf.String(), "pass prometheus\n") {
		t.Fatal(err, buf.String())
	}
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".config", "cache"))
	if err := Guard(); err == nil {
		t.Fatal("cache inside config accepted")
	}
}

func one(f form, key string) string {
	val, _ := f.get(key)
	return val
}

func parseForm(t *testing.T, body string) form {
	t.Helper()
	var out form
	if body == "" {
		return out
	}
	for _, part := range strings.Split(body, "&") {
		key, val, _ := strings.Cut(part, "=")
		uk, _ := url.QueryUnescape(key)
		uv, _ := url.QueryUnescape(val)
		out.add(uk, uv)
	}
	return out
}

func floatPtr(v float64) *float64 { return &v }
func strPtr(v string) *string     { return &v }
func boolPtr(v bool) *bool        { return &v }
func intPtr(v int) *int           { return &v }

func same(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func graySample() map[string]any {
	return map[string]any{
		"total_results": 2.0,
		"fields":        []any{"timestamp", "source", "message", "level", "logger_name", "trace_id", "app", "facility"},
		"messages": []any{
			map[string]any{"message": map[string]any{"timestamp": "2026-06-01T10:00:00.000Z", "message": "RPC timeout", "source": "node-1", "logger_name": "com.example.OrderService", "trace_id": "abc123", "level": 3.0}},
			map[string]any{"message": map[string]any{"timestamp": "2026-06-01T10:00:01.000Z", "message": "RPC timeout again", "source": "node-2", "logger_name": "com.example.OrderService", "trace_id": "def456", "level": 3.0}},
		},
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func samplePanel() Panel {
	ds := &DsRef{Type: "prometheus", UID: "prom-uid"}
	return Panel{
		ID: intPtr(1), Type: "timeseries", Title: "业务接口看板", Datasource: ds,
		Targets: []Target{
			{RefID: strPtr("A"), Expr: strPtr("sum(irate(http_biz_count_total[5m])) by (path)"), HasRange: true, Range: true, LegendFormat: strPtr("{{path}}"), Datasource: ds},
			{RefID: strPtr("B"), Expr: strPtr("hidden"), HasRange: true, Range: true, Hide: true, Datasource: ds},
		},
	}
}

func sampleGrafResult() AnalyzeResult {
	last, max := 4.0, 8.0
	lastTime := 3.0
	return AnalyzeResult{
		Title: "业务接口",
		UID:   "c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a",
		URL:   "https://grafana.dtactivity.cn/d/c529cbb2-fc4d-45fc-b1fb-c68bc9aafb2a/biz?orgId=1&from=now-6h&to=now",
		OrgID: intPtr(1),
		Time:  dashTime{From: "now-6h", To: "now"},
		Panels: []PanelSum{{
			ID: intPtr(1), Title: "业务接口看板", Type: "timeseries",
			Targets: []TargetSum{{
				RefID: "A", Expr: "up", SeriesCount: 1,
				Series: []SeriesSum{{Name: "/x", Labels: map[string]string{"path": "/x"}, SampleCount: 3, Last: &last, LastTime: &lastTime, Max: &max}},
			}},
		}},
	}
}
