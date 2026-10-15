package rclone

import (
	"bytes"
	"encoding/json"
	"strings"
)

type lsSource struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type lsTarget struct {
	Name             string `json:"name"`
	URL              string `json:"url"`
	Vendor           string `json:"vendor"`
	Root             string `json:"root"`
	Crypt            int    `json:"crypt"`
	CryptDefault     string `json:"cryptDefault,omitempty"`
	EncryptFilenames bool   `json:"encryptFilenames"`
	SkipLinks        bool   `json:"skipLinks"`
	SkipHidden       bool   `json:"skipHidden"`
	SizeOnly         bool   `json:"sizeOnly"`
}

type lsTask struct {
	Name             string   `json:"name"`
	Source           string   `json:"source"`
	Target           string   `json:"target"`
	Excludes         []string `json:"excludes"`
	Transfers        *float64 `json:"transfers,omitempty"`
	Tpslimit         *float64 `json:"tpslimit,omitempty"`
	Timeout          *string  `json:"timeout,omitempty"`
	EncryptFilenames *bool    `json:"encryptFilenames,omitempty"`
	SkipHidden       *bool    `json:"skipHidden,omitempty"`
}

type lsView struct {
	Sources []lsSource `json:"sources"`
	Targets []lsTarget `json:"targets"`
	Tasks   []lsTask   `json:"tasks"`
}

type invJSON struct {
	Verb   string   `json:"verb"`
	Task   string   `json:"task"`
	Rel    string   `json:"rel"`
	Rows   []InvRow `json:"rows"`
	Counts Counts   `json:"counts"`
	Limit  *int     `json:"limit,omitempty"`
	Total  *int     `json:"total,omitempty"`
}

var invOrder = []string{"ok", "ok-bin", "missing", "differ", "extra"}
var invProblems = map[string]bool{"missing": true, "differ": true, "extra": true}

// RenderLs 不联网。口令不能出现在输出里，否则 600 权限没有意义。
func RenderLs(cfg *FileCfg, audience string) (string, error) {
	view := lsView{Sources: []lsSource{}, Targets: []lsTarget{}, Tasks: []lsTask{}}
	for _, source := range cfg.Sources {
		view.Sources = append(view.Sources, lsSource{Name: source.Name, Path: source.Path})
	}
	for _, target := range cfg.Targets {
		item := lsTarget{
			Name: target.Name, URL: target.URL, Vendor: target.Vendor, Root: target.Root,
			EncryptFilenames: target.EncryptFilenames, SkipLinks: target.SkipLinks,
			SkipHidden: target.SkipHidden, SizeOnly: target.SizeOnly,
		}
		if target.HasCrypt {
			item.Crypt = len(target.Crypt)
			for _, pair := range target.Crypt {
				if pair.Default {
					item.CryptDefault = pair.Name
				}
			}
		}
		view.Targets = append(view.Targets, item)
	}
	for _, task := range cfg.Tasks {
		excludes := task.Excludes
		if excludes == nil {
			excludes = []string{}
		}
		view.Tasks = append(view.Tasks, lsTask{
			Name: task.Name, Source: task.Source, Target: task.Target, Excludes: excludes,
			Transfers: task.Transfers, Tpslimit: task.Tpslimit, Timeout: task.Timeout,
			EncryptFilenames: task.EncryptFilenames, SkipHidden: task.SkipHidden,
		})
	}
	if audience == "agent" {
		return toJSON(view)
	}
	lines := []string{"sources:"}
	for _, source := range view.Sources {
		lines = append(lines, "  "+source.Name+"  "+source.Path)
	}
	lines = append(lines, "targets:")
	for _, target := range view.Targets {
		crypt := "plain"
		if target.Crypt != 0 {
			crypt = "crypt=" + numStr(float64(target.Crypt))
		}
		names := "names=off"
		if target.EncryptFilenames {
			names = "names=on"
		}
		lines = append(lines, "  "+target.Name+"  "+target.Vendor+"  "+target.URL+"  root="+target.Root+"  "+crypt+"  "+names)
	}
	lines = append(lines, "tasks:")
	for _, task := range view.Tasks {
		lines = append(lines, "  "+task.Name+"  "+task.Source+" -> "+task.Target)
	}
	return joinLines(lines), nil
}

func renderRuns(recs []RunRec, audience string) (string, error) {
	if audience == "agent" {
		if recs == nil {
			recs = []RunRec{}
		}
		return toJSON(recs)
	}
	if len(recs) == 0 {
		return "no runs", nil
	}
	lines := make([]string, len(recs))
	for i, rec := range recs {
		lines[i] = renderRunLine(rec)
	}
	return joinLines(lines), nil
}

func renderInv(report *InvReport, audience, verb string) (string, error) {
	if report.Rows == nil {
		report.Rows = []InvRow{}
	}
	if audience == "agent" {
		body := invJSON{
			Verb: verb, Task: report.Task, Rel: report.Rel, Rows: report.Rows,
			Counts: report.Counts, Limit: report.Limit, Total: report.Total,
		}
		return toJSON(body)
	}
	head := verb + " " + report.Task
	if report.Rel != "" {
		head += " " + report.Rel
	}
	lines := []string{head}
	if report.Limit != nil && report.Total != nil {
		lines = append(lines, "  showing "+numStr(float64(len(report.Rows)))+"/"+numStr(float64(*report.Total))+"  limit="+numStr(float64(*report.Limit)))
	}
	for _, state := range invOrder {
		lines = append(lines, "  "+pad(state, 8)+" "+numStr(float64(countOf(report.Counts, state))))
	}
	var show []InvRow
	if verb == "ls" {
		show = report.Rows
	} else {
		for _, row := range report.Rows {
			if invProblems[row.State] {
				show = append(show, row)
			}
		}
	}
	if len(show) == 0 {
		return joinLines(lines), nil
	}
	lines = append(lines, "")
	for _, row := range show {
		lines = append(lines, "  "+formatRow(row))
	}
	return joinLines(lines), nil
}

func renderXfer(rec RunRec, audience string) (string, error) {
	if audience == "agent" {
		if rec.Args == nil {
			rec.Args = []string{}
		}
		return toJSON(rec)
	}
	if rec.Code == 0 {
		dry := ""
		if rec.DryRun {
			dry = " dry-run"
		}
		return rec.Action + " " + rec.Task + " ok" + dry, nil
	}
	reason := rec.Error
	if reason == "" {
		reason = "code " + numStr(float64(rec.Code))
	}
	return rec.Action + " " + rec.Task + " failed (" + numStr(float64(rec.Code)) + "): " + firstLine(reason), nil
}

func renderRunLine(rec RunRec) string {
	dry := ""
	if rec.DryRun {
		dry = " dry-run"
	}
	head := rec.FinishedAt + " " + rec.Action + " " + rec.Task + " code=" + numStr(float64(rec.Code)) + dry + " " + rec.Remote
	if rec.Error == "" {
		return head
	}
	return head + " error=" + firstLine(rec.Error)
}

func formatRow(row InvRow) string {
	bits := []string{pad(row.State, 8), row.Path}
	if row.Local != nil {
		bits = append(bits, "local="+numStr(float64(*row.Local)))
	}
	if row.Remote != nil {
		bits = append(bits, "remote="+numStr(float64(*row.Remote)))
	}
	if row.Via != "" {
		bits = append(bits, "via="+row.Via)
	}
	return strings.Join(bits, "  ")
}

func countOf(counts Counts, state string) int {
	switch state {
	case "ok":
		return counts.OK
	case "ok-bin":
		return counts.OKBin
	case "missing":
		return counts.Missing
	case "differ":
		return counts.Differ
	case "extra":
		return counts.Extra
	default:
		return 0
	}
}

func pad(text string, width int) string {
	if len(text) >= width {
		return text
	}
	return text + strings.Repeat(" ", width-len(text))
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	runes := []rune(line)
	if len(runes) <= 200 {
		return line
	}
	return string(runes[:200])
}

func toJSON(value any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return "", fail(1, "json: %s", err.Error())
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func toJSONLine(value any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return "", fail(1, "json: %s", err.Error())
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}
