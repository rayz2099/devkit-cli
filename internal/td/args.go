package td

import (
	"fmt"
	"strings"
	"time"
)

const (
	kindHelp        = "help"
	kindFish        = "completion-fish"
	kindComplete    = "complete"
	kindLogin       = "auth-login"
	kindStatus      = "auth-status"
	kindProjects    = "project-ls"
	kindEvents      = "meta-events"
	kindProps       = "meta-props"
	kindReportLs    = "report-ls"
	kindReportGet   = "report-get"
	kindQueryEvent  = "query-event"
	kindQueryProp   = "query-property"
	kindQueryRun    = "query-run"
	kindQueryReport = "query-report"
	kindVersion     = "version"
	kindDoctor      = "doctor"
)

// Cmd 是一次解析结果. 旗标可以出现在命令前后, 所以不交给 cobra 的旗标集.
type Cmd struct {
	Kind      string
	Audience  string
	Topic     string
	Profile   string
	Config    string
	Project   string
	Event     *string
	ReportID  string
	EventName string
	Analysis  string
	Quota     string
	Group     *string
	Recent    *string
	From      *string
	To        *string
	Particle  string
	QP        *string
	Model     int
	ModelSet  bool
	Timeout   time.Duration
	Tokens    []string
	Current   string
}

var valueFlags = map[string]struct{}{
	"-c": {}, "--config": {}, "-p": {}, "--profile": {},
	"--project": {}, "--event": {}, "--id": {}, "--analysis": {},
	"--quota": {}, "--group": {}, "--qp": {}, "--recent": {},
	"--from": {}, "--to": {}, "--particle": {}, "--model": {},
	"--timeout": {},
}

var boolFlags = map[string]struct{}{
	"-h": {}, "--help": {},
}

var writeWords = map[string]struct{}{
	"save": {}, "delete": {}, "del": {}, "update": {}, "create": {},
	"edit": {}, "import": {}, "upload": {}, "download": {}, "remove": {},
}

var qpMix = []string{
	"--event", "--analysis", "--quota", "--group", "--recent", "--from", "--to", "--particle",
}

func parseArgs(argv []string) (Cmd, error) {
	if len(argv) > 0 && argv[0] == "completion" {
		if len(argv) > 1 && argv[1] == "fish" {
			return Cmd{Kind: kindFish}, nil
		}
		return Cmd{}, fmt.Errorf("usage: td-cli completion fish")
	}
	if len(argv) > 0 && argv[0] == "__complete" {
		return parseComplete(argv[1:]), nil
	}
	flags, pos, err := takeCmd(argv)
	if err != nil {
		return Cmd{}, err
	}
	cmd, err := parseCmd(flags, pos)
	if err != nil {
		return Cmd{}, err
	}
	return attachCfg(cmd, flags), nil
}

func helpText(topic string) string {
	if topic == "auth" {
		return `td-cli auth

Usage:
  td-cli auth login
  td-cli auth status

账号在 profile 的 url、username、password 里. login 只验证, 不把令牌写回配置.
`
	}
	if topic == "query" {
		return `td-cli query

Usage:
  td-cli query event --project ID [--event NAME] [--analysis A100] [--recent 1-7] [--from TIME --to TIME] [--particle T1] [--group COL] [--qp FILE]
  td-cli query property --project ID [--analysis A101] [--quota PROP] [--group COL] [--qp FILE]
  td-cli query run --project ID --model N --qp FILE
  td-cli query report --project ID --id REPORT [--model N]

Models: 0 event, 1 retention, 2 funnel, 3 property, 4 trace, 7 scatter, 12 interval, 14 attribution
--group 用 name, event:name 或 user:name
--qp 给出页面 QP JSON, 此时不能再带其它查询条件
`
	}
	return `td-cli

Usage:
  td-cli [-c config.json] [-p profile] [agent|human] <command>

Commands:
  auth login|status
  project ls
  meta events|props --project ID [--event NAME]
  report ls|get --project ID [--id REPORT]
  query event|property|run|report
  completion fish

只读. 每次用 profile 的 username 和 password 登录, 不走设备码, 不保存令牌.
`
}

// takeCmd 把旗标和位置参数拆开. 值不能以 - 开头, 避免把下一个旗标吃掉.
func takeCmd(argv []string) (map[string]string, []string, error) {
	flags := map[string]string{}
	var pos []string
	for i := 0; i < len(argv); i++ {
		token := argv[i]
		if _, ok := valueFlags[token]; ok {
			if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
				return nil, nil, fmt.Errorf("%s requires a value", token)
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
			return nil, nil, fmt.Errorf("unknown flag: %s", token)
		}
		pos = append(pos, token)
	}
	return flags, pos, nil
}

func parseCmd(flags map[string]string, pos []string) (Cmd, error) {
	_, helpShort := flags["-h"]
	_, helpLong := flags["--help"]
	if helpShort || helpLong || (len(pos) > 0 && pos[0] == "help") {
		topic := ""
		if len(pos) > 0 && pos[0] == "help" && len(pos) > 1 {
			topic = pos[1]
		}
		return Cmd{Kind: kindHelp, Audience: "human", Topic: topic}, nil
	}
	audience := "human"
	rest := append([]string{}, pos...)
	if len(rest) > 0 && (rest[0] == "agent" || rest[0] == "human") {
		audience = rest[0]
		rest = rest[1:]
	}
	profile := flag(flags, "-p", "--profile")
	if len(rest) == 0 {
		return Cmd{Kind: kindHelp, Audience: audience, Profile: profile}, nil
	}
	head := rest[0]
	if _, ok := writeWords[head]; ok {
		return Cmd{}, fmt.Errorf("read-only cli, write is disabled")
	}
	scope := Cmd{Audience: audience, Profile: profile}
	switch head {
	case "auth":
		return parseAuth(rest[1:], scope)
	case "project":
		return parseProject(rest[1:], scope)
	case "meta":
		return parseMeta(rest[1:], flags, scope)
	case "report":
		return parseReport(rest[1:], flags, scope)
	case "query":
		return parseQuery(rest[1:], flags, scope)
	case "version":
		if len(rest) > 1 {
			return Cmd{}, fmt.Errorf("unknown command: version %s", rest[1])
		}
		scope.Kind = kindVersion
		return scope, nil
	case "doctor":
		if len(rest) > 1 {
			return Cmd{}, fmt.Errorf("unknown command: doctor %s", rest[1])
		}
		scope.Kind = kindDoctor
		return scope, nil
	default:
		return Cmd{}, fmt.Errorf("unknown command: %s", head)
	}
}

func parseAuth(argv []string, scope Cmd) (Cmd, error) {
	if len(argv) == 0 || argv[0] == "help" {
		return Cmd{Kind: kindHelp, Audience: scope.Audience, Topic: "auth", Profile: scope.Profile}, nil
	}
	if _, ok := writeWords[argv[0]]; ok {
		return Cmd{}, fmt.Errorf("read-only cli, write is disabled")
	}
	switch argv[0] {
	case "login":
		scope.Kind = kindLogin
		return scope, nil
	case "status":
		scope.Kind = kindStatus
		return scope, nil
	default:
		return Cmd{}, fmt.Errorf("unknown command: auth %s", argv[0])
	}
}

func parseProject(argv []string, scope Cmd) (Cmd, error) {
	if len(argv) == 0 || argv[0] == "help" {
		return Cmd{Kind: kindHelp, Audience: scope.Audience, Profile: scope.Profile}, nil
	}
	if _, ok := writeWords[argv[0]]; ok {
		return Cmd{}, fmt.Errorf("read-only cli, write is disabled")
	}
	if argv[0] != "ls" {
		return Cmd{}, fmt.Errorf("unknown command: project %s", argv[0])
	}
	scope.Kind = kindProjects
	return scope, nil
}

func parseMeta(argv []string, flags map[string]string, scope Cmd) (Cmd, error) {
	if len(argv) == 0 || argv[0] == "help" {
		return Cmd{Kind: kindHelp, Audience: scope.Audience, Profile: scope.Profile}, nil
	}
	if _, ok := writeWords[argv[0]]; ok {
		return Cmd{}, fmt.Errorf("read-only cli, write is disabled")
	}
	project, err := need(flag(flags, "--project"), "missing --project")
	if err != nil {
		return Cmd{}, err
	}
	scope.Project = project
	switch argv[0] {
	case "events":
		scope.Kind = kindEvents
		return scope, nil
	case "props":
		scope.Kind = kindProps
		if ev := flag(flags, "--event"); ev != "" {
			scope.Event = &ev
		}
		return scope, nil
	default:
		return Cmd{}, fmt.Errorf("unknown command: meta %s", argv[0])
	}
}

func parseReport(argv []string, flags map[string]string, scope Cmd) (Cmd, error) {
	if len(argv) == 0 || argv[0] == "help" {
		return Cmd{Kind: kindHelp, Audience: scope.Audience, Profile: scope.Profile}, nil
	}
	if _, ok := writeWords[argv[0]]; ok {
		return Cmd{}, fmt.Errorf("read-only cli, write is disabled")
	}
	project, err := need(flag(flags, "--project"), "missing --project")
	if err != nil {
		return Cmd{}, err
	}
	scope.Project = project
	switch argv[0] {
	case "ls":
		scope.Kind = kindReportLs
		return scope, nil
	case "get":
		id, err := need(flag(flags, "--id"), "missing --id")
		if err != nil {
			return Cmd{}, err
		}
		scope.Kind = kindReportGet
		scope.ReportID = id
		return scope, nil
	default:
		return Cmd{}, fmt.Errorf("unknown command: report %s", argv[0])
	}
}

func parseQuery(argv []string, flags map[string]string, scope Cmd) (Cmd, error) {
	if len(argv) == 0 || argv[0] == "help" {
		return Cmd{Kind: kindHelp, Audience: scope.Audience, Topic: "query", Profile: scope.Profile}, nil
	}
	if _, ok := writeWords[argv[0]]; ok {
		return Cmd{}, fmt.Errorf("read-only cli, write is disabled")
	}
	project, err := need(flag(flags, "--project"), "missing --project")
	if err != nil {
		return Cmd{}, err
	}
	timeout, err := readTimeout(flag(flags, "--timeout"))
	if err != nil {
		return Cmd{}, err
	}
	scope.Project = project
	scope.Timeout = timeout
	var qp *string
	if raw := flag(flags, "--qp"); raw != "" {
		qp = &raw
	}
	switch argv[0] {
	case "event":
		if err := rejectQPMix(flags, qp); err != nil {
			return Cmd{}, err
		}
		scope.Kind = kindQueryEvent
		scope.EventName = "anyEvent"
		if name := flag(flags, "--event"); name != "" {
			scope.EventName = name
		}
		scope.Analysis = "A100"
		if analysis := flag(flags, "--analysis"); analysis != "" {
			scope.Analysis = analysis
		}
		scope.Particle = "T1"
		if particle := flag(flags, "--particle"); particle != "" {
			scope.Particle = particle
		}
		scope.Recent = opt(flag(flags, "--recent"))
		scope.From = opt(flag(flags, "--from"))
		scope.To = opt(flag(flags, "--to"))
		scope.Group = opt(flag(flags, "--group"))
		scope.QP = qp
		return scope, nil
	case "property":
		if err := rejectQPMix(flags, qp); err != nil {
			return Cmd{}, err
		}
		scope.Kind = kindQueryProp
		scope.Analysis = "A101"
		if analysis := flag(flags, "--analysis"); analysis != "" {
			scope.Analysis = analysis
		}
		scope.Quota = flag(flags, "--quota")
		scope.Group = opt(flag(flags, "--group"))
		scope.Recent = opt(flag(flags, "--recent"))
		scope.From = opt(flag(flags, "--from"))
		scope.To = opt(flag(flags, "--to"))
		scope.QP = qp
		return scope, nil
	case "run":
		modelRaw, err := need(flag(flags, "--model"), "missing --model")
		if err != nil {
			return Cmd{}, err
		}
		model, err := asInt(modelRaw, "--model")
		if err != nil {
			return Cmd{}, err
		}
		qpRaw, err := need(flag(flags, "--qp"), "missing --qp")
		if err != nil {
			return Cmd{}, err
		}
		scope.Kind = kindQueryRun
		scope.Model = model
		scope.ModelSet = true
		scope.QP = &qpRaw
		return scope, nil
	case "report":
		id, err := need(flag(flags, "--id"), "missing --id")
		if err != nil {
			return Cmd{}, err
		}
		scope.Kind = kindQueryReport
		scope.ReportID = id
		if raw := flag(flags, "--model"); raw != "" {
			model, err := asInt(raw, "--model")
			if err != nil {
				return Cmd{}, err
			}
			scope.Model = model
			scope.ModelSet = true
		}
		return scope, nil
	default:
		return Cmd{}, fmt.Errorf("unknown command: query %s", argv[0])
	}
}

func rejectQPMix(flags map[string]string, qp *string) error {
	if qp == nil {
		return nil
	}
	for _, name := range qpMix {
		if _, ok := flags[name]; ok {
			return fmt.Errorf("--qp replaces other query flags")
		}
	}
	return nil
}

func readTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		return 120 * time.Second, nil
	}
	sec, err := asInt(raw, "--timeout")
	if err != nil {
		return 0, err
	}
	if sec <= 0 {
		return 0, fmt.Errorf("--timeout must be > 0")
	}
	return time.Duration(sec) * time.Second, nil
}

func attachCfg(cmd Cmd, flags map[string]string) Cmd {
	if cmd.Kind == kindFish || cmd.Kind == kindComplete {
		return cmd
	}
	cmd.Config = flag(flags, "-c", "--config")
	return cmd
}

func parseComplete(argv []string) Cmd {
	tokens := append([]string{}, argv...)
	current := ""
	if len(tokens) > 0 {
		current = tokens[len(tokens)-1]
		tokens = tokens[:len(tokens)-1]
	}
	return Cmd{Kind: kindComplete, Tokens: tokens, Current: current}
}

func flag(flags map[string]string, names ...string) string {
	for _, name := range names {
		value, ok := flags[name]
		if ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func need(value, message string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s", message)
	}
	return value, nil
}

func opt(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func audienceOf(argv []string) string {
	_, pos, err := takeCmd(argv)
	if err != nil {
		return "human"
	}
	if len(pos) > 0 && (pos[0] == "agent" || pos[0] == "human") {
		return pos[0]
	}
	return "human"
}
