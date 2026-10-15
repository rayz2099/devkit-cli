package td

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type usageBody struct {
	Usage string `json:"usage"`
}

type loginView struct {
	Profile  string `json:"profile"`
	URL      string `json:"url"`
	Username string `json:"username"`
	OK       bool   `json:"ok"`
}

type statusView struct {
	Profile  string `json:"profile"`
	URL      string `json:"url"`
	Username string `json:"username"`
}

type okEnv struct {
	OK   bool `json:"ok"`
	Data any  `json:"data"`
}

// emitRaw 让 agent 拿到稳定 JSON, human 才用对齐文本.
func emitRaw(audience string, raw json.RawMessage, text string) (string, error) {
	if audience == "agent" {
		return agentRaw(raw)
	}
	return humanText(text), nil
}

func emitValue(audience string, data any, text string) (string, error) {
	if audience == "agent" {
		return marshalPretty(okEnv{OK: true, Data: data})
	}
	return humanText(text), nil
}

func humanText(text string) string {
	if text == "" || strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}

func agentRaw(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		raw = json.RawMessage("null")
	}
	pretty, err := indentRaw(raw)
	if err != nil {
		return "", err
	}
	lines := strings.Split(pretty, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = "  " + lines[i]
	}
	return "{\n  \"ok\": true,\n  \"data\": " + strings.Join(lines, "\n") + "\n}\n", nil
}

func errBody(message string) string {
	return "{\n  \"ok\": false,\n  \"error\": {\n    \"type\": " + strconv.Quote(errType(message)) + ",\n    \"message\": " + strconv.Quote(message) + "\n  }\n}\n"
}

func errType(message string) string {
	if message == "unauthorized" || strings.Contains(message, "未登录") {
		return "auth"
	}
	return "error"
}

func renderTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	widths := make([]int, len(headers))
	all := make([][]string, 0, len(rows)+1)
	all = append(all, headers)
	all = append(all, rows...)
	for _, row := range all {
		for col := range headers {
			cell := ""
			if col < len(row) {
				cell = row[col]
			}
			n := len([]rune(cell))
			if n > widths[col] {
				widths[col] = n
			}
		}
	}
	var b strings.Builder
	for _, row := range all {
		for col := range headers {
			if col > 0 {
				b.WriteString("  ")
			}
			cell := ""
			if col < len(row) {
				cell = row[col]
			}
			b.WriteString(padEnd(cell, widths[col]))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func jsonText(raw json.RawMessage) (string, error) {
	text, err := indentRaw(raw)
	if err != nil {
		return "", err
	}
	return text + "\n", nil
}

func indentRaw(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("invalid json")
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return "", fmt.Errorf("invalid json")
	}
	return buf.String(), nil
}

func marshalPretty(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// eventRows 从目录树里抽出事件名. 目录接口把事件嵌在多层 catalog 里.
func eventRows(raw json.RawMessage) ([][]string, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var rows [][]string
	walk(v, func(obj map[string]any) {
		name := asText(obj["eventName"])
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		rows = append(rows, []string{name, asText(obj["eventDesc"])})
	})
	return rows, nil
}

// propRows 兼容两套字段. quot 和 columnName 都表示属性名.
func propRows(raw json.RawMessage) ([][]string, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	var rows [][]string
	walk(v, func(obj map[string]any) {
		name := asText(obj["quot"])
		if name == "" {
			name = asText(obj["columnName"])
		}
		if name == "" {
			return
		}
		desc := asText(obj["quotDesc"])
		if desc == "" {
			desc = asText(obj["columnDesc"])
		}
		rows = append(rows, []string{name, desc, asText(obj["tableType"])})
	})
	return rows, nil
}

func projectRows(raw json.RawMessage) ([][]string, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	var rows [][]string
	for _, item := range firstArray(v) {
		obj, ok := asMap(item)
		if !ok {
			continue
		}
		id := asText(obj["projectId"])
		if id == "" {
			id = asText(obj["id"])
		}
		name := asText(obj["projectName"])
		if name == "" {
			name = asText(obj["name"])
		}
		if id == "" && name == "" {
			continue
		}
		rows = append(rows, []string{id, name})
	}
	return rows, nil
}

func reportRows(raw json.RawMessage) ([][]string, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	var rows [][]string
	for _, item := range firstArray(v) {
		obj, ok := asMap(item)
		if !ok {
			continue
		}
		id := asText(obj["reportId"])
		if id == "" {
			id = asText(obj["id"])
		}
		name := asText(obj["reportName"])
		if name == "" {
			name = asText(obj["name"])
		}
		if id == "" && name == "" {
			continue
		}
		rows = append(rows, []string{id, name, asText(obj["reportModel"])})
	}
	return rows, nil
}

// queryText 优先打页面的 x/y 表. 没有这层结构时保留原始 JSON.
func queryText(raw json.RawMessage) (string, error) {
	table, err := resultTable(raw)
	if err != nil {
		return "", err
	}
	if table != "" {
		return table, nil
	}
	return jsonText(raw)
}

func resultTable(raw json.RawMessage) (string, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return "", err
	}
	root := v
	if obj, ok := asMap(v); ok {
		if data, ok := asMap(obj["data"]); ok {
			root = data
		}
	}
	obj, ok := asMap(root)
	if !ok {
		return "", nil
	}
	xs, okX := obj["x"].([]any)
	ys, okY := obj["y"].([]any)
	if !okX || !okY {
		return "", nil
	}
	headers := make([]string, 0, len(xs)+1)
	headers = append(headers, "NAME")
	for _, item := range xs {
		headers = append(headers, asText(item))
	}
	var rows [][]string
	for _, item := range ys {
		rowObj, ok := asMap(item)
		if !ok {
			continue
		}
		nums, ok := rowObj["data"].([]any)
		if !ok {
			continue
		}
		name := asText(rowObj["eventNameDisplay"])
		if name == "" {
			name = asText(rowObj["eventName"])
		}
		if name == "" {
			name = asText(rowObj["name"])
		}
		line := make([]string, 0, len(nums)+1)
		line = append(line, name)
		for _, cell := range nums {
			line = append(line, asText(cell))
		}
		rows = append(rows, line)
	}
	if len(rows) == 0 {
		return "", nil
	}
	return renderTable(headers, rows), nil
}

func rowsOrJSON(headers []string, rows [][]string, raw json.RawMessage) (string, error) {
	if len(rows) == 0 {
		return jsonText(raw)
	}
	return renderTable(headers, rows), nil
}

func firstArray(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	obj, ok := asMap(v)
	if !ok {
		return nil
	}
	for _, key := range []string{"list", "data", "reports", "projects"} {
		if arr, ok := obj[key].([]any); ok {
			return arr
		}
	}
	return nil
}

func walk(v any, visit func(map[string]any)) {
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			walk(item, visit)
		}
	case map[string]any:
		visit(t)
		for _, item := range t {
			walk(item, visit)
		}
	}
}
