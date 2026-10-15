package codeup

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"unicode/utf8"
)

// RenderJSON 给 agent 稳定缩进。人读输出不走这条，避免两套排版。
func RenderJSON(data any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	widths := make([]int, len(headers))
	for col, head := range headers {
		widths[col] = utf8.RuneCountInString(head)
	}
	for _, row := range rows {
		for col := range headers {
			cell := ""
			if col < len(row) {
				cell = row[col]
			}
			n := utf8.RuneCountInString(cell)
			if n > widths[col] {
				widths[col] = n
			}
		}
	}
	var b strings.Builder
	writeRow(&b, headers, widths)
	for _, row := range rows {
		writeRow(&b, row, widths)
	}
	return b.String()
}

func writeRow(b *strings.Builder, row []string, widths []int) {
	for col, width := range widths {
		if col > 0 {
			b.WriteString("  ")
		}
		cell := ""
		if col < len(row) {
			cell = row[col]
		}
		b.WriteString(padEnd(cell, width))
	}
	b.WriteByte('\n')
}

func padEnd(text string, width int) string {
	n := utf8.RuneCountInString(text)
	if n >= width {
		return text
	}
	return text + strings.Repeat(" ", width-n)
}

type initJSON struct {
	Profile      string   `json:"profile"`
	Count        int      `json:"count"`
	UpdatedAt    string   `json:"updatedAt"`
	Repositories []string `json:"repositories"`
}

type reposJSON struct {
	Repositories []Repo `json:"repositories"`
	Count        int    `json:"count"`
}

type crListJSON struct {
	Repo           string          `json:"repo"`
	ChangeRequests []ChangeRequest `json:"changeRequests"`
	Count          int             `json:"count"`
}

type crFlatJSON struct {
	Repo        string `json:"repo"`
	LocalID     string `json:"localId"`
	Title       string `json:"title"`
	State       string `json:"state"`
	SrcBranch   string `json:"srcBranch"`
	TgtBranch   string `json:"tgtBranch"`
	Author      string `json:"author"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	WebURL      string `json:"webUrl"`
	CrURL       string `json:"crUrl"`
	Description string `json:"description"`
}

type crMergeJSON struct {
	Repo        string `json:"repo"`
	MergeType   string `json:"mergeType"`
	LocalID     string `json:"localId"`
	Title       string `json:"title"`
	State       string `json:"state"`
	SrcBranch   string `json:"srcBranch"`
	TgtBranch   string `json:"tgtBranch"`
	Author      string `json:"author"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	WebURL      string `json:"webUrl"`
	CrURL       string `json:"crUrl"`
	Description string `json:"description"`
}

type pushJSON struct {
	Remote string `json:"remote"`
	Branch string `json:"branch"`
	Pushed bool   `json:"pushed"`
}

type hooksJSON struct {
	Repo     string    `json:"repo"`
	Webhooks []Webhook `json:"webhooks"`
	Count    int       `json:"count"`
}

func pickBody(audience, text string, data any) (string, error) {
	if audience == audienceAgent {
		return RenderJSON(data)
	}
	return text, nil
}

func reposText(repos []Repo) string {
	if len(repos) == 0 {
		return "No repositories found\n"
	}
	rows := make([][]string, len(repos))
	for i, repo := range repos {
		rows[i] = []string{repo.ID, repo.Name, repo.PathNs, repo.DefBranch, repo.WebURL}
	}
	return renderTable([]string{"ID", "NAME", "PATH", "DEFAULT", "URL"}, rows)
}

func crListText(repo string, crs []ChangeRequest) string {
	if len(crs) == 0 {
		return "No change requests in " + repo + "\n"
	}
	rows := make([][]string, len(crs))
	for i, cr := range crs {
		rows[i] = []string{
			cr.LocalID,
			cr.State,
			clip(cr.Title, 50),
			cr.SrcBranch,
			cr.TgtBranch,
			cr.Author,
			cr.UpdatedAt,
		}
	}
	return renderTable([]string{"#", "STATE", "TITLE", "SOURCE", "TARGET", "AUTHOR", "UPDATED"}, rows)
}

func crViewText(repo string, cr ChangeRequest) string {
	lines := []string{
		"repo\t" + repo,
		"#\t" + cr.LocalID,
		"state\t" + cr.State,
		"title\t" + cr.Title,
		"source\t" + cr.SrcBranch,
		"target\t" + cr.TgtBranch,
		"author\t" + cr.Author,
		"created\t" + cr.CreatedAt,
		"updated\t" + cr.UpdatedAt,
		"url\t" + cr.CrURL,
	}
	if cr.Description != "" {
		lines = append(lines, "", cr.Description)
	}
	return strings.Join(lines, "\n") + "\n"
}

func webhookText(hooks []Webhook) string {
	if len(hooks) == 0 {
		return "No webhooks found\n"
	}
	rows := make([][]string, len(hooks))
	for i, hook := range hooks {
		rows[i] = []string{
			hook.ID,
			hook.URL,
			yn(hook.PushEvents),
			yn(hook.MergeRequestsEvents),
			yn(hook.TagPushEvents),
			yn(hook.NoteEvents),
			hook.SecretToken,
			hook.UpdatedAt,
		}
	}
	return renderTable([]string{"ID", "URL", "PUSH", "MR", "TAG", "NOTE", "SECRET", "UPDATED"}, rows)
}

// MaskHooks 默认打码 secret。钩子 URL 的 query token 也要打，否则只藏字段还会漏。
func MaskHooks(hooks []Webhook, showSecrets bool) []Webhook {
	if showSecrets {
		return hooks
	}
	out := make([]Webhook, len(hooks))
	for i, hook := range hooks {
		out[i] = hook
		out[i].URL = maskHookURL(hook.URL)
		if hook.SecretToken != "" {
			out[i].SecretToken = "***"
		}
	}
	return out
}

func maskHookURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return raw
	}
	if !queryHas(parsed.RawQuery, "token") {
		return raw
	}
	parsed.RawQuery = replaceToken(parsed.RawQuery)
	return parsed.String()
}

func queryHas(raw, key string) bool {
	if raw == "" {
		return false
	}
	for _, part := range strings.Split(raw, "&") {
		if queryName(part) == key {
			return true
		}
	}
	return false
}

func replaceToken(raw string) string {
	parts := strings.Split(raw, "&")
	out := make([]string, 0, len(parts))
	seen := false
	for _, part := range parts {
		if queryName(part) != "token" {
			out = append(out, part)
			continue
		}
		if seen {
			continue
		}
		seen = true
		name := part
		if i := strings.IndexByte(part, '='); i >= 0 {
			name = part[:i]
		}
		out = append(out, name+"=***")
	}
	return strings.Join(out, "&")
}

func queryName(part string) string {
	name := part
	if i := strings.IndexByte(part, '='); i >= 0 {
		name = part[:i]
	}
	decoded, err := url.QueryUnescape(name)
	if err != nil {
		return name
	}
	return decoded
}

func yn(value bool) string {
	if value {
		return "Y"
	}
	return "N"
}

func clip(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}
