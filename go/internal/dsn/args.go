package dsn

import "strings"

type audience string

const (
	audHuman audience = "human"
	audAgent audience = "agent"
)

type cmd struct {
	kind     string
	topic    string
	audience audience
	profile  string
	hasProf  bool
	stmt     string
	output   string
	pretty   bool
	limit    *int
	connect  float64
	exec     float64
	config   string
	tokens   []string
	current  string
}

var valueFlags = map[string]struct{}{
	"-p": {}, "--profile": {}, "--output": {}, "--limit": {},
	"--timeout": {}, "--connect-timeout": {}, "-c": {}, "--config": {},
}

var boolFlags = map[string]struct{}{
	"-h": {}, "--help": {}, "--pretty": {},
}

// parseArgs 自己切 argv。Audience 前缀和 -p 可以前后插，不能绑死在某个子命令上。
func parseArgs(argv []string) (cmd, error) {
	if len(argv) > 0 && argv[0] == "completion" {
		if len(argv) > 1 && argv[1] == "fish" {
			return cmd{kind: "completion-fish"}, nil
		}
		return cmd{}, exitf(1, "usage: dsn-cli completion fish")
	}
	if len(argv) > 0 && argv[0] == "__complete" {
		return parseComplete(argv[1:]), nil
	}
	if len(argv) > 0 && argv[0] == "version" {
		if len(argv) > 1 {
			return cmd{}, exitf(1, "version: unexpected argument")
		}
		return cmd{kind: "version"}, nil
	}
	flags, pos, err := takeCmd(argv)
	if err != nil {
		return cmd{}, exitf(1, "%s", err.Error())
	}
	parsed, err := parseCmd(flags, pos)
	if err != nil {
		return cmd{}, err
	}
	if cfg := firstNonEmpty(flags["-c"], flags["--config"]); cfg != "" {
		parsed.config = cfg
	}
	return parsed, nil
}

func parseCmd(flags map[string]string, pos []string) (cmd, error) {
	if _, help := flags["-h"]; help {
		return parseHelp(flags, pos), nil
	}
	if _, help := flags["--help"]; help {
		return parseHelp(flags, pos), nil
	}
	if len(pos) > 0 && pos[0] == "help" {
		return parseHelp(flags, pos), nil
	}
	aud := audHuman
	rest := append([]string(nil), pos...)
	if len(rest) > 0 && (rest[0] == "agent" || rest[0] == "human") {
		aud = audience(rest[0])
		rest = rest[1:]
	}
	profile, hasProf := profileFlag(flags)
	if len(rest) == 0 {
		if !hasProf {
			return cmd{kind: "help"}, nil
		}
		return cmd{kind: "console", audience: aud, profile: profile, hasProf: true}, nil
	}
	switch rest[0] {
	case "query":
		return parseQuery(rest[1:], flags, aud, profile, hasProf)
	case "doctor":
		return parseDoctor(rest[1:], flags, aud, profile, hasProf)
	case "ds":
		return parseDs(rest[1:], flags, aud, profile, hasProf)
	case "version":
		if len(rest) > 1 {
			return cmd{}, exitf(1, "version: unexpected argument")
		}
		return cmd{kind: "version"}, nil
	case "completion":
		return cmd{}, exitf(1, "usage: dsn-cli completion fish")
	default:
		return cmd{}, exitf(1, "unknown command: %s", rest[0])
	}
}

func parseQuery(rest []string, flags map[string]string, aud audience, profile string, hasProf bool) (cmd, error) {
	if !hasProf || strings.TrimSpace(profile) == "" {
		return cmd{}, exitf(1, "-p is required")
	}
	stmt := strings.TrimSpace(strings.Join(rest, " "))
	if stmt == "" {
		return cmd{}, exitf(1, "query: missing statement")
	}
	output, err := readOutput(flags["--output"], flagSet(flags, "--output"))
	if err != nil {
		return cmd{}, err
	}
	pretty := flagSet(flags, "--pretty")
	if err := assertPretty(output, pretty); err != nil {
		return cmd{}, err
	}
	limit, err := readLimit(flags["--limit"], flagSet(flags, "--limit"))
	if err != nil {
		return cmd{}, err
	}
	connect, err := readSec(flags["--connect-timeout"], flagSet(flags, "--connect-timeout"), 3)
	if err != nil {
		return cmd{}, err
	}
	execSec, err := readSec(flags["--timeout"], flagSet(flags, "--timeout"), 30)
	if err != nil {
		return cmd{}, err
	}
	return cmd{
		kind: "query", audience: aud, profile: profile, hasProf: true,
		stmt: stmt, output: output, pretty: pretty, limit: limit,
		connect: connect, exec: execSec,
	}, nil
}

func parseDoctor(rest []string, flags map[string]string, aud audience, profile string, hasProf bool) (cmd, error) {
	if len(rest) > 0 {
		return cmd{}, exitf(1, "doctor: unexpected argument")
	}
	if hasProf && strings.TrimSpace(profile) == "" {
		return cmd{}, exitf(1, "-p is required")
	}
	output, err := readOutput(flags["--output"], flagSet(flags, "--output"))
	if err != nil {
		return cmd{}, err
	}
	pretty := flagSet(flags, "--pretty")
	if err := assertPretty(output, pretty); err != nil {
		return cmd{}, err
	}
	connect, err := readSec(flags["--connect-timeout"], flagSet(flags, "--connect-timeout"), 3)
	if err != nil {
		return cmd{}, err
	}
	execSec, err := readSec(flags["--timeout"], flagSet(flags, "--timeout"), 5)
	if err != nil {
		return cmd{}, err
	}
	out := cmd{
		kind: "doctor", audience: aud, output: output, pretty: pretty,
		connect: connect, exec: execSec,
	}
	if hasProf {
		out.profile = profile
		out.hasProf = true
	}
	return out, nil
}

func parseDs(rest []string, flags map[string]string, aud audience, profile string, hasProf bool) (cmd, error) {
	if len(rest) > 0 {
		return cmd{}, exitf(1, "ds: unexpected argument")
	}
	if hasProf && strings.TrimSpace(profile) == "" {
		return cmd{}, exitf(1, "-p is required")
	}
	output, err := readOutput(flags["--output"], flagSet(flags, "--output"))
	if err != nil {
		return cmd{}, err
	}
	pretty := flagSet(flags, "--pretty")
	if err := assertPretty(output, pretty); err != nil {
		return cmd{}, err
	}
	out := cmd{kind: "ds", audience: aud, output: output, pretty: pretty}
	if hasProf {
		out.profile = profile
		out.hasProf = true
	}
	return out, nil
}

func parseHelp(flags map[string]string, pos []string) cmd {
	var topic string
	if len(pos) > 0 && pos[0] == "help" {
		if len(pos) > 1 {
			topic = pos[1]
		}
	} else {
		topic = helpTopicOf(pos)
	}
	out := cmd{kind: "help", topic: topic}
	if profile, ok := profileFlag(flags); ok {
		out.profile = profile
		out.hasProf = true
	}
	if cfg := firstNonEmpty(flags["-c"], flags["--config"]); cfg != "" {
		out.config = cfg
	}
	return out
}

func helpTopicOf(pos []string) string {
	for _, item := range pos {
		if item == "query" || item == "doctor" || item == "ds" {
			return item
		}
	}
	if len(pos) > 0 {
		return pos[0]
	}
	return ""
}

func parseComplete(argv []string) cmd {
	if len(argv) == 0 {
		return cmd{kind: "complete", current: ""}
	}
	return cmd{kind: "complete", tokens: argv[:len(argv)-1], current: argv[len(argv)-1]}
}

func takeCmd(argv []string) (map[string]string, []string, error) {
	flags := map[string]string{}
	var pos []string
	for i := 0; i < len(argv); i++ {
		token := argv[i]
		if _, ok := valueFlags[token]; ok {
			if i+1 >= len(argv) {
				return nil, nil, exitf(1, "%s requires a value", token)
			}
			flags[token] = argv[i+1]
			i++
			continue
		}
		if _, ok := boolFlags[token]; ok {
			flags[token] = "true"
			continue
		}
		if strings.HasPrefix(token, "-") {
			return nil, nil, exitf(1, "unknown flag: %s", token)
		}
		pos = append(pos, token)
	}
	return flags, pos, nil
}

func profileFlag(flags map[string]string) (string, bool) {
	if val, ok := flags["-p"]; ok && strings.TrimSpace(val) != "" {
		return val, true
	}
	if val, ok := flags["--profile"]; ok && strings.TrimSpace(val) != "" {
		return val, true
	}
	if _, ok := flags["-p"]; ok {
		return "", true
	}
	if _, ok := flags["--profile"]; ok {
		return "", true
	}
	return "", false
}

func flagSet(flags map[string]string, name string) bool {
	_, ok := flags[name]
	return ok
}

func assertPretty(output string, pretty bool) error {
	if pretty && output != "json" {
		return exitf(1, "--pretty requires --output json")
	}
	return nil
}

func readOutput(raw string, set bool) (string, error) {
	if !set {
		return "table", nil
	}
	switch raw {
	case "json", "csv", "plain", "table":
		return raw, nil
	default:
		return "", exitf(1, "invalid --output: json|csv|plain")
	}
}

func readSec(raw string, set bool, def float64) (float64, error) {
	if !set {
		return def, nil
	}
	val, ok := parseFloatPrefix(strings.TrimSpace(raw))
	if !ok || val <= 0 {
		return 0, exitf(1, "invalid timeout: %s", raw)
	}
	return val, nil
}

func readLimit(raw string, set bool) (*int, error) {
	if !set {
		return nil, nil
	}
	val, ok := parseIntPrefix(strings.TrimSpace(raw))
	if !ok || val < 0 {
		return nil, exitf(1, "invalid --limit: %s", raw)
	}
	return &val, nil
}

func parseFloatPrefix(raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}
	i := 0
	if raw[0] == '+' || raw[0] == '-' {
		i++
	}
	saw := false
	dot := false
	for i < len(raw) {
		ch := raw[i]
		if ch >= '0' && ch <= '9' {
			saw = true
			i++
			continue
		}
		if ch == '.' && !dot {
			dot = true
			i++
			continue
		}
		break
	}
	if !saw {
		return 0, false
	}
	if i < len(raw) && (raw[i] == 'e' || raw[i] == 'E') {
		j := i + 1
		if j < len(raw) && (raw[j] == '+' || raw[j] == '-') {
			j++
		}
		exp := false
		for j < len(raw) && raw[j] >= '0' && raw[j] <= '9' {
			exp = true
			j++
		}
		if exp {
			i = j
		}
	}
	var n float64
	sign := 1.0
	s := raw[:i]
	if s[0] == '+' {
		s = s[1:]
	} else if s[0] == '-' {
		sign = -1
		s = s[1:]
	}
	intPart := 0.0
	frac := 0.0
	div := 1.0
	seenDot := false
	for _, ch := range s {
		if ch == '.' {
			seenDot = true
			continue
		}
		if !seenDot {
			intPart = intPart*10 + float64(ch-'0')
			continue
		}
		div *= 10
		frac = frac*10 + float64(ch-'0')
	}
	n = sign * (intPart + frac/div)
	return n, true
}

func parseIntPrefix(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	i := 0
	sign := 1
	if raw[0] == '+' || raw[0] == '-' {
		if raw[0] == '-' {
			sign = -1
		}
		i++
	}
	if i >= len(raw) || raw[i] < '0' || raw[i] > '9' {
		return 0, false
	}
	n := 0
	for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
		n = n*10 + int(raw[i]-'0')
		i++
	}
	return sign * n, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
