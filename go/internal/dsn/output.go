package dsn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// QueryOut 是各 kind 共用的表。列顺序决定人读的表头和 JSON 键顺序。
type QueryOut struct {
	Columns []string
	Rows    []map[string]any
}

// capRows 在结果读完后截断。禁止把 LIMIT 写进用户语句。
func capRows(rows []map[string]any, limit int) ([]map[string]any, bool) {
	if limit == 0 || len(rows) <= limit {
		return rows, false
	}
	return rows[:limit], true
}

func agentLimit(limit *int) int {
	if limit == nil {
		return 1000
	}
	return *limit
}

// Render 按受众出文本。agent 永远是一个 JSON 对象，忽略 --output。
func Render(audience, format string, data QueryOut, truncated, pretty bool) string {
	if audience == "agent" {
		body := agentJSON(data, truncated)
		return body + "\n"
	}
	if format == "json" {
		if pretty {
			return prettyArray(data) + "\n"
		}
		if len(data.Rows) == 0 {
			return ""
		}
		var b strings.Builder
		for _, row := range data.Rows {
			b.Write(rowJSON(data.Columns, row))
			b.WriteByte('\n')
		}
		return b.String()
	}
	if format == "csv" {
		return renderCSV(data)
	}
	if format == "plain" {
		return renderPlain(data)
	}
	return renderTable(data)
}

func agentJSON(data QueryOut, truncated bool) string {
	raws := make([]json.RawMessage, len(data.Rows))
	for i, row := range data.Rows {
		raws[i] = rowJSON(data.Columns, row)
	}
	doc := struct {
		Rows      []json.RawMessage `json:"rows"`
		Truncated bool              `json:"truncated"`
	}{Rows: raws, Truncated: truncated}
	buf, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return ""
	}
	return string(buf)
}

func prettyArray(data QueryOut) string {
	if len(data.Rows) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[\n")
	for i, row := range data.Rows {
		b.WriteString(prettyObj(data.Columns, row, "  "))
		if i+1 != len(data.Rows) {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteByte(']')
	return b.String()
}

func prettyObj(cols []string, row map[string]any, pad string) string {
	keys := rowKeys(cols, row)
	var b strings.Builder
	b.WriteString(pad)
	b.WriteString("{\n")
	for i, key := range keys {
		b.WriteString(pad)
		b.WriteString("  ")
		b.Write(mustMarshal(key))
		b.WriteString(": ")
		b.Write(mustMarshal(normJSON(row[key])))
		if i+1 != len(keys) {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(pad)
	b.WriteByte('}')
	return b.String()
}

func rowJSON(cols []string, row map[string]any) json.RawMessage {
	keys := rowKeys(cols, row)
	var b bytes.Buffer
	b.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(mustMarshal(key))
		b.WriteByte(':')
		b.Write(mustMarshal(normJSON(row[key])))
	}
	b.WriteByte('}')
	return b.Bytes()
}

func rowKeys(cols []string, row map[string]any) []string {
	seen := map[string]struct{}{}
	var keys []string
	for _, col := range cols {
		if _, ok := row[col]; !ok {
			continue
		}
		if _, dup := seen[col]; dup {
			continue
		}
		seen[col] = struct{}{}
		keys = append(keys, col)
	}
	extras := make([]string, 0)
	for key := range row {
		if _, ok := seen[key]; ok {
			continue
		}
		extras = append(extras, key)
	}
	sortStrings(extras)
	return append(keys, extras...)
}

func sortStrings(in []string) {
	for i := 1; i < len(in); i++ {
		j := i
		for j > 0 && in[j] < in[j-1] {
			in[j], in[j-1] = in[j-1], in[j]
			j--
		}
	}
}

func mustMarshal(v any) []byte {
	buf, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return buf
}

func normJSON(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case time.Time:
		return t.UTC().Format("2006-01-02T15:04:05.000Z")
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t.String()
	default:
		return v
	}
}

func renderTable(data QueryOut) string {
	if len(data.Rows) == 0 {
		return ""
	}
	cols := columnsOf(data)
	lines := make([][]string, 0, len(data.Rows)+1)
	lines = append(lines, cols)
	for _, row := range data.Rows {
		cells := make([]string, len(cols))
		for i, col := range cols {
			cells[i] = cellText(row[col])
		}
		lines = append(lines, cells)
	}
	widths := make([]int, len(cols))
	for _, line := range lines {
		for i, cell := range line {
			n := utf8.RuneCountInString(cell)
			if n > widths[i] {
				widths[i] = n
			}
		}
	}
	var b strings.Builder
	for _, line := range lines {
		for col, cell := range line {
			if col > 0 {
				b.WriteString("  ")
			}
			b.WriteString(padEnd(cell, widths[col]))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func padEnd(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func renderCSV(data QueryOut) string {
	cols := columnsOf(data)
	var b strings.Builder
	for i, col := range cols {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(csvCell(col))
	}
	b.WriteByte('\n')
	for _, row := range data.Rows {
		for i, col := range cols {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(csvCell(cellText(row[col])))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func renderPlain(data QueryOut) string {
	if len(data.Rows) == 0 {
		return ""
	}
	cols := columnsOf(data)
	var b strings.Builder
	for _, row := range data.Rows {
		for c, col := range cols {
			if c > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(cellText(row[col]))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func columnsOf(data QueryOut) []string {
	if len(data.Columns) > 0 {
		return data.Columns
	}
	seen := map[string]struct{}{}
	var cols []string
	for _, row := range data.Rows {
		for _, key := range rowKeys(nil, row) {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			cols = append(cols, key)
		}
	}
	return cols
}

func csvCell(value string) string {
	if strings.ContainsAny(value, "\",\n") {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return value
}

func cellText(value any) string {
	if value == nil {
		return ""
	}
	switch t := value.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", t)
	case int32:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case uint32:
		return fmt.Sprintf("%d", t)
	case uint64:
		return fmt.Sprintf("%d", t)
	case float32:
		return fmt.Sprint(t)
	case float64:
		return fmt.Sprint(t)
	case json.Number:
		return t.String()
	case time.Time:
		return t.UTC().Format("2006-01-02T15:04:05.000Z")
	case []byte:
		return string(t)
	default:
		buf, err := json.Marshal(normJSON(t))
		if err != nil {
			return ""
		}
		return string(buf)
	}
}

func renderText(text string) string {
	if text == "" || strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}
