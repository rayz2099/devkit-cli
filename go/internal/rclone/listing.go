package rclone

import (
	"encoding/json"
	"sort"
	"strings"
)

// FileEnt 是清单里的一个文件。一次拉全量，才能把同名和 .bin 对齐。
type FileEnt struct {
	Path string
	Size int64
}

// InvRow 是本地和远端对上的一行。extra 不让 check 失败。
type InvRow struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Local  *int64 `json:"local,omitempty"`
	Remote *int64 `json:"remote,omitempty"`
	Via    string `json:"via,omitempty"`
}

// Counts 是全量计数。截断展示时也不能把 missing 藏掉。
type Counts struct {
	OK      int `json:"ok"`
	OKBin   int `json:"ok-bin"`
	Missing int `json:"missing"`
	Differ  int `json:"differ"`
	Extra   int `json:"extra"`
}

// InvReport 是一次对账。ls 可以只展示前几行，计数仍是全量。
type InvReport struct {
	Task   string   `json:"task"`
	Rel    string   `json:"rel"`
	Rows   []InvRow `json:"rows"`
	Counts Counts   `json:"counts"`
	Limit  *int     `json:"limit,omitempty"`
	Total  *int     `json:"total,omitempty"`
}

func parseLsJson(stdout string) ([]FileEnt, error) {
	text := strings.TrimSpace(stdout)
	if text == "" {
		return []FileEnt{}, nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fail(1, "rclone lsjson is not JSON")
	}
	items, ok := parsed.([]any)
	if !ok {
		return nil, fail(1, "rclone lsjson must be an array")
	}
	out := []FileEnt{}
	for _, item := range items {
		ent, keep, err := readEnt(item)
		if err != nil {
			return nil, err
		}
		if keep {
			out = append(out, ent)
		}
	}
	return out, nil
}

func readEnt(value any) (FileEnt, bool, error) {
	raw, ok := value.(map[string]any)
	if !ok || value == nil {
		return FileEnt{}, false, fail(1, "rclone lsjson entry must be an object")
	}
	if raw["IsDir"] == true {
		return FileEnt{}, false, nil
	}
	path, ok := raw["Path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return FileEnt{}, false, fail(1, "rclone lsjson entry.Path must be a non-empty string")
	}
	size, ok := raw["Size"].(float64)
	if !ok {
		return FileEnt{}, false, fail(1, "rclone lsjson entry.Size missing: %s", path)
	}
	n := int64(size)
	return FileEnt{Path: strings.ReplaceAll(path, "\\", "/"), Size: n}, true, nil
}

// ComputeInv 用同名优先。.bin 只在同名不存在时当成同一份对象。
func ComputeInv(task, rel string, local, remote []FileEnt, allowBin bool) (*InvReport, error) {
	remoteBy := indexFiles(remote)
	localBy := indexFiles(local)
	used := map[string]bool{}
	rows := []InvRow{}
	keys := sortedKeys(localBy)
	for _, path := range keys {
		localSize := localBy[path]
		if named, ok := remoteBy[path]; ok {
			used[path] = true
			rows = append(rows, sizeRow(path, localSize, named, "name"))
			continue
		}
		binPath := path + ".bin"
		legacy, hasBin := remoteBy[binPath]
		if allowBin && hasBin {
			used[binPath] = true
			rows = append(rows, sizeRow(path, localSize, legacy, "bin"))
			continue
		}
		n := localSize
		rows = append(rows, InvRow{Path: path, State: "missing", Local: &n})
	}
	var extras []string
	for path := range remoteBy {
		if !used[path] {
			extras = append(extras, path)
		}
	}
	sort.Strings(extras)
	for _, path := range extras {
		remoteSize := remoteBy[path]
		via := "name"
		if allowBin && strings.HasSuffix(path, ".bin") {
			via = "bin"
		}
		n := remoteSize
		rows = append(rows, InvRow{Path: path, State: "extra", Remote: &n, Via: via})
	}
	return &InvReport{Task: task, Rel: rel, Rows: rows, Counts: countStates(rows)}, nil
}

func checkFailed(report *InvReport) bool {
	return report.Counts.Missing > 0 || report.Counts.Differ > 0
}

func clipInv(report *InvReport, limit int) *InvReport {
	rows := report.Rows
	if limit < len(rows) {
		rows = rows[:limit]
	}
	total := len(report.Rows)
	lim := limit
	return &InvReport{
		Task: report.Task, Rel: report.Rel, Rows: rows, Counts: report.Counts,
		Limit: &lim, Total: &total,
	}
}

func buildListArgv(plan *Plan, cfgFile, root string, probe bool) []string {
	args := []string{"--config", cfgFile, "lsjson", root, "--files-only"}
	if !probe {
		args = append(args, "--recursive")
		for _, pattern := range plan.Excludes {
			args = append(args, "--exclude", pattern)
		}
		if plan.Target.SkipLinks {
			args = append(args, "--skip-links")
		}
		args = append(args,
			"--retries", numStr(plan.Target.Retries),
			"--retries-sleep", plan.Target.RetriesSleep,
			"--low-level-retries", numStr(plan.Target.LowLevelRetries),
		)
	} else {
		args = append(args, "--retries", "1", "--retries-sleep", "1s", "--low-level-retries", "1")
	}
	args = append(args,
		"--timeout", plan.Timeout,
		"--contimeout", plan.Target.Contimeout,
		"--tpslimit", numStr(plan.Tpslimit),
	)
	return args
}

func listFiles(bin string, args []string, spawn SpawnFn) ([]FileEnt, error) {
	return lsJSON(bin, args, spawn, "remote")
}

func loadInv(bin string, localArgs, remoteArgs []string, spawn SpawnFn, task, rel string, allowBin bool) (*InvReport, error) {
	local, err := lsJSON(bin, localArgs, spawn, "local")
	if err != nil {
		return nil, err
	}
	remote, err := lsJSON(bin, remoteArgs, spawn, "remote")
	if err != nil {
		return nil, err
	}
	return ComputeInv(task, rel, local, remote, allowBin)
}

func lsJSON(bin string, args []string, spawn SpawnFn, side string) ([]FileEnt, error) {
	ret, err := spawn(bin, args, noopSink())
	if err != nil {
		return nil, err
	}
	if ret.Code != 0 {
		if isMissingList(ret.Stderr) {
			return []FileEnt{}, nil
		}
		text := strings.TrimSpace(ret.Stderr)
		if text == "" {
			text = "rclone lsjson " + side + " exited " + numStr(float64(ret.Code))
		}
		return nil, &Err{Code: ret.Code, Msg: text}
	}
	return parseLsJson(ret.Stdout)
}

func indexFiles(items []FileEnt) map[string]int64 {
	out := map[string]int64{}
	for _, item := range items {
		out[item.Path] = item.Size
	}
	return out
}

func sortedKeys(items map[string]int64) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sizeRow(path string, local, remote int64, via string) InvRow {
	l := local
	r := remote
	if local == remote {
		state := "ok"
		if via == "bin" {
			state = "ok-bin"
		}
		return InvRow{Path: path, State: state, Local: &l, Remote: &r, Via: via}
	}
	return InvRow{Path: path, State: "differ", Local: &l, Remote: &r, Via: via}
}

func countStates(rows []InvRow) Counts {
	var counts Counts
	for _, row := range rows {
		switch row.State {
		case "ok":
			counts.OK++
		case "ok-bin":
			counts.OKBin++
		case "missing":
			counts.Missing++
		case "differ":
			counts.Differ++
		case "extra":
			counts.Extra++
		}
	}
	return counts
}

func isMissingList(stderr string) bool {
	text := strings.ToLower(stderr)
	return strings.Contains(text, "not found") || strings.Contains(text, "directory not found") || strings.Contains(text, "object not found")
}
