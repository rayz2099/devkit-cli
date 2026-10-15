package olly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultTimeDur = 10800.0

var defaultStatCols = []string{
	"per_min(sum(_count))",
	"_error_rate",
	"p50(_duration)",
	"p90(_duration)",
	"p99(_duration)",
}

// TimeInput 是页面上的 time_gte/time_dur，要换成 API 的 time_gte/time_lt。
type TimeInput struct {
	TimeStart *string
	TimeEnd   *string
	TimeGte   *string
	TimeLt    *string
	TimeDur   *float64
}

// GroupsInput 是 groups 和 group-stats 共用的查询条件。
type GroupsInput struct {
	TimeInput
	Service     *string
	Env         *string
	Query       *string
	Search      *string
	Limit       *float64
	DurationGte *float64
	DurationLt  *float64
}

// SpansInput 是 span 列表查询。trace 明细不走这里。
type SpansInput struct {
	TimeInput
	TraceID     *string
	SpanID      *float64
	ParentID    *float64
	Limit       *float64
	DurationGte *float64
	DurationLt  *float64
	Query       *string
	SortBy      *string
	SortDesc    *bool
	Page        *float64
}

// UpSpec 是一次 Uptrace 内部 API 调用。
type UpSpec struct {
	URL     string
	Query   form
	Headers map[string]string
}

// TimeRange 是已经换成 API 时间的窗口。
type TimeRange struct {
	Gte string
	Lt  string
}

// ParseTimeRange 把紧凑时间和时长换成 UTC 起止。now 只在两端都没给时才用。
func ParseTimeRange(in TimeInput, now time.Time) (TimeRange, error) {
	end := in.TimeEnd
	if end == nil {
		end = in.TimeLt
	}
	if in.TimeStart != nil && end != nil {
		gte, err := normalizeAPITime(*in.TimeStart)
		if err != nil {
			return TimeRange{}, err
		}
		lt, err := normalizeAPITime(*end)
		if err != nil {
			return TimeRange{}, err
		}
		return TimeRange{Gte: gte, Lt: lt}, nil
	}
	dur := defaultTimeDur
	if in.TimeDur != nil {
		dur = *in.TimeDur
	}
	if in.TimeGte != nil {
		start, err := parseLooseTime(*in.TimeGte)
		if err != nil {
			return TimeRange{}, err
		}
		return TimeRange{
			Gte: iso(start),
			Lt:  iso(start.Add(time.Duration(dur * float64(time.Second)))),
		}, nil
	}
	return TimeRange{
		Gte: iso(now.Add(-time.Duration(dur * float64(time.Second)))),
		Lt:  iso(now),
	}, nil
}

// BuildGroupsRequest 拼 groups。没给 UQL 时用服务加环境的默认聚合，这是排查慢接口的第一步。
func BuildGroupsRequest(cfg UptraceConfig, in GroupsInput) (UpSpec, error) {
	return buildGroupsLike(cfg, "groups", in)
}

// BuildGroupStatsRequest 在 groups 条件上固定追加页面上的指标列。
func BuildGroupStatsRequest(cfg UptraceConfig, in GroupsInput) (UpSpec, error) {
	spec, err := buildGroupsLike(cfg, "group-stats", in)
	if err != nil {
		return UpSpec{}, err
	}
	for _, col := range defaultStatCols {
		spec.Query.add("column[]", col)
	}
	return spec, nil
}

func buildGroupsLike(cfg UptraceConfig, kind string, in GroupsInput) (UpSpec, error) {
	in.TimeDur = coalesceDur(in.TimeDur, cfg.DefaultTimeDur)
	rng, err := ParseTimeRange(in.TimeInput, time.Now())
	if err != nil {
		return UpSpec{}, err
	}
	query := form{}
	addInternalTime(&query, rng)
	uql := buildDefaultGroupsUQL(strOr(in.Service), envOr(in.Env, cfg))
	if in.Query != nil {
		uql = *in.Query
	}
	query.set("query", uql)
	addNum(&query, "limit", in.Limit)
	addStr(&query, "search", in.Search)
	addNum(&query, "duration_gte", in.DurationGte)
	addNum(&query, "duration_lt", in.DurationLt)
	return buildUp(cfg, kind, query)
}

// BuildSpansRequest 拉 span 明细，用来从聚合结果落到具体 trace。
func BuildSpansRequest(cfg UptraceConfig, in SpansInput) (UpSpec, error) {
	in.TimeDur = coalesceDur(in.TimeDur, cfg.DefaultTimeDur)
	rng, err := ParseTimeRange(in.TimeInput, time.Now())
	if err != nil {
		return UpSpec{}, err
	}
	query := form{}
	addInternalTime(&query, rng)
	addStr(&query, "query", in.Query)
	addStr(&query, "trace_id", in.TraceID)
	addNum(&query, "id", in.SpanID)
	addNum(&query, "parent_id", in.ParentID)
	addNum(&query, "limit", in.Limit)
	addNum(&query, "duration_gte", in.DurationGte)
	addNum(&query, "duration_lt", in.DurationLt)
	addStr(&query, "sort_by", in.SortBy)
	if in.SortDesc != nil {
		query.set("sort_desc", strconvBool(*in.SortDesc))
	}
	addNum(&query, "page", in.Page)
	return buildUp(cfg, "spans", query)
}

func strconvBool(val bool) string {
	if val {
		return "true"
	}
	return "false"
}

// BuildTraceRequest 打浏览器实际用的内部 trace 接口，返回嵌套调用链。
func BuildTraceRequest(cfg UptraceConfig, traceID string) (UpSpec, error) {
	headers, err := upAuth(cfg)
	if err != nil {
		return UpSpec{}, err
	}
	spec := UpSpec{
		URL:     fmt.Sprintf("%s/internal/v1/tracing/%s/traces/%s", trimRightSlash(cfg.BaseURL), jsNum(cfg.ProjectID), traceID),
		Headers: headers,
	}
	return spec, nil
}

// AppendQuery 把 query 接到 URL 上。没有参数时不能多一个问号。
func AppendQuery(spec UpSpec) string {
	text := spec.Query.encode()
	if text == "" {
		return spec.URL
	}
	return spec.URL + "?" + text
}

// BuildDefaultGroupsUQL 固化压测常用视角，调用方仍可用 --query 覆盖。
func BuildDefaultGroupsUQL(service, env string) string {
	return buildDefaultGroupsUQL(service, env)
}

func buildDefaultGroupsUQL(service, env string) string {
	clauses := []string{
		"group by _group_id",
		"per_min(sum(_count))",
		"_error_rate",
		"{p50,p90,p99}(_duration)",
	}
	if env != "" {
		clauses = append(clauses, `where deployment_environment = "`+escapeUQL(env)+`"`)
	}
	if service != "" {
		clauses = append(clauses, `where service_name = "`+escapeUQL(service)+`"`)
	}
	return strings.Join(clauses, " | ")
}

func buildUp(cfg UptraceConfig, kind string, query form) (UpSpec, error) {
	headers, err := upAuth(cfg)
	if err != nil {
		return UpSpec{}, err
	}
	return UpSpec{
		URL:     fmt.Sprintf("%s/internal/v1/tracing/%s/%s", trimRightSlash(cfg.BaseURL), jsNum(cfg.ProjectID), kind),
		Query:   query,
		Headers: headers,
	}, nil
}

func addInternalTime(query *form, rng TimeRange) {
	query.set("time_gte", rng.Gte)
	query.set("time_lt", rng.Lt)
	query.set("system[]", "all")
}

func upAuth(cfg UptraceConfig) (map[string]string, error) {
	headers := map[string]string{"Accept": "application/json"}
	if cfg.HasJWT && cfg.JWTToken != "" {
		headers["Cookie"] = "token=" + cfg.JWTToken
		return headers, nil
	}
	if cfg.HasAuth && cfg.AuthToken != "" {
		headers["Authorization"] = "Bearer " + cfg.AuthToken
		return headers, nil
	}
	return nil, errors.New("config missing uptrace auth: expected auth_token or jwt_token")
}

func coalesceDur(in, def *float64) *float64 {
	if in != nil {
		return in
	}
	return def
}

func strOr(val *string) string {
	if val == nil {
		return ""
	}
	return *val
}

func envOr(in *string, cfg UptraceConfig) string {
	if in != nil {
		return *in
	}
	if cfg.HasDefaultEnv {
		return cfg.DefaultEnv
	}
	return ""
}

func addStr(query *form, key string, val *string) {
	if val != nil && *val != "" {
		query.set(key, *val)
	}
}

func addNum(query *form, key string, val *float64) {
	if val != nil {
		query.set(key, jsNum(*val))
	}
}

func escapeUQL(val string) string {
	val = strings.ReplaceAll(val, `\`, `\\`)
	return strings.ReplaceAll(val, `"`, `\"`)
}

var compactTime = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})$`)

func normalizeAPITime(val string) (string, error) {
	at, err := parseLooseTime(val)
	if err != nil {
		return "", err
	}
	return iso(at), nil
}

func parseLooseTime(val string) (time.Time, error) {
	if match := compactTime.FindStringSubmatch(val); match != nil {
		year := atoi(match[1])
		month := time.Month(atoi(match[2]))
		return time.Date(year, month, atoi(match[3]), atoi(match[4]), atoi(match[5]), atoi(match[6]), 0, time.UTC), nil
	}
	if at, err := time.Parse(time.RFC3339Nano, val); err == nil {
		return at, nil
	}
	return time.Time{}, fmt.Errorf("invalid time value: %s", val)
}

func atoi(val string) int {
	num := 0
	for _, ch := range val {
		num = num*10 + int(ch-'0')
	}
	return num
}

func iso(at time.Time) string {
	return at.UTC().Format("2006-01-02T15:04:05.000Z")
}

// UpGet 请求 Uptrace 并解析 JSON。非 2xx 要把状态和正文一起抛出。
func UpGet(ctx context.Context, client *http.Client, spec UpSpec) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AppendQuery(spec), nil)
	if err != nil {
		return nil, err
	}
	for key, val := range spec.Headers {
		req.Header.Set(key, val)
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(fmt.Sprintf("Uptrace API failed: %d %s %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(buf)))
		return nil, errors.New(msg)
	}
	if len(bytesTrim(buf)) == 0 {
		return nil, errors.New("uptrace response is empty")
	}
	val, err := decodeJSON(buf)
	if err != nil {
		return nil, fmt.Errorf("uptrace response is not json: %s", string(buf))
	}
	return val, nil
}

func bytesTrim(buf []byte) []byte {
	return []byte(strings.TrimSpace(string(buf)))
}

// ExtractGroups 兼容 {groups:[]} 和直接数组两种返回。
func ExtractGroups(body any) []any {
	if rec, ok := isRecord(body); ok {
		if list, ok := rec["groups"].([]any); ok {
			return list
		}
	}
	if list, ok := body.([]any); ok {
		return list
	}
	return []any{}
}

// ExtractSpans 兼容 {spans:[]} 和直接数组。
func ExtractSpans(body any) []any {
	if rec, ok := isRecord(body); ok {
		if list, ok := rec["spans"].([]any); ok {
			return list
		}
	}
	if list, ok := body.([]any); ok {
		return list
	}
	return []any{}
}

// ExtractTraceSpans 把内部接口的 root 树摊平。子节点上的 children 不能留在 span 上。
func ExtractTraceSpans(body any) []any {
	if rec, ok := isRecord(body); ok {
		if root, ok := isRecord(rec["root"]); ok {
			flat := flattenTrace(root)
			out := make([]any, len(flat))
			for i := range flat {
				out[i] = flat[i]
			}
			return out
		}
	}
	return ExtractSpans(body)
}

func flattenTrace(span map[string]any) []map[string]any {
	var kids []any
	if list, ok := span["children"].([]any); ok {
		kids = list
	}
	delete(span, "children")
	out := []map[string]any{span}
	for _, kid := range kids {
		if child, ok := isRecord(kid); ok {
			out = append(out, flattenTrace(child)...)
		}
	}
	return out
}

// FormatGroups 给 groups 做轻量可读化。字段随 UQL 变化，human 不猜列名。
func FormatGroups(body any, mode string) (string, error) {
	if mode == "plain" {
		return pretty(body)
	}
	groups := ExtractGroups(body)
	if mode == "agent" {
		return pretty(struct {
			Groups []any `json:"groups"`
		}{Groups: groups})
	}
	if len(groups) == 0 {
		return "No groups.\n", nil
	}
	lines := []string{"Groups:"}
	for _, group := range groups {
		buf, err := json.Marshal(group)
		if err != nil {
			return "", err
		}
		lines = append(lines, "- "+string(buf))
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// FormatJSON 是 plain 和 spans 共用的缩进 JSON。
func FormatJSON(body any) (string, error) {
	return pretty(body)
}

// ParsedTrace 是从裸 id、日志或页面 URL 里抽出的 trace。
type ParsedTrace struct {
	TraceID   string
	ProjectID *float64
}

var traceIDPat = regexp.MustCompile(`\b[a-fA-F0-9]{32}\b`)

// ParseTraceInput 把日志、浏览器 URL 和裸 trace id 收成同一个入口。
func ParseTraceInput(input string) (ParsedTrace, error) {
	trimmed := strings.TrimSpace(input)
	if parsed, ok, err := parseTraceURL(trimmed); ok || err != nil {
		return parsed, err
	}
	match := traceIDPat.FindString(trimmed)
	if match == "" {
		return ParsedTrace{}, errors.New("invalid trace id input: expected 32 hex trace id or Uptrace trace URL")
	}
	return ParsedTrace{TraceID: strings.ToLower(match)}, nil
}

func parseTraceURL(input string) (ParsedTrace, bool, error) {
	if !strings.Contains(input, "://") {
		return ParsedTrace{}, false, nil
	}
	u, err := url.Parse(input)
	if err != nil || u.Host == "" {
		return ParsedTrace{}, false, nil
	}
	parts := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })
	idx := -1
	for i, part := range parts {
		if part == "traces" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ParsedTrace{}, false, nil
	}
	if idx+2 >= len(parts) || !traceIDPat.MatchString(parts[idx+2]) {
		return ParsedTrace{}, true, errors.New("invalid Uptrace trace URL: expected /traces/{project_id}/{trace_id}")
	}
	project, err := parseProject(parts[idx+1])
	if err != nil {
		return ParsedTrace{}, true, err
	}
	return ParsedTrace{TraceID: strings.ToLower(parts[idx+2]), ProjectID: &project}, true, nil
}

func parseProject(val string) (float64, error) {
	num, err := strconv.ParseFloat(val, 64)
	if err != nil || math.IsInf(num, 0) || num != math.Trunc(num) {
		return 0, errors.New("invalid Uptrace trace URL: project id must be an integer")
	}
	return num, nil
}

// WithProject 用 URL 里的 project 覆盖配置。0 在 JS 里是假值，不能覆盖。
func WithProject(cfg UptraceConfig, project *float64) UptraceConfig {
	if project == nil || *project == 0 {
		return cfg
	}
	cfg.ProjectID = *project
	return cfg
}
