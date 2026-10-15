package jenkins

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// cmd 是解析后的一条命令。字段按需填，避免每个动词一套长参数。
type cmd struct {
	kind     string
	topic    string
	audience string
	profile  string
	config   string
	folder   string
	job      string
	build    string
	limit    int
	tail     *int
	slim     bool
	tokens   []string
	current  string
}

var valueFlags = map[string]struct{}{
	"-p": {}, "--profile": {}, "--limit": {}, "--tail": {}, "-c": {}, "--config": {},
}

var boolFlags = map[string]struct{}{
	"-h": {}, "--help": {},
}

// parseArgs 自己切 argv。Audience 前缀和 -p/-c 可以出现在命令前后，交给 cobra 默认分词会对不齐。
func parseArgs(argv []string) (cmd, error) {
	if len(argv) > 0 && argv[0] == "completion" {
		if len(argv) > 1 && argv[1] == "fish" {
			return cmd{kind: "completion-fish"}, nil
		}
		return cmd{}, errors.New("usage: jenkins-cli completion fish")
	}
	if len(argv) > 0 && argv[0] == "__complete" {
		toks, cur := splitComplete(argv[1:])
		return cmd{kind: "complete", tokens: toks, current: cur}, nil
	}
	flags, pos, err := takeCmd(argv)
	if err != nil {
		return cmd{}, err
	}
	parsed, err := parseCmd(flags, pos)
	if err != nil {
		return cmd{}, err
	}
	parsed.config = firstNonEmpty(flags["-c"], flags["--config"])
	return parsed, nil
}

func parseCmd(flags map[string]string, pos []string) (cmd, error) {
	if _, ok := flags["-h"]; ok || flags["--help"] == "true" || at(pos, 0) == "help" {
		topic := at(pos, 0)
		if topic == "help" {
			topic = at(pos, 1)
		}
		return cmd{kind: "help", topic: topic}, nil
	}
	aud := "human"
	rest := pos
	if len(rest) > 0 && (rest[0] == "agent" || rest[0] == "human") {
		aud = rest[0]
		rest = rest[1:]
	}
	profile := firstNonEmpty(flags["-p"], flags["--profile"])
	if len(rest) == 0 {
		return cmd{kind: "help"}, nil
	}
	head := rest[0]
	if aud == "agent" {
		aliased, ok, err := parseAlias(head, rest[1:], flags, aud, profile)
		if err != nil {
			return cmd{}, err
		}
		if ok {
			return aliased, nil
		}
	}
	switch head {
	case "job":
		return parseJob(rest[1:], aud, profile)
	case "run":
		return parseRun(rest[1:], flags, aud, profile)
	case "log":
		return parseLog(rest[1:], flags, aud, profile)
	case "queue":
		return parseQueue(rest[1:], aud, profile)
	case "doctor":
		return cmd{kind: "doctor", audience: aud, profile: profile}, nil
	case "version":
		return cmd{kind: "version"}, nil
	default:
		return cmd{}, fmt.Errorf("unknown command: %s", head)
	}
}

func helpText(topic string) string {
	switch topic {
	case "job":
		return `jenkins-cli job

Usage:
  jenkins-cli job ls [folder]
  jenkins-cli job view <jobPath>
`
	case "run":
		return `jenkins-cli run

Usage:
  jenkins-cli run ls <jobPath> [--limit N]
  jenkins-cli run view <jobPath> <buildNo>
  jenkins-cli run start <jobPath>
  jenkins-cli run cancel <jobPath> <buildNo>
  jenkins-cli run rerun <jobPath> <buildNo>
`
	case "agent":
		return `jenkins-cli agent

Usage:
  jenkins-cli agent status <jobPath> [buildNo]
  jenkins-cli agent list <jobPath> [--limit N]
  jenkins-cli agent info <jobPath> <buildNo>
  jenkins-cli agent log <jobPath> <buildNo> [--tail N]
  jenkins-cli agent trigger <jobPath>
`
	default:
		return `jenkins-cli

Usage:
  jenkins-cli [-c config.json] [-p profile] [agent|human] <command>

Commands:
  job ls|view
  run ls|view|start|cancel|rerun
  log <jobPath> <buildNo> [--tail N]
  queue ls
  completion fish

Agent aliases:
  status list info log trigger
`
	}
}

func parseAlias(head string, rest []string, flags map[string]string, aud, profile string) (cmd, bool, error) {
	switch head {
	case "status":
		job, err := need(at(rest, 0), "status: missing <jobPath>")
		if err != nil {
			return cmd{}, false, err
		}
		build := "lastBuild"
		if len(rest) > 1 {
			build = rest[1]
		}
		return cmd{kind: "run-view", audience: aud, profile: profile, job: job, build: build, slim: true}, true, nil
	case "list":
		job, err := need(at(rest, 0), "list: missing <jobPath>")
		if err != nil {
			return cmd{}, false, err
		}
		lim, err := flagInt(flags, "--limit", 10)
		if err != nil {
			return cmd{}, false, err
		}
		return cmd{kind: "run-ls", audience: aud, profile: profile, job: job, limit: lim}, true, nil
	case "info":
		job, err := need(at(rest, 0), "info: missing <jobPath>")
		if err != nil {
			return cmd{}, false, err
		}
		build, err := need(at(rest, 1), "info: missing <buildNo>")
		if err != nil {
			return cmd{}, false, err
		}
		return cmd{kind: "run-view", audience: aud, profile: profile, job: job, build: build, slim: false}, true, nil
	case "trigger":
		job, err := need(at(rest, 0), "trigger: missing <jobPath>")
		if err != nil {
			return cmd{}, false, err
		}
		return cmd{kind: "run-start", audience: aud, profile: profile, job: job}, true, nil
	default:
		return cmd{}, false, nil
	}
}

func parseJob(argv []string, aud, profile string) (cmd, error) {
	sub := at(argv, 0)
	if sub == "" || sub == "help" {
		return cmd{kind: "help", topic: "job"}, nil
	}
	if sub == "ls" {
		return cmd{kind: "job-ls", audience: aud, profile: profile, folder: at(argv, 1)}, nil
	}
	if sub == "view" {
		job, err := need(at(argv, 1), "job view: missing <jobPath>")
		if err != nil {
			return cmd{}, err
		}
		return cmd{kind: "job-view", audience: aud, profile: profile, job: job}, nil
	}
	return cmd{}, fmt.Errorf("unknown command: job %s", sub)
}

func parseRun(argv []string, flags map[string]string, aud, profile string) (cmd, error) {
	sub := at(argv, 0)
	if sub == "" || sub == "help" {
		return cmd{kind: "help", topic: "run"}, nil
	}
	if sub == "ls" {
		job, err := need(at(argv, 1), "run ls: missing <jobPath>")
		if err != nil {
			return cmd{}, err
		}
		lim, err := flagInt(flags, "--limit", 10)
		if err != nil {
			return cmd{}, err
		}
		return cmd{kind: "run-ls", audience: aud, profile: profile, job: job, limit: lim}, nil
	}
	if sub == "view" {
		job, err := need(at(argv, 1), "run view: missing <jobPath>")
		if err != nil {
			return cmd{}, err
		}
		build, err := need(at(argv, 2), "run view: missing <buildNo>")
		if err != nil {
			return cmd{}, err
		}
		return cmd{kind: "run-view", audience: aud, profile: profile, job: job, build: build}, nil
	}
	if sub == "start" {
		job, err := need(at(argv, 1), "run start: missing <jobPath>")
		if err != nil {
			return cmd{}, err
		}
		return cmd{kind: "run-start", audience: aud, profile: profile, job: job}, nil
	}
	if sub == "cancel" {
		job, err := need(at(argv, 1), "run cancel: missing <jobPath>")
		if err != nil {
			return cmd{}, err
		}
		build, err := need(at(argv, 2), "run cancel: missing <buildNo>")
		if err != nil {
			return cmd{}, err
		}
		return cmd{kind: "run-cancel", audience: aud, profile: profile, job: job, build: build}, nil
	}
	if sub == "rerun" {
		job, err := need(at(argv, 1), "run rerun: missing <jobPath>")
		if err != nil {
			return cmd{}, err
		}
		build, err := need(at(argv, 2), "run rerun: missing <buildNo>")
		if err != nil {
			return cmd{}, err
		}
		return cmd{kind: "run-rerun", audience: aud, profile: profile, job: job, build: build}, nil
	}
	return cmd{}, fmt.Errorf("unknown command: run %s", sub)
}

func parseLog(argv []string, flags map[string]string, aud, profile string) (cmd, error) {
	job, err := need(at(argv, 0), "log: missing <jobPath>")
	if err != nil {
		return cmd{}, err
	}
	build, err := need(at(argv, 1), "log: missing <buildNo>")
	if err != nil {
		return cmd{}, err
	}
	tail, err := flagTail(flags)
	if err != nil {
		return cmd{}, err
	}
	return cmd{kind: "log", audience: aud, profile: profile, job: job, build: build, tail: tail}, nil
}

func parseQueue(argv []string, aud, profile string) (cmd, error) {
	sub := at(argv, 0)
	if sub == "" || sub == "help" {
		return cmd{kind: "help", topic: "queue"}, nil
	}
	if sub == "ls" {
		return cmd{kind: "queue-ls", audience: aud, profile: profile}, nil
	}
	return cmd{}, fmt.Errorf("unknown command: queue %s", sub)
}

func splitComplete(argv []string) ([]string, string) {
	if len(argv) == 0 {
		return nil, ""
	}
	cur := argv[len(argv)-1]
	toks := append([]string{}, argv[:len(argv)-1]...)
	return toks, cur
}

func takeCmd(argv []string) (map[string]string, []string, error) {
	flags := map[string]string{}
	var pos []string
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if _, ok := valueFlags[tok]; ok {
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("%s requires a value", tok)
			}
			flags[tok] = argv[i+1]
			i++
			continue
		}
		if _, ok := boolFlags[tok]; ok {
			flags[tok] = "true"
			continue
		}
		if strings.HasPrefix(tok, "-") {
			return nil, nil, fmt.Errorf("unknown flag: %s", tok)
		}
		pos = append(pos, tok)
	}
	return flags, pos, nil
}

func flagInt(flags map[string]string, key string, def int) (int, error) {
	raw, ok := flags[key]
	if !ok {
		return def, nil
	}
	n, good := jsParseInt(raw)
	if !good || n <= 0 {
		return 0, fmt.Errorf("invalid number: %s", raw)
	}
	return n, nil
}

func flagTail(flags map[string]string) (*int, error) {
	raw, ok := flags["--tail"]
	if !ok {
		return nil, nil
	}
	n, good := jsParseInt(raw)
	if !good || n <= 0 {
		return nil, fmt.Errorf("invalid number: %s", raw)
	}
	return &n, nil
}

// jsParseInt 对齐 JS parseInt：跳过空白，读到第一个非数字为止。非法或非正数由调用方拒绝。
func jsParseInt(raw string) (int, bool) {
	i := 0
	for i < len(raw) && isJSSpace(raw[i]) {
		i++
	}
	if i >= len(raw) {
		return 0, false
	}
	sign := int64(1)
	if raw[i] == '+' || raw[i] == '-' {
		if raw[i] == '-' {
			sign = -1
		}
		i++
	}
	if i >= len(raw) || raw[i] < '0' || raw[i] > '9' {
		return 0, false
	}
	var n int64
	for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
		if n > (math.MaxInt64-9)/10 {
			return 0, false
		}
		n = n*10 + int64(raw[i]-'0')
		i++
	}
	out := sign * n
	if out > math.MaxInt || out < math.MinInt {
		return 0, false
	}
	return int(out), true
}

func isJSSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func need(v, msg string) (string, error) {
	if strings.TrimSpace(v) == "" {
		return "", errors.New(msg)
	}
	return v, nil
}

func at(pos []string, i int) string {
	if i < 0 || i >= len(pos) {
		return ""
	}
	return pos[i]
}
