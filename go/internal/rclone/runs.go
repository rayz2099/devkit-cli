package rclone

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func appendRun(rec RunRec) error {
	path, err := runsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fail(1, "runs dir: %s", err.Error())
	}
	if rec.Args == nil {
		rec.Args = []string{}
	}
	line, err := toJSONLine(rec)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fail(1, "runs: %s", err.Error())
	}
	defer file.Close()
	if _, err := file.WriteString(line + "\n"); err != nil {
		return fail(1, "runs: %s", err.Error())
	}
	return nil
}

// ReadRuns 把最新的一次放在前面。文件按时间追加，不在配置里再存一份索引。
func ReadRuns() ([]RunRec, error) {
	path, err := runsPath()
	if err != nil {
		return nil, err
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []RunRec{}, nil
		}
		return nil, fail(1, "runs: %s", err.Error())
	}
	lines := strings.Split(string(buf), "\n")
	var recs []RunRec
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		rec, err := parseRunLine(line, i+1)
		if err != nil {
			return nil, err
		}
		recs = append(recs, rec)
	}
	reverse(recs)
	if recs == nil {
		recs = []RunRec{}
	}
	return recs, nil
}

func reverse(recs []RunRec) {
	for i, j := 0, len(recs)-1; i < j; i, j = i+1, j-1 {
		recs[i], recs[j] = recs[j], recs[i]
	}
}

func parseRunLine(line string, number int) (RunRec, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil || raw == nil {
		return RunRec{}, fail(1, "runs.jsonl line %d is invalid", number)
	}
	action, _ := raw["action"].(string)
	// 和 TypeScript 一样只认这三种。放宽成 get/download 会让坏记录被当成成功历史。
	if action != "push" && action != "pull" && action != "check" {
		return RunRec{}, fail(1, "runs.jsonl line %d is invalid", number)
	}
	id, idOK := raw["id"].(string)
	task, taskOK := raw["task"].(string)
	code, codeOK := raw["code"].(float64)
	dry, dryOK := raw["dryRun"].(bool)
	if !idOK || !taskOK || !codeOK || !dryOK {
		return RunRec{}, fail(1, "runs.jsonl line %d is invalid", number)
	}
	started, err := needRecStr(raw["startedAt"], number)
	if err != nil {
		return RunRec{}, err
	}
	finished, err := needRecStr(raw["finishedAt"], number)
	if err != nil {
		return RunRec{}, err
	}
	local, err := needRecStr(raw["local"], number)
	if err != nil {
		return RunRec{}, err
	}
	remote, err := needRecStr(raw["remote"], number)
	if err != nil {
		return RunRec{}, err
	}
	args, err := readArgs(raw["args"], number)
	if err != nil {
		return RunRec{}, err
	}
	rec := RunRec{
		ID: id, Task: task, Action: action, StartedAt: started, FinishedAt: finished,
		Code: int(code), Local: local, Remote: remote, DryRun: dry, Args: args,
	}
	if n, ok, err := optRecNum(raw, "bytes", number); err != nil {
		return RunRec{}, err
	} else if ok {
		rec.Bytes = &n
	}
	if n, ok, err := optRecNum(raw, "checks", number); err != nil {
		return RunRec{}, err
	} else if ok {
		rec.Checks = &n
	}
	if n, ok, err := optRecNum(raw, "transfers", number); err != nil {
		return RunRec{}, err
	} else if ok {
		rec.Transfers = &n
	}
	if text, ok := raw["error"].(string); ok {
		rec.Error = text
	}
	return rec, nil
}

func needRecStr(value any, number int) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", fail(1, "runs.jsonl line %d is invalid", number)
	}
	return text, nil
}

func readArgs(value any, number int) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fail(1, "runs.jsonl line %d is invalid", number)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fail(1, "runs.jsonl line %d is invalid", number)
		}
		out = append(out, text)
	}
	return out, nil
}

func optRecNum(raw map[string]any, key string, number int) (int64, bool, error) {
	value, ok := raw[key]
	if !ok || value == nil {
		return 0, false, nil
	}
	num, isNum := value.(float64)
	if !isNum {
		return 0, false, fail(1, "runs.jsonl line %d is invalid", number)
	}
	return int64(num), true, nil
}
