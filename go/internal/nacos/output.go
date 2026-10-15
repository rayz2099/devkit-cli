package nacos

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf16"
)

func render(format, text string, data any) (string, error) {
	if format == "json" {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
	if text == "" {
		return "", nil
	}
	return text + "\n", nil
}

// renderTable 把 DATA_ID 和 GROUP 按列对齐。宽度按 JS 的 UTF-16 长度算，中文才不会错位。
func renderTable(summary string, headers []string, rows [][]string) string {
	var lines []string
	if strings.TrimSpace(summary) != "" {
		lines = append(lines, summary)
	}
	table := make([][]string, 0, 1+len(rows))
	table = append(table, headers)
	table = append(table, rows...)
	widths := make([]int, len(headers))
	for col := range headers {
		for _, row := range table {
			cell := cellAt(row, col)
			if n := jsWidth(cell); n > widths[col] {
				widths[col] = n
			}
		}
	}
	for _, row := range table {
		parts := make([]string, len(headers))
		for col := range headers {
			parts[col] = padEnd(cellAt(row, col), widths[col])
		}
		lines = append(lines, strings.Join(parts, "  "))
	}
	return strings.Join(lines, "\n")
}

func cellAt(row []string, col int) string {
	if col < 0 || col >= len(row) {
		return ""
	}
	return row[col]
}

func jsWidth(text string) int {
	return len(utf16.Encode([]rune(text)))
}

func padEnd(text string, width int) string {
	n := jsWidth(text)
	if n >= width {
		return text
	}
	return text + strings.Repeat(" ", width-n)
}
