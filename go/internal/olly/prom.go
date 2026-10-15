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
	"strconv"
	"strings"
)

// PromQuery 是 /api/v1/query 的参数。空指针表示用户没传，不能和空字符串混用。
type PromQuery struct {
	Query   string
	Time    *string
	Timeout *string
	Limit   *float64
}

// PromRange 是 /api/v1/query_range 的参数。start、end、step 缺一不可。
type PromRange struct {
	Query   string
	Start   *string
	End     *string
	Step    *string
	Timeout *string
	Limit   *float64
}

// PromMeta 是 labels 和 series 共用的过滤参数。match 可重复。
type PromMeta struct {
	Matches []string
	Start   *string
	End     *string
	Limit   *float64
}

// PromSpec 是一次 Prometheus HTTP 调用。Body 为空表示 GET。
type PromSpec struct {
	Method string
	URL    string
	Body   string
}

// BuildQueryRequest 用 POST form 发 PromQL。表达式又长又带特殊字符，不能塞进 URL。
func BuildQueryRequest(cfg PrometheusConfig, in PromQuery) (PromSpec, error) {
	if err := requireText(in.Query, "query"); err != nil {
		return PromSpec{}, err
	}
	body := form{}
	body.set("query", in.Query)
	setOpt(&body, "time", in.Time)
	timeout := in.Timeout
	if timeout == nil && cfg.HasTimeout {
		timeout = &cfg.DefaultTimeout
	}
	setOpt(&body, "timeout", timeout)
	setNum(&body, "limit", in.Limit)
	return promPost(cfg, "/api/v1/query", body), nil
}

// BuildRangeRequest 组装 query_range。step 可以来自命令行，也可以来自配置里的默认值。
func BuildRangeRequest(cfg PrometheusConfig, in PromRange) (PromSpec, error) {
	if err := requireText(in.Query, "query"); err != nil {
		return PromSpec{}, err
	}
	if err := requirePtr(in.Start, "--start"); err != nil {
		return PromSpec{}, err
	}
	if err := requirePtr(in.End, "--end"); err != nil {
		return PromSpec{}, err
	}
	step := in.Step
	if step == nil && cfg.HasStep {
		step = &cfg.DefaultStep
	}
	if err := requirePtr(step, "--step"); err != nil {
		return PromSpec{}, err
	}
	body := form{}
	body.set("query", in.Query)
	body.set("start", *in.Start)
	body.set("end", *in.End)
	body.set("step", *step)
	timeout := in.Timeout
	if timeout == nil && cfg.HasTimeout {
		timeout = &cfg.DefaultTimeout
	}
	setOpt(&body, "timeout", timeout)
	setNum(&body, "limit", in.Limit)
	return promPost(cfg, "/api/v1/query_range", body), nil
}

// BuildLabelsRequest 组装 /api/v1/labels。
func BuildLabelsRequest(cfg PrometheusConfig, in PromMeta) PromSpec {
	return promPost(cfg, "/api/v1/labels", metaBody(in))
}

// BuildLabelValuesRequest 组装某个 label 的 values。label 名进路径，必须做百分号编码。
func BuildLabelValuesRequest(cfg PrometheusConfig, label string, in PromMeta) (PromSpec, error) {
	if err := requireText(label, "label name"); err != nil {
		return PromSpec{}, err
	}
	path := "/api/v1/label/" + url.PathEscape(label) + "/values"
	return promPost(cfg, path, metaBody(in)), nil
}

// BuildSeriesRequest 至少要一个 match，否则 Prometheus 会扫全库。
func BuildSeriesRequest(cfg PrometheusConfig, in PromMeta) (PromSpec, error) {
	if len(in.Matches) == 0 {
		return PromSpec{}, errors.New("prometheus query series requires at least one --match")
	}
	return promPost(cfg, "/api/v1/series", metaBody(in)), nil
}

// BuildReadyRequest 是 /-/ready，返回纯文本而不是 JSON envelope。
func BuildReadyRequest(cfg PrometheusConfig) PromSpec {
	return promGet(cfg, "/-/ready")
}

// BuildHealthyRequest 是 /-/healthy。
func BuildHealthyRequest(cfg PrometheusConfig) PromSpec {
	return promGet(cfg, "/-/healthy")
}

// BuildBuildInfoRequest 是 /api/v1/status/buildinfo。
func BuildBuildInfoRequest(cfg PrometheusConfig) PromSpec {
	return promGet(cfg, "/api/v1/status/buildinfo")
}

// BuildRuntimeInfoRequest 是 /api/v1/status/runtimeinfo。
func BuildRuntimeInfoRequest(cfg PrometheusConfig) PromSpec {
	return promGet(cfg, "/api/v1/status/runtimeinfo")
}

func metaBody(in PromMeta) form {
	body := form{}
	for _, match := range in.Matches {
		body.add("match[]", match)
	}
	setOpt(&body, "start", in.Start)
	setOpt(&body, "end", in.End)
	setNum(&body, "limit", in.Limit)
	return body
}

func promPost(cfg PrometheusConfig, path string, body form) PromSpec {
	return PromSpec{Method: http.MethodPost, URL: cfg.BaseURL + path, Body: body.encode()}
}

func promGet(cfg PrometheusConfig, path string) PromSpec {
	return PromSpec{Method: http.MethodGet, URL: cfg.BaseURL + path}
}

func setOpt(body *form, key string, val *string) {
	if val != nil {
		body.set(key, *val)
	}
}

func setNum(body *form, key string, val *float64) {
	if val != nil {
		body.set(key, jsNum(*val))
	}
}

func requireText(val, name string) error {
	if val == "" {
		return fmt.Errorf("prometheus %s is required", name)
	}
	return nil
}

func requirePtr(val *string, name string) error {
	if val == nil || *val == "" {
		return fmt.Errorf("prometheus %s is required", name)
	}
	return nil
}

// PromDo 执行一次 Prometheus 请求。plain 模式要原始 JSON，所以同时返回 body。
func PromDo(ctx context.Context, client *http.Client, spec PromSpec) (int, []byte, error) {
	var body io.Reader
	if spec.Body != "" {
		body = strings.NewReader(spec.Body)
	}
	req, err := http.NewRequestWithContext(ctx, spec.Method, spec.URL, body)
	if err != nil {
		return 0, nil, err
	}
	if spec.Method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, buf, nil
}

// PromText 读就绪类纯文本。没有换行时补一个，已经有换行则不动。
func PromText(ctx context.Context, client *http.Client, spec PromSpec) (string, error) {
	status, buf, err := PromDo(ctx, client, spec)
	if err != nil {
		return "", err
	}
	text := string(buf)
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("prometheus http %d: %s", status, text)
	}
	if strings.HasSuffix(text, "\n") {
		return text, nil
	}
	return text + "\n", nil
}

type promEnv struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	HasData   bool            `json:"-"`
	ErrorType string          `json:"errorType"`
	Error     string          `json:"error"`
	HasType   bool            `json:"-"`
	HasErr    bool            `json:"-"`
}

// ReadPromData 从成功 envelope 取出 data。失败 envelope 要带上 errorType，方便人对着 Prometheus 文档看。
func ReadPromData(status int, raw []byte) (json.RawMessage, error) {
	env, text, err := readPromEnv(status, raw)
	if err != nil {
		return nil, err
	}
	if env.Status == "success" {
		if !env.HasData {
			return nil, errors.New("prometheus response missing data")
		}
		return env.Data, nil
	}
	if env.Status == "error" {
		kind := "error"
		if env.HasType {
			kind = env.ErrorType
		}
		msg := "unknown prometheus error"
		if env.HasErr {
			msg = env.Error
		}
		return nil, fmt.Errorf("%s: %s", kind, msg)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("prometheus http %d: %s", status, text)
	}
	return nil, errors.New("prometheus response is not a valid envelope")
}

func readPromEnv(status int, raw []byte) (promEnv, string, error) {
	text := string(raw)
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return promEnv{}, text, fmt.Errorf("prometheus response is not json: %s", text)
	}
	stRaw, ok := probe["status"]
	if !ok {
		if status < 200 || status >= 300 {
			return promEnv{}, text, fmt.Errorf("prometheus http %d: %s", status, text)
		}
		return promEnv{}, text, errors.New("prometheus response is not a valid envelope")
	}
	var st string
	if err := json.Unmarshal(stRaw, &st); err != nil || (st != "success" && st != "error") {
		if status < 200 || status >= 300 {
			return promEnv{}, text, fmt.Errorf("prometheus http %d: %s", status, text)
		}
		return promEnv{}, text, errors.New("prometheus response is not a valid envelope")
	}
	env := promEnv{Status: st}
	if data, ok := probe["data"]; ok {
		env.Data = data
		env.HasData = true
	}
	if kind, ok := probe["errorType"]; ok {
		_ = json.Unmarshal(kind, &env.ErrorType)
		env.HasType = true
	}
	if msg, ok := probe["error"]; ok {
		_ = json.Unmarshal(msg, &env.Error)
		env.HasErr = true
	}
	if (status < 200 || status >= 300) && st != "error" {
		return promEnv{}, text, fmt.Errorf("prometheus http %d: %s", status, text)
	}
	return env, text, nil
}

// PromFormatCtx 决定 agent 摘要里要不要把原始点位带回去。默认不带，避免撑爆上下文。
type PromFormatCtx struct {
	Command       string
	Query         string
	HasQuery      bool
	IncludeValues bool
}

// PromPlain 在 plain 模式打印原始 envelope。非法 JSON 和 HTTP 错误要先变成和查询一样的错误，不能把语法错误直接抛出去。
func PromPlain(status int, raw []byte) (string, error) {
	if _, _, err := readPromEnv(status, raw); err != nil {
		return "", err
	}
	return prettyRaw(raw)
}

// FormatPromResult 把 envelope 打成 human、agent 或 plain。plain 必须是原始响应，不能丢掉 warnings。
func FormatPromResult(raw []byte, mode string, ctx PromFormatCtx) (string, error) {
	if mode == "plain" {
		return PromPlain(200, raw)
	}
	env, _, err := readPromEnv(200, raw)
	if err != nil {
		return "", err
	}
	if isQueryData(env.Data) {
		if mode == "agent" {
			return formatQueryAgent(env.Data, ctx)
		}
		return formatQueryHuman(env.Data)
	}
	return formatPromMeta(env.Data, mode)
}

// FormatPromData 给非 plain 用。调用方已经确认 status=success，这里只包一层 success 再格式化。
func FormatPromData(data json.RawMessage, mode string, ctx PromFormatCtx) (string, error) {
	wrapped := append([]byte(`{"status":"success","data":`), data...)
	wrapped = append(wrapped, '}')
	return FormatPromResult(wrapped, mode, ctx)
}

func isQueryData(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	_, hasType := probe["resultType"]
	_, hasResult := probe["result"]
	return hasType && hasResult
}

type ordMap struct {
	keys []string
	vals map[string]string
}

func (m ordMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		vb, err := json.Marshal(m.vals[key])
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (m *ordMap) UnmarshalJSON(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return errors.New("metric is not object")
	}
	m.vals = map[string]string{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyTok.(string)
		if !ok {
			return errors.New("metric key is not string")
		}
		var val string
		if err := dec.Decode(&val); err != nil {
			return err
		}
		if _, exists := m.vals[key]; !exists {
			m.keys = append(m.keys, key)
		}
		m.vals[key] = val
	}
	_, err = dec.Token()
	return err
}

func (m ordMap) format() string {
	parts := make([]string, 0, len(m.keys))
	for _, key := range m.keys {
		parts = append(parts, key+`="`+m.vals[key]+`"`)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

type promSample struct {
	at  float64
	val string
}

func (s promSample) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{s.at, s.val})
}

func (s *promSample) UnmarshalJSON(raw []byte) error {
	var pair []json.RawMessage
	if err := json.Unmarshal(raw, &pair); err != nil || len(pair) < 2 {
		return errors.New("bad sample")
	}
	if err := json.Unmarshal(pair[0], &s.at); err != nil {
		return err
	}
	return json.Unmarshal(pair[1], &s.val)
}

type promSeries struct {
	Metric ordMap       `json:"metric"`
	Value  *promSample  `json:"value"`
	Values []promSample `json:"values"`
}

type seriesSum struct {
	Metric      ordMap        `json:"metric"`
	SampleCount int           `json:"sample_count"`
	First       *promSample   `json:"first,omitempty"`
	Last        *promSample   `json:"last,omitempty"`
	Min         *float64      `json:"min,omitempty"`
	Max         *float64      `json:"max,omitempty"`
	Avg         *float64      `json:"avg,omitempty"`
	Values      *[]promSample `json:"values,omitempty"`
}

func formatQueryAgent(raw json.RawMessage, ctx PromFormatCtx) (string, error) {
	series := seriesOnly(raw)
	sums := make([]seriesSum, 0, len(series))
	total := 0
	for _, item := range series {
		sum := summarizeSeries(item, ctx.IncludeValues)
		total += sum.SampleCount
		sums = append(sums, sum)
	}
	var resultType string
	_ = json.Unmarshal(mustField(raw, "resultType"), &resultType)
	payload := struct {
		Command     string      `json:"command"`
		Query       string      `json:"query,omitempty"`
		ResultType  string      `json:"result_type"`
		SeriesCount int         `json:"series_count"`
		SampleCount int         `json:"sample_count"`
		Series      []seriesSum `json:"series"`
	}{
		Command:     ctx.Command,
		ResultType:  resultType,
		SeriesCount: len(sums),
		SampleCount: total,
		Series:      sums,
	}
	if ctx.HasQuery {
		payload.Query = ctx.Query
	}
	return pretty(payload)
}

func formatQueryHuman(raw json.RawMessage) (string, error) {
	result := mustField(raw, "result")
	if len(result) == 0 || result[0] != '[' {
		text, err := compactJSON(result)
		if err != nil {
			return "", err
		}
		return text + "\n", nil
	}
	var list []json.RawMessage
	if err := json.Unmarshal(result, &list); err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "No result.\n", nil
	}
	var resultType string
	_ = json.Unmarshal(mustField(raw, "resultType"), &resultType)
	lines := []string{"Prometheus " + resultType + ":"}
	for _, item := range list {
		var series promSeries
		if err := json.Unmarshal(item, &series); err != nil || !hasMetric(item) {
			text, err := compactJSON(item)
			if err != nil {
				return "", err
			}
			lines = append(lines, "- "+text)
			continue
		}
		if !hasMetric(item) {
			text, err := compactJSON(item)
			if err != nil {
				return "", err
			}
			lines = append(lines, "- "+text)
			continue
		}
		sum := summarizeSeries(series, false)
		label := series.Metric.format()
		if series.Value != nil {
			lines = append(lines, fmt.Sprintf("- %s value=%s time=%s", label, series.Value.val, jsNum(series.Value.at)))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s samples=%d first=%s last=%s min=%s max=%s avg=%s",
			label, sum.SampleCount, formatSample(sum.First), formatSample(sum.Last), fmtOpt(sum.Min), fmtOpt(sum.Max), fmtOpt(sum.Avg)))
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func hasMetric(raw json.RawMessage) bool {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	_, ok := probe["metric"]
	return ok
}

func seriesOnly(raw json.RawMessage) []promSeries {
	result := mustField(raw, "result")
	if len(result) == 0 || result[0] != '[' {
		return nil
	}
	var list []json.RawMessage
	if err := json.Unmarshal(result, &list); err != nil {
		return nil
	}
	var out []promSeries
	for _, item := range list {
		if !hasMetric(item) {
			continue
		}
		var series promSeries
		if err := json.Unmarshal(item, &series); err != nil {
			continue
		}
		out = append(out, series)
	}
	return out
}

func summarizeSeries(series promSeries, include bool) seriesSum {
	// values 优先于 value，空数组也算用户给了序列，不能再退回单点。
	samples := series.Values
	if samples == nil && series.Value != nil {
		samples = []promSample{*series.Value}
	}
	sum := seriesSum{Metric: series.Metric, SampleCount: len(samples)}
	if len(samples) > 0 {
		first := samples[0]
		last := samples[len(samples)-1]
		sum.First = &first
		sum.Last = &last
	}
	var nums []float64
	for _, sample := range samples {
		num, err := strconv.ParseFloat(sample.val, 64)
		if err == nil && !isBad(num) {
			nums = append(nums, num)
		}
	}
	if len(nums) > 0 {
		min, max := nums[0], nums[0]
		total := 0.0
		for _, num := range nums {
			if num < min {
				min = num
			}
			if num > max {
				max = num
			}
			total += num
		}
		avg := total / float64(len(nums))
		sum.Min = &min
		sum.Max = &max
		sum.Avg = &avg
	}
	if include {
		cp := append([]promSample(nil), samples...)
		sum.Values = &cp
	}
	return sum
}

func isBad(num float64) bool {
	return num != num || num > 1e308 || num < -1e308
}

func formatSample(sample *promSample) string {
	if sample == nil {
		return "-"
	}
	return sample.val + "@" + jsNum(sample.at)
}

func fmtOpt(num *float64) string {
	if num == nil {
		return "-"
	}
	return jsNum(*num)
}

func formatPromMeta(raw json.RawMessage, mode string) (string, error) {
	if len(raw) == 0 || raw[0] != '[' {
		return prettyRaw(append(bytes.TrimSpace(raw), '\n'))
	}
	// prettyRaw adds newline; raw may already be compact. Re-indent objects below.
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return "", err
	}
	if mode == "agent" {
		vals := make([]any, 0, len(list))
		for _, item := range list {
			var val any
			if err := json.Unmarshal(item, &val); err != nil {
				return "", err
			}
			vals = append(vals, val)
		}
		return pretty(struct {
			Count  int `json:"count"`
			Values any `json:"values"`
		}{Count: len(list), Values: vals})
	}
	lines := make([]string, 0, len(list))
	for _, item := range list {
		var text string
		if err := json.Unmarshal(item, &text); err == nil && len(item) > 0 && item[0] == '"' {
			lines = append(lines, text)
			continue
		}
		var metric ordMap
		if err := json.Unmarshal(item, &metric); err == nil && len(item) > 0 && item[0] == '{' {
			lines = append(lines, metric.format())
			continue
		}
		compact, err := compactJSON(item)
		if err != nil {
			return "", err
		}
		lines = append(lines, compact)
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func mustField(raw json.RawMessage, key string) json.RawMessage {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil
	}
	return probe[key]
}

func compactJSON(raw json.RawMessage) (string, error) {
	var val any
	if err := json.Unmarshal(raw, &val); err != nil {
		return "", err
	}
	buf, err := json.Marshal(val)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}
