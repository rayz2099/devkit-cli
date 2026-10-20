package olly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/grafana"
)

const (
	defaultIntervalMs = 15000
	defaultMaxPts     = 1100
	humanTopLimit     = 10
)

// DashRef 继续留在 olly，分析和测试不用改 import。定位和 HTTP 已经抽到 grafana。
type DashRef = grafana.DashRef

// ParseDashRef 只从粘贴的 URL 里拿 uid、orgId 和时间窗。
func ParseDashRef(source string) (DashRef, error) {
	return grafana.ParseDashRef(source)
}

// BuildDashURL 给人点的链接必须用粘贴 URL 的 origin，不能把内网 API 地址暴露成入口。
func BuildDashURL(source, webBase, path, uid string, orgID *int, from, to string) (string, error) {
	return grafana.BuildDashURL(source, webBase, path, uid, orgID, from, to)
}

// Panel 是看板里一个可查询的面板。row 只是容器，本身不查。
type Panel struct {
	ID            *int
	Title         string
	Type          string
	Datasource    *DsRef
	Targets       []Target
	Panels        []Panel
	MaxDataPoints *int
}

// DsRef 必须是 prometheus 对象。字符串数据源一律拒绝，避免打到别的库。
type DsRef struct {
	Type string
	UID  string
	Bad  bool
}

// Target 是面板上的一条 PromQL。hide 的不查。
type Target struct {
	RefID        *string
	Expr         *string
	Hide         bool
	HasRange     bool
	Range        bool
	HasInstant   bool
	Instant      bool
	LegendFormat *string
	Datasource   *DsRef
}

// QueryBody 是 /api/ds/query 的正文。
type QueryBody struct {
	From    string      `json:"from"`
	To      string      `json:"to"`
	Queries []QueryItem `json:"queries"`
}

// QueryItem 是一条已经展开默认 interval 的查询。
type QueryItem struct {
	RefID         string  `json:"refId"`
	Datasource    dsJSON  `json:"datasource"`
	Expr          string  `json:"expr"`
	Range         bool    `json:"range"`
	Instant       bool    `json:"instant"`
	IntervalMs    int     `json:"intervalMs"`
	MaxDataPoints int     `json:"maxDataPoints"`
	LegendFormat  *string `json:"legendFormat,omitempty"`
}

type dsJSON struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

// CollectPanels 展开折叠 row。不展开就读不到嵌在里面的图。
func CollectPanels(panels []Panel) []Panel {
	var out []Panel
	for _, panel := range panels {
		if panel.Type == "row" {
			out = append(out, CollectPanels(panel.Panels)...)
			continue
		}
		out = append(out, panel)
	}
	return out
}

func visibleTargets(panel Panel) []Target {
	var out []Target
	for _, target := range panel.Targets {
		if target.Hide {
			continue
		}
		out = append(out, target)
	}
	return out
}

// BuildQueryBody 把面板收成 Grafana 查询。非 Prometheus 数据源直接失败。
func BuildQueryBody(panel Panel, from, to string) (QueryBody, error) {
	targets := visibleTargets(panel)
	if len(targets) == 0 {
		return QueryBody{}, fmt.Errorf("grafana panel %s has no query", panelLabel(panel))
	}
	items := make([]QueryItem, 0, len(targets))
	for i, target := range targets {
		item, err := toQueryItem(panel, target, i)
		if err != nil {
			return QueryBody{}, err
		}
		items = append(items, item)
	}
	return QueryBody{From: from, To: to, Queries: items}, nil
}

func toQueryItem(panel Panel, target Target, index int) (QueryItem, error) {
	expr := ""
	if target.Expr != nil {
		expr = strings.TrimSpace(*target.Expr)
	}
	if expr == "" {
		return QueryItem{}, fmt.Errorf("grafana panel %s target missing expr", panelLabel(panel))
	}
	ds, err := promDs(panel, target)
	if err != nil {
		return QueryItem{}, err
	}
	isRange := target.HasRange && target.Range
	if !isRange && !(target.HasInstant && target.Instant) && isRangePanel(panel.Type) {
		isRange = true
	}
	instant := (target.HasInstant && target.Instant) || !isRange
	ref := string(rune('A' + index))
	if target.RefID != nil && strings.TrimSpace(*target.RefID) != "" {
		ref = strings.TrimSpace(*target.RefID)
	}
	maxPts := defaultMaxPts
	if panel.MaxDataPoints != nil {
		maxPts = *panel.MaxDataPoints
	}
	return QueryItem{
		RefID:         ref,
		Datasource:    ds,
		Expr:          expr,
		Range:         isRange,
		Instant:       instant,
		IntervalMs:    defaultIntervalMs,
		MaxDataPoints: maxPts,
		LegendFormat:  target.LegendFormat,
	}, nil
}

func promDs(panel Panel, target Target) (dsJSON, error) {
	ds := target.Datasource
	if ds == nil {
		ds = panel.Datasource
	}
	if ds == nil || ds.Bad || ds.Type != "prometheus" || ds.UID == "" {
		return dsJSON{}, fmt.Errorf("grafana panel %s datasource is not prometheus", panelLabel(panel))
	}
	return dsJSON{Type: "prometheus", UID: ds.UID}, nil
}

func isRangePanel(kind string) bool {
	return kind == "timeseries" || kind == "graph" || kind == "heatmap" || kind == "bargauge"
}

func panelLabel(panel Panel) string {
	if panel.ID != nil {
		return strconv.Itoa(*panel.ID)
	}
	if panel.Title != "" {
		return panel.Title
	}
	return "<unknown>"
}

// SeriesSum 只留 last 和 max。原始点位不进 agent 上下文。
type SeriesSum struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	SampleCount int               `json:"sample_count"`
	Last        *float64          `json:"last,omitempty"`
	LastTime    *float64          `json:"last_time,omitempty"`
	Max         *float64          `json:"max,omitempty"`
}

// TargetSum 是一个 refId 下的序列摘要。
type TargetSum struct {
	RefID       string      `json:"refId"`
	Expr        string      `json:"expr"`
	SeriesCount int         `json:"series_count"`
	Series      []SeriesSum `json:"series"`
}

// PanelSum 是一个面板的查询结果。
type PanelSum struct {
	ID      *int        `json:"id,omitempty"`
	Title   string      `json:"title"`
	Type    string      `json:"type"`
	Targets []TargetSum `json:"targets"`
}

// AnalyzeResult 是一次看板分析。plain 还要原始 dashboard 和发出去的 query。
type AnalyzeResult struct {
	Title      string          `json:"title"`
	UID        string          `json:"uid"`
	URL        string          `json:"url,omitempty"`
	OrgID      *int            `json:"orgId,omitempty"`
	Time       dashTime        `json:"time"`
	PanelCount int             `json:"panel_count,omitempty"`
	Panels     []PanelSum      `json:"panels"`
	Dashboard  json.RawMessage `json:"-"`
	Queries    []QueryBody     `json:"-"`
}

type dashTime struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ReadTargetSums 把 /api/ds/query 的 frames 收成 last/max。任一 ref 失败就整次失败。
func ReadTargetSums(body QueryBody, response map[string]any) ([]TargetSum, error) {
	results, ok := isRecord(response["results"])
	if !ok {
		return nil, errors.New("grafana query response missing results")
	}
	out := make([]TargetSum, 0, len(body.Queries))
	for _, query := range body.Queries {
		raw, ok := results[query.RefID]
		if !ok {
			return nil, fmt.Errorf("grafana query %s missing result", query.RefID)
		}
		result, ok := isRecord(raw)
		if !ok {
			return nil, fmt.Errorf("grafana query %s missing result", query.RefID)
		}
		if text, ok := result["error"].(string); ok && text != "" {
			return nil, fmt.Errorf("grafana query %s: %s", query.RefID, text)
		}
		if status, ok := asFloat(result["status"]); ok && status >= 400 {
			return nil, fmt.Errorf("grafana query %s status %s", query.RefID, jsNum(status))
		}
		frames, _ := result["frames"].([]any)
		series := summarizeFrames(frames)
		out = append(out, TargetSum{
			RefID:       query.RefID,
			Expr:        query.Expr,
			SeriesCount: len(series),
			Series:      series,
		})
	}
	return out, nil
}

func summarizeFrames(frames []any) []SeriesSum {
	var series []SeriesSum
	for _, item := range frames {
		frame, ok := isRecord(item)
		if !ok {
			continue
		}
		schema, _ := isRecord(frame["schema"])
		fields, _ := schema["fields"].([]any)
		data, _ := isRecord(frame["data"])
		values, _ := data["values"].([]any)
		timeIdx := -1
		for i, field := range fields {
			rec, _ := isRecord(field)
			if rec["type"] == "time" {
				timeIdx = i
				break
			}
		}
		var timeVals []any
		if timeIdx >= 0 && timeIdx < len(values) {
			timeVals, _ = values[timeIdx].([]any)
		}
		frameName, _ := schema["name"].(string)
		for i, field := range fields {
			rec, _ := isRecord(field)
			if rec["type"] != "number" {
				continue
			}
			var points []any
			if i < len(values) {
				points, _ = values[i].([]any)
			}
			sum := summarizePoints(points, timeVals)
			labels := map[string]string{}
			if raw, ok := isRecord(rec["labels"]); ok {
				for key, val := range raw {
					if text, ok := val.(string); ok {
						labels[key] = text
					}
				}
			}
			name := frameName
			if name == "" {
				name = formatLabels(labels)
			}
			if cfg, ok := isRecord(rec["config"]); ok {
				if text, ok := cfg["displayNameFromDS"].(string); ok && text != "" {
					name = text
				}
			}
			series = append(series, SeriesSum{
				Name:        name,
				Labels:      labels,
				SampleCount: sum.count,
				Last:        sum.last,
				LastTime:    sum.lastTime,
				Max:         sum.max,
			})
		}
	}
	if series == nil {
		series = []SeriesSum{}
	}
	return series
}

type pointSum struct {
	count    int
	last     *float64
	lastTime *float64
	max      *float64
}

func summarizePoints(points, times []any) pointSum {
	var out pointSum
	for i, point := range points {
		num, ok := asFloat(point)
		if !ok || math.IsNaN(num) || math.IsInf(num, 0) {
			continue
		}
		out.count++
		val := num
		out.last = &val
		if i < len(times) {
			if at, ok := asFloat(times[i]); ok && !math.IsNaN(at) && !math.IsInf(at, 0) {
				stamp := at
				out.lastTime = &stamp
			}
		}
		if out.max == nil || num > *out.max {
			peak := num
			out.max = &peak
		}
	}
	return out
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+`="`+labels[key]+`"`)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// GrafGet 读看板定义。org 通过头传递，不放进 URL。
func GrafGet(ctx context.Context, client *http.Client, cfg GrafanaConfig, uid string, orgID *int) (json.RawMessage, map[string]any, error) {
	return grafana.Get(ctx, client, cfg, uid, orgID)
}

// GrafQuery 执行面板查询。
func GrafQuery(ctx context.Context, client *http.Client, cfg GrafanaConfig, body QueryBody, orgID *int) (map[string]any, error) {
	buf, err := marshalCompact(body)
	if err != nil {
		return nil, err
	}
	_, rec, err := grafana.Do(ctx, client, cfg, http.MethodPost, cfg.BaseURL+"/api/ds/query", orgID, buf)
	return rec, err
}

// AnalyzeInput 是 grafana 命令的入口。from/to/org 可以覆盖 URL 和看板默认时间。
type AnalyzeInput struct {
	Source     string
	WebBaseURL string
	From       *string
	To         *string
	OrgID      *int
}

// AnalyzeDashboard 拉看板定义并执行每个 Prometheus 面板。只返回 PromQL 不够，agent 要看当前值。
func AnalyzeDashboard(ctx context.Context, client *http.Client, cfg GrafanaConfig, in AnalyzeInput) (AnalyzeResult, error) {
	ref, err := ParseDashRef(in.Source)
	if err != nil {
		return AnalyzeResult{}, err
	}
	orgID := in.OrgID
	if orgID == nil {
		orgID = ref.OrgID
	}
	raw, payload, err := GrafGet(ctx, client, cfg, ref.UID, orgID)
	if err != nil {
		return AnalyzeResult{}, err
	}
	dash, panels, dashUID, title, from, to, metaURL, err := readDash(payload)
	if err != nil {
		return AnalyzeResult{}, err
	}
	_ = dash
	if in.From != nil {
		from = *in.From
	} else if ref.From != nil {
		from = *ref.From
	}
	if in.To != nil {
		to = *in.To
	} else if ref.To != nil {
		to = *ref.To
	}
	if from == "" || to == "" {
		return AnalyzeResult{}, errors.New("grafana dashboard time range is required")
	}
	if dashUID == "" {
		return AnalyzeResult{}, errors.New("grafana dashboard missing uid")
	}
	var queries []QueryBody
	var sums []PanelSum
	for _, panel := range CollectPanels(panels) {
		body, err := BuildQueryBody(panel, from, to)
		if err != nil {
			return AnalyzeResult{}, err
		}
		response, err := GrafQuery(ctx, client, cfg, body, orgID)
		if err != nil {
			return AnalyzeResult{}, err
		}
		targets, err := ReadTargetSums(body, response)
		if err != nil {
			return AnalyzeResult{}, err
		}
		queries = append(queries, body)
		titleText := panel.Title
		if titleText == "" {
			idText := "unknown"
			if panel.ID != nil {
				idText = strconv.Itoa(*panel.ID)
			}
			titleText = "panel-" + idText
		}
		kind := panel.Type
		if kind == "" {
			kind = "unknown"
		}
		sums = append(sums, PanelSum{ID: panel.ID, Title: titleText, Type: kind, Targets: targets})
	}
	if sums == nil {
		sums = []PanelSum{}
	}
	link, err := BuildDashURL(in.Source, in.WebBaseURL, metaURL, dashUID, orgID, from, to)
	if err != nil {
		return AnalyzeResult{}, err
	}
	if title == "" {
		title = dashUID
	}
	return AnalyzeResult{
		Title:     title,
		UID:       dashUID,
		URL:       link,
		OrgID:     orgID,
		Time:      dashTime{From: from, To: to},
		Panels:    sums,
		Dashboard: raw,
		Queries:   queries,
	}, nil
}

func readDash(payload map[string]any) (map[string]any, []Panel, string, string, string, string, string, error) {
	dash, ok := isRecord(payload["dashboard"])
	if !ok {
		return nil, nil, "", "", "", "", "", errors.New("grafana dashboard missing uid")
	}
	uid, _ := dash["uid"].(string)
	title, _ := dash["title"].(string)
	from, to := "", ""
	if timeRec, ok := isRecord(dash["time"]); ok {
		from, _ = timeRec["from"].(string)
		to, _ = timeRec["to"].(string)
	}
	panels, err := parsePanels(dash["panels"])
	if err != nil {
		return nil, nil, "", "", "", "", "", err
	}
	metaURL := ""
	if meta, ok := isRecord(payload["meta"]); ok {
		metaURL, _ = meta["url"].(string)
	}
	return dash, panels, uid, title, from, to, metaURL, nil
}

func parsePanels(val any) ([]Panel, error) {
	list, ok := val.([]any)
	if !ok {
		return nil, nil
	}
	out := make([]Panel, 0, len(list))
	for _, item := range list {
		rec, ok := isRecord(item)
		if !ok {
			continue
		}
		panel, err := parsePanel(rec)
		if err != nil {
			return nil, err
		}
		out = append(out, panel)
	}
	return out, nil
}

func parsePanel(rec map[string]any) (Panel, error) {
	panel := Panel{}
	if num, ok := asFloat(rec["id"]); ok {
		id := int(num)
		panel.ID = &id
	}
	panel.Title, _ = rec["title"].(string)
	panel.Type, _ = rec["type"].(string)
	if _, ok := rec["datasource"]; ok && rec["datasource"] != nil {
		ds, err := parseDs(rec["datasource"])
		if err != nil {
			return Panel{}, err
		}
		panel.Datasource = &ds
	}
	if num, ok := asFloat(rec["maxDataPoints"]); ok {
		pts := int(num)
		panel.MaxDataPoints = &pts
	}
	if targets, ok := rec["targets"].([]any); ok {
		for _, item := range targets {
			trec, ok := isRecord(item)
			if !ok {
				continue
			}
			target, err := parseTarget(trec)
			if err != nil {
				return Panel{}, err
			}
			panel.Targets = append(panel.Targets, target)
		}
	}
	nested, err := parsePanels(rec["panels"])
	if err != nil {
		return Panel{}, err
	}
	panel.Panels = nested
	return panel, nil
}

func parseTarget(rec map[string]any) (Target, error) {
	target := Target{}
	if text, ok := rec["refId"].(string); ok {
		target.RefID = &text
	}
	if text, ok := rec["expr"].(string); ok {
		target.Expr = &text
	}
	if rec["hide"] == true {
		target.Hide = true
	}
	if _, ok := rec["range"]; ok {
		target.HasRange = true
		target.Range = rec["range"] == true
	}
	if _, ok := rec["instant"]; ok {
		target.HasInstant = true
		target.Instant = rec["instant"] == true
	}
	if text, ok := rec["legendFormat"].(string); ok {
		target.LegendFormat = &text
	}
	if _, ok := rec["datasource"]; ok && rec["datasource"] != nil {
		ds, err := parseDs(rec["datasource"])
		if err != nil {
			return Target{}, err
		}
		target.Datasource = &ds
	}
	return target, nil
}

func parseDs(val any) (DsRef, error) {
	if _, ok := val.(string); ok {
		return DsRef{Bad: true}, nil
	}
	rec, ok := isRecord(val)
	if !ok {
		return DsRef{Bad: true}, nil
	}
	kind, _ := rec["type"].(string)
	uid, _ := rec["uid"].(string)
	return DsRef{Type: kind, UID: uid}, nil
}

// FormatGrafana 按模式输出看板摘要。plain 保留原始 dashboard，方便核对请求。
func FormatGrafana(result AnalyzeResult, mode string) (string, error) {
	if mode == "plain" {
		return formatGrafPlain(result)
	}
	if mode == "agent" {
		result.PanelCount = len(result.Panels)
		return pretty(result)
	}
	return formatGrafHuman(result), nil
}

func formatGrafPlain(result AnalyzeResult) (string, error) {
	timeBuf, err := marshalCompact(result.Time)
	if err != nil {
		return "", err
	}
	queryBuf, err := marshalCompact(result.Queries)
	if err != nil {
		return "", err
	}
	dash := result.Dashboard
	if len(dash) == 0 {
		dash = []byte("null")
	}
	raw := append([]byte(`{"dashboard":`), dash...)
	raw = append(raw, []byte(`,"time":`)...)
	raw = append(raw, timeBuf...)
	raw = append(raw, []byte(`,"queries":`)...)
	raw = append(raw, queryBuf...)
	raw = append(raw, '}')
	return prettyRaw(raw)
}

func formatGrafHuman(result AnalyzeResult) string {
	count := 0
	for _, panel := range result.Panels {
		for _, target := range panel.Targets {
			count += target.SeriesCount
		}
	}
	urlText := result.URL
	if urlText == "" {
		urlText = "-"
	}
	lines := []string{
		"Grafana: " + result.Title,
		"URL: " + urlText,
		fmt.Sprintf("Time: %s → %s", result.Time.From, result.Time.To),
		fmt.Sprintf("Panels: %d  Series: %d", len(result.Panels), count),
	}
	if len(result.Panels) == 0 {
		lines = append(lines, "No panels.")
		return strings.Join(lines, "\n") + "\n"
	}
	for _, panel := range result.Panels {
		id := "-"
		if panel.ID != nil {
			id = strconv.Itoa(*panel.ID)
		}
		lines = append(lines, "", "#"+id+" "+panel.Title)
		for _, target := range panel.Targets {
			lines = append(lines, fmt.Sprintf("  %s  %s  series=%d", target.RefID, target.Expr, target.SeriesCount))
			ranked := rankSeries(target)
			if len(ranked) == 0 {
				lines = append(lines, "    (no series)")
				continue
			}
			if target.SeriesCount > humanTopLimit {
				lines = append(lines, fmt.Sprintf("    top %d by max:", humanTopLimit))
			}
			for _, series := range ranked {
				lines = append(lines, "    "+formatSeries(series))
			}
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func rankSeries(target TargetSum) []SeriesSum {
	cp := append([]SeriesSum(nil), target.Series...)
	sort.SliceStable(cp, func(i, j int) bool {
		return maxOr(cp[i].Max) > maxOr(cp[j].Max)
	})
	if len(cp) > humanTopLimit {
		cp = cp[:humanTopLimit]
	}
	return cp
}

func maxOr(val *float64) float64 {
	if val == nil {
		return math.Inf(-1)
	}
	return *val
}

func formatSeries(series SeriesSum) string {
	return fmt.Sprintf("%s  last=%s max=%s samples=%d", series.Name, fmtPtr(series.Last), fmtPtr(series.Max), series.SampleCount)
}

func fmtPtr(val *float64) string {
	if val == nil {
		return "-"
	}
	return jsNum(*val)
}
