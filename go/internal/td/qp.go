package td

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// 为什么: 属性分析和事件分析的页面 QP 不是 Open API 形状, 计算接口吃的是 events/eventView.
const uiJSON = `{"tableSorts":[],"chartSort":"num-desc","showChartLabel":false,"showChartPercent":true,"topNOptions":10}`

var eventDesc = map[string]string{
	"A100": "总次数",
	"A101": "触发用户数",
	"A102": "人均次数",
}

type groupCol struct {
	ColumnName    string `json:"columnName"`
	ColumnDesc    string `json:"columnDesc"`
	TableType     string `json:"tableType"`
	PropertyRange string `json:"propertyRange"`
}

type eventItem struct {
	Analysis         string `json:"analysis"`
	AnalysisDesc     string `json:"analysisDesc"`
	EventName        string `json:"eventName"`
	EventDesc        string `json:"eventDesc"`
	EventType        string `json:"eventType"`
	EventNameDisplay string `json:"eventNameDisplay"`
	Filts            []any  `json:"filts"`
	Quota            string `json:"quota"`
	QuotaDesc        string `json:"quotaDesc"`
	Relation         int    `json:"relation"`
	Type             int    `json:"type"`
}

type eventView struct {
	StartTime        string     `json:"startTime"`
	EndTime          string     `json:"endTime"`
	RecentDay        string     `json:"recentDay"`
	TimeParticleSize string     `json:"timeParticleSize"`
	GraphShape       string     `json:"graphShape"`
	GroupBy          []groupCol `json:"groupBy"`
	Filts            []any      `json:"filts"`
	Relation         int        `json:"relation"`
	ComparedByTime   bool       `json:"comparedByTime"`
	RowSpanType      string     `json:"rowSpanType"`
	UICommonConfig   string     `json:"uiCommonConfig"`
}

type eventDoc struct {
	Events    []eventItem `json:"events"`
	EventView eventView   `json:"eventView"`
	Origin    int         `json:"origin"`
}

type propItem struct {
	Analysis         string `json:"analysis"`
	AnalysisDesc     string `json:"analysisDesc"`
	EventNameDisplay string `json:"eventNameDisplay"`
	Filts            []any  `json:"filts"`
	Quota            string `json:"quota"`
	QuotaDesc        string `json:"quotaDesc"`
	Relation         int    `json:"relation"`
	TableType        string `json:"tableType"`
}

type propView struct {
	GraphShape     string     `json:"graphShape"`
	GroupBy        []groupCol `json:"groupBy"`
	UICommonConfig string     `json:"uiCommonConfig"`
	UserCrowds     []any      `json:"userCrowds"`
	RecentDay      *string    `json:"recentDay,omitempty"`
	StartTime      *string    `json:"startTime,omitempty"`
	EndTime        *string    `json:"endTime,omitempty"`
}

type propDoc struct {
	Events    []propItem `json:"events"`
	EventView propView   `json:"eventView"`
	Origin    int        `json:"origin"`
}

type eventIn struct {
	EventName string
	Analysis  string
	Span      Span
	Particle  string
	Group     *string
}

type propIn struct {
	Analysis string
	Quota    string
	Group    *string
	Span     *Span
}

// parseGroup 带上表类型. 计算接口靠 tableType 区分事件属性和用户属性.
func parseGroup(raw, table string) (groupCol, error) {
	kind := table
	name := raw
	idx := strings.Index(raw, ":")
	if idx > 0 {
		head := raw[:idx]
		name = raw[idx+1:]
		switch head {
		case "event":
			kind = "0"
		case "user":
			kind = "1"
		default:
			return groupCol{}, fmt.Errorf("unknown group table: %s", head)
		}
	}
	if strings.TrimSpace(name) == "" {
		return groupCol{}, fmt.Errorf("group name is empty")
	}
	return groupCol{
		ColumnName:    name,
		ColumnDesc:    name,
		TableType:     kind,
		PropertyRange: "",
	}, nil
}

func propertyQP(in propIn) (json.RawMessage, error) {
	desc := in.Analysis
	if in.Analysis == "A101" {
		desc = "用户数"
	}
	groups, err := groupsOf(in.Group, "1")
	if err != nil {
		return nil, err
	}
	view := propView{
		GraphShape:     "L1",
		GroupBy:        groups,
		UICommonConfig: uiJSON,
		UserCrowds:     []any{},
	}
	if in.Span != nil {
		recent := in.Span.RecentDay
		start := in.Span.StartTime
		end := in.Span.EndTime
		view.RecentDay = &recent
		view.StartTime = &start
		view.EndTime = &end
	}
	doc := propDoc{
		Events: []propItem{{
			Analysis:         in.Analysis,
			AnalysisDesc:     desc,
			EventNameDisplay: "",
			Filts:            []any{},
			Quota:            in.Quota,
			QuotaDesc:        "",
			Relation:         1,
			TableType:        "1",
		}},
		EventView: view,
		Origin:    1,
	}
	return marshalCompact(doc)
}

func eventQP(in eventIn) (json.RawMessage, error) {
	desc, ok := eventDesc[in.Analysis]
	if !ok {
		desc = in.Analysis
	}
	evDesc := in.EventName
	if in.EventName == "anyEvent" {
		evDesc = "任意事件"
	}
	groups, err := groupsOf(in.Group, "0")
	if err != nil {
		return nil, err
	}
	doc := eventDoc{
		Events: []eventItem{{
			Analysis:         in.Analysis,
			AnalysisDesc:     desc,
			EventName:        in.EventName,
			EventDesc:        evDesc,
			EventType:        "event",
			EventNameDisplay: "",
			Filts:            []any{},
			Quota:            "",
			QuotaDesc:        "",
			Relation:         1,
			Type:             0,
		}},
		EventView: eventView{
			StartTime:        in.Span.StartTime,
			EndTime:          in.Span.EndTime,
			RecentDay:        in.Span.RecentDay,
			TimeParticleSize: in.Particle,
			GraphShape:       "L0",
			GroupBy:          groups,
			Filts:            []any{},
			Relation:         1,
			ComparedByTime:   false,
			RowSpanType:      "fold",
			UICommonConfig:   uiJSON,
		},
		Origin: 1,
	}
	return marshalCompact(doc)
}

func groupsOf(group *string, table string) ([]groupCol, error) {
	if group == nil {
		return []groupCol{}, nil
	}
	col, err := parseGroup(*group, table)
	if err != nil {
		return nil, err
	}
	return []groupCol{col}, nil
}

// assertReadModel 拒绝 SQL 和热力图. 统计命令只放行页面上的只读分析模型.
func assertReadModel(model int) error {
	switch model {
	case 0, 1, 2, 3, 4, 7, 12, 14:
		return nil
	default:
		return fmt.Errorf("model %d is not a read-only statistic", model)
	}
}

func modelPrefix(model int) (string, error) {
	prefixes := map[int]string{
		0:  "WS_EVENT",
		1:  "WS_RETENTION",
		2:  "WS_FUNNEL",
		3:  "WS_PROPERTY",
		4:  "WS_TRACE",
		7:  "WS_SCATTER",
		12: "WS_INTERVAL",
		14: "WS_ATTRIBUTION",
	}
	prefix, ok := prefixes[model]
	if !ok {
		return "", fmt.Errorf("model %d is not a read-only statistic", model)
	}
	return prefix, nil
}

// takeReport 用报表自己的模型和 QP. 旗标里的模型只用来核对, 不能另造条件.
func takeReport(raw json.RawMessage, modelFlag *int) (int, json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return 0, nil, fmt.Errorf("report has no qp")
	}
	from, hasModel, err := reportModel(obj)
	if err != nil {
		return 0, nil, err
	}
	if hasModel && modelFlag != nil && from != *modelFlag {
		return 0, nil, fmt.Errorf("report model %d does not match --model %d", from, *modelFlag)
	}
	var model int
	switch {
	case hasModel:
		model = from
	case modelFlag != nil:
		model = *modelFlag
	default:
		return 0, nil, fmt.Errorf("report has no reportModel")
	}
	if err := assertReadModel(model); err != nil {
		return 0, nil, err
	}
	qp, err := takeQP(obj)
	if err != nil {
		return 0, nil, err
	}
	return model, qp, nil
}

func reportModel(obj map[string]json.RawMessage) (int, bool, error) {
	for _, key := range []string{"reportModel", "eventModel"} {
		raw, ok := obj[key]
		if !ok {
			continue
		}
		n, ok := rawInt(raw)
		if ok {
			return n, true, nil
		}
	}
	return 0, false, nil
}

func rawInt(raw json.RawMessage) (int, bool) {
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, false
	}
	i, err := n.Int64()
	if err != nil || i > maxSafeInt || i < -maxSafeInt {
		return 0, false
	}
	return int(i), true
}

func takeQP(obj map[string]json.RawMessage) (json.RawMessage, error) {
	if raw, ok := obj["qp"]; ok {
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			return compactRaw([]byte(text))
		}
		if len(raw) > 0 && raw[0] == '{' {
			return compactRaw(raw)
		}
	}
	events, hasEvents := obj["events"]
	view, hasView := obj["eventView"]
	if hasEvents && hasView {
		origin := json.RawMessage("1")
		if raw, ok := obj["origin"]; ok && string(raw) != "null" {
			origin = raw
		}
		var buf bytes.Buffer
		buf.WriteString(`{"events":`)
		buf.Write(events)
		buf.WriteString(`,"eventView":`)
		buf.Write(view)
		buf.WriteString(`,"origin":`)
		buf.Write(origin)
		buf.WriteByte('}')
		return compactRaw(buf.Bytes())
	}
	return nil, fmt.Errorf("report has no qp")
}

func compactRaw(buf []byte) (json.RawMessage, error) {
	var out bytes.Buffer
	if err := json.Compact(&out, buf); err != nil {
		return nil, fmt.Errorf("report has no qp")
	}
	return out.Bytes(), nil
}

func marshalCompact(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return compactRaw(bytes.TrimSpace(buf.Bytes()))
}
