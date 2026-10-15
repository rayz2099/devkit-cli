package jenkins

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func renderJSON(data any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderText(text string) string {
	if text == "" {
		return ""
	}
	if strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}

func renderTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	width := make([]int, len(headers))
	all := make([][]string, 0, len(rows)+1)
	all = append(all, headers)
	all = append(all, rows...)
	for _, row := range all {
		for col := range headers {
			cell := ""
			if col < len(row) {
				cell = row[col]
			}
			n := utf8.RuneCountInString(cell)
			if n > width[col] {
				width[col] = n
			}
		}
	}
	var b strings.Builder
	for i, row := range all {
		for col := range headers {
			if col > 0 {
				b.WriteString("  ")
			}
			cell := ""
			if col < len(row) {
				cell = row[col]
			}
			b.WriteString(pad(cell, width[col]))
		}
		if i != len(all)-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	return b.String()
}

func jobLsText(jobs []JobRef) string {
	if len(jobs) == 0 {
		return "No jobs found\n"
	}
	rows := make([][]string, 0, len(jobs))
	for _, job := range jobs {
		rows = append(rows, []string{job.Name, classTail(job.ClassName)})
	}
	return renderTable([]string{"NAME", "CLASS"}, rows)
}

func jobViewText(job JobView) string {
	lines := []string{
		"name\t" + job.Name,
		"url\t" + job.URL,
		"buildable\t" + strconv.FormatBool(job.Buildable),
		"lastBuild\t" + fmtNum(job.Last),
		"lastSuccessfulBuild\t" + fmtNum(job.LastOK),
		"lastFailedBuild\t" + fmtNum(job.LastFailed),
		"",
	}
	return strings.Join(lines, "\n")
}

func runLsText(items []RunItem) string {
	if len(items) == 0 {
		return "No runs found\n"
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			strconv.FormatInt(item.Number, 10),
			item.Result,
			fmtTime(item.Timestamp),
			fmtDur(item.DurationMs),
		})
	}
	return renderTable([]string{"NUMBER", "RESULT", "TIMESTAMP", "DURATION"}, rows)
}

func queueLsText(items []QueueItem) string {
	if len(items) == 0 {
		return "Queue is empty\n"
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{strconv.FormatInt(item.ID, 10), item.Name, item.Why})
	}
	return renderTable([]string{"ID", "JOB", "WHY"}, rows)
}

func pickBody(aud, text string, data any) (string, error) {
	if aud == "agent" {
		return renderJSON(data)
	}
	return text, nil
}

func classTail(className string) string {
	if i := strings.LastIndex(className, "."); i >= 0 {
		return className[i+1:]
	}
	return className
}

func fmtNum(v *int64) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatInt(*v, 10)
}

func fmtTime(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	return time.UnixMilli(ts).UTC().Format("2006-01-02T15:04:05.000Z")
}

func fmtDur(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	sec := ms / 1000
	min := sec / 60
	rem := sec % 60
	if min > 0 {
		return strconv.FormatInt(min, 10) + "m" + strconv.FormatInt(rem, 10) + "s"
	}
	return strconv.FormatInt(rem, 10) + "s"
}

func pad(s string, n int) string {
	w := utf8.RuneCountInString(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func tailText(text string, n int) string {
	lines := strings.Split(text, "\n")
	if n > len(lines) {
		n = len(lines)
	}
	sliced := strings.Join(lines[len(lines)-n:], "\n")
	if strings.HasSuffix(text, "\n") {
		return sliced + "\n"
	}
	return sliced
}
