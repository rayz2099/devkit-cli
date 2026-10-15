package rclone

// 为什么: 自己切 argv，agent 前缀和 --dry-run 才能出现在任务名前面，不靠 cobra 吃掉下一个词。

var xferActions = []string{"push", "pull", "check", "get", "download"}

type cliCmd struct {
	kind     string
	detail   string
	audience string
	action   string
	task     string
	hasTask  bool
	path     string
	hasPath  bool
	dest     string
	hasDest  bool
	dryRun   bool
	verbose  bool
	limit    int
	cfgPath  string
	tokens   []string
	current  string
}

func parseArgs(argv []string) (cliCmd, error) {
	if len(argv) > 0 && argv[0] == "completion" {
		if len(argv) == 2 && argv[1] == "fish" {
			return cliCmd{kind: "completion-fish"}, nil
		}
		return cliCmd{}, fail(2, "usage: rclone-cli completion fish")
	}
	if len(argv) > 0 && argv[0] == "__complete" {
		return parseComplete(argv[1:]), nil
	}
	taken, err := takeCmd(argv)
	if err != nil {
		return cliCmd{}, err
	}
	if taken.flags["-h"] == "true" || taken.flags["--help"] == "true" || first(taken.pos) == "help" {
		return cliCmd{kind: "help"}, nil
	}
	aud, pos := takeAudience(taken.pos)
	head := first(pos)
	if head == "" {
		return cliCmd{kind: "usage", audience: aud, cfgPath: taken.cfg}, nil
	}
	if head == "version" {
		if err := assertNoExtra(pos, 1, taken.flags); err != nil {
			return cliCmd{}, err
		}
		if err := rejectLimit(taken.flags); err != nil {
			return cliCmd{}, err
		}
		return cliCmd{kind: "version", cfgPath: taken.cfg}, nil
	}
	if head == "doctor" {
		if err := assertNoExtra(pos, 1, taken.flags); err != nil {
			return cliCmd{}, err
		}
		if err := rejectLimit(taken.flags); err != nil {
			return cliCmd{}, err
		}
		return cliCmd{kind: "doctor", audience: aud, cfgPath: taken.cfg}, nil
	}
	if head == "ls" {
		return parseLs(taken, aud, pos)
	}
	if head == "runs" {
		if err := assertNoExtra(pos, 1, taken.flags); err != nil {
			return cliCmd{}, err
		}
		if err := rejectLimit(taken.flags); err != nil {
			return cliCmd{}, err
		}
		return cliCmd{kind: "runs", audience: aud, cfgPath: taken.cfg}, nil
	}
	if !isAction(head) {
		return cliCmd{kind: "usage", detail: "unknown command: " + head, audience: aud, cfgPath: taken.cfg}, nil
	}
	if err := rejectLimit(taken.flags); err != nil {
		return cliCmd{}, err
	}
	if head == "check" && taken.flags["--dry-run"] == "true" {
		return cliCmd{}, fail(2, "check does not take --dry-run")
	}
	task := at(pos, 1)
	if task == "" {
		return cliCmd{kind: "usage", audience: aud, cfgPath: taken.cfg}, nil
	}
	path := at(pos, 2)
	if (head == "get" || head == "download") && path == "" {
		return cliCmd{kind: "usage", detail: head + " requires a path", audience: aud, cfgPath: taken.cfg}, nil
	}
	dest := at(pos, 3)
	extraAt := 3
	if head == "get" || head == "download" {
		extraAt = 4
	}
	if len(pos) > extraAt {
		return cliCmd{}, fail(2, "unexpected argument: %s", at(pos, extraAt))
	}
	cmd := cliCmd{
		kind:     "xfer",
		action:   head,
		audience: aud,
		task:     task,
		dryRun:   head != "check" && taken.flags["--dry-run"] == "true",
		verbose:  taken.flags["-v"] == "true" || taken.flags["--verbose"] == "true",
		cfgPath:  taken.cfg,
	}
	if len(pos) > 2 {
		cmd.hasPath = true
		cmd.path = path
	}
	if (head == "get" || head == "download") && len(pos) > 3 {
		cmd.hasDest = true
		cmd.dest = dest
	}
	return cmd, nil
}

func parseLs(taken takenCmd, aud string, pos []string) (cliCmd, error) {
	if taken.flags["--dry-run"] == "true" {
		return cliCmd{}, fail(2, "unknown flag: --dry-run")
	}
	if len(pos) > 3 {
		return cliCmd{}, fail(2, "unexpected argument: %s", at(pos, 3))
	}
	task := at(pos, 1)
	if task == "" {
		if _, ok := taken.flags["--limit"]; ok {
			return cliCmd{}, fail(2, "ls without a task does not take --limit")
		}
		return cliCmd{kind: "ls", audience: aud, cfgPath: taken.cfg}, nil
	}
	limit, err := readLimit(taken.flags, 10)
	if err != nil {
		return cliCmd{}, err
	}
	cmd := cliCmd{kind: "ls", audience: aud, task: task, hasTask: true, limit: limit, cfgPath: taken.cfg}
	if len(pos) > 2 {
		cmd.hasPath = true
		cmd.path = at(pos, 2)
	}
	return cmd, nil
}

func usageText(names []string, detail string) string {
	lines := []string{
		"usage:",
		"  rclone-cli push <task> [path] [--dry-run]",
		"  rclone-cli pull <task> [path] [--dry-run]",
		"  rclone-cli check <task> [path]",
		"  rclone-cli get <task> <path> [dest] [--dry-run] [-v]",
		"  rclone-cli download <task> <path> [dest] [--dry-run] [-v]",
		"  rclone-cli ls",
		"  rclone-cli ls <task> [path] [--limit n]",
		"  rclone-cli runs",
		"  rclone-cli completion fish",
		"",
		"no default task; a task name is required.",
		"",
		"tasks:",
	}
	if len(names) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, name := range names {
			lines = append(lines, "  "+name)
		}
	}
	if detail != "" {
		lines = append(lines, "", detail)
	}
	return joinLines(lines)
}

func helpText() string {
	return joinLines([]string{
		"usage:",
		"  rclone-cli push <task> [path] [--dry-run]",
		"  rclone-cli pull <task> [path] [--dry-run]",
		"  rclone-cli check <task> [path]",
		"  rclone-cli get <task> <path> [dest] [--dry-run] [-v]",
		"  rclone-cli download <task> <path> [dest] [--dry-run] [-v]",
		"  rclone-cli ls",
		"  rclone-cli ls <task> [path] [--limit n]",
		"  rclone-cli runs",
		"  rclone-cli completion fish",
		"",
		"no default task; a task name is required.",
		"ls with no task prints config. ls <task> lists files, default --limit 10.",
		"check lists both sides and compares size; leftover .bin counts as present.",
		"get writes one decrypted file into the source tree unless dest is set.",
		"download writes one file to the current directory unless dest is set.",
		"pull restores a directory into the source tree; a file path is download.",
		"-v / --verbose prints rclone logs and key-pair retries. default is quiet.",
	})
}

func parseComplete(argv []string) cliCmd {
	tokens := append([]string{}, argv...)
	current := ""
	if len(tokens) > 0 {
		current = tokens[len(tokens)-1]
		tokens = tokens[:len(tokens)-1]
	}
	return cliCmd{kind: "complete", tokens: tokens, current: current}
}

type takenCmd struct {
	flags map[string]string
	pos   []string
	cfg   string
}

func takeCmd(argv []string) (takenCmd, error) {
	flags := map[string]string{}
	var pos []string
	for i := 0; i < len(argv); i++ {
		token := argv[i]
		if isBoolFlag(token) {
			flags[token] = "true"
			continue
		}
		if len(token) > len("--limit=") && token[:len("--limit=")] == "--limit=" {
			flags["--limit"] = token[len("--limit="):]
			continue
		}
		if token == "--config" || token == "-c" {
			val, next, err := takeVal(argv, i, token)
			if err != nil {
				return takenCmd{}, err
			}
			flags["config"] = val
			i = next
			continue
		}
		if len(token) > len("--config=") && token[:len("--config=")] == "--config=" {
			flags["config"] = token[len("--config="):]
			continue
		}
		if token == "--limit" || token == "-n" {
			val, next, err := takeVal(argv, i, token)
			if err != nil {
				return takenCmd{}, err
			}
			flags["--limit"] = val
			i = next
			continue
		}
		if len(token) > 0 && token[0] == '-' {
			return takenCmd{}, fail(2, "unknown flag: %s", token)
		}
		pos = append(pos, token)
	}
	return takenCmd{flags: flags, pos: pos, cfg: flags["config"]}, nil
}

func takeVal(argv []string, i int, token string) (string, int, error) {
	if i+1 >= len(argv) || (len(argv[i+1]) > 0 && argv[i+1][0] == '-') {
		return "", 0, fail(2, "%s requires a value", token)
	}
	return argv[i+1], i + 1, nil
}

func isBoolFlag(token string) bool {
	switch token {
	case "-h", "--help", "--dry-run", "-v", "--verbose":
		return true
	default:
		return false
	}
}

func readLimit(flags map[string]string, whenOmit int) (int, error) {
	raw, ok := flags["--limit"]
	if !ok {
		return whenOmit, nil
	}
	if !positiveInt(raw) {
		return 0, fail(2, "--limit must be a positive integer")
	}
	n := 0
	for _, ch := range raw {
		n = n*10 + int(ch-'0')
	}
	return n, nil
}

func positiveInt(raw string) bool {
	if raw == "" || raw[0] < '1' || raw[0] > '9' {
		return false
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func takeAudience(pos []string) (string, []string) {
	if len(pos) == 0 {
		return "human", pos
	}
	if pos[0] != "agent" && pos[0] != "human" {
		return "human", pos
	}
	return pos[0], pos[1:]
}

func assertNoExtra(pos []string, count int, flags map[string]string) error {
	if len(pos) > count {
		return fail(2, "unexpected argument: %s", at(pos, count))
	}
	if flags["--dry-run"] == "true" {
		return fail(2, "unknown flag: --dry-run")
	}
	return nil
}

func rejectLimit(flags map[string]string) error {
	if _, ok := flags["--limit"]; ok {
		return fail(2, "unknown flag: --limit")
	}
	return nil
}

func isAction(value string) bool {
	for _, item := range xferActions {
		if item == value {
			return true
		}
	}
	return false
}

func first(pos []string) string {
	if len(pos) == 0 {
		return ""
	}
	return pos[0]
}

func at(pos []string, i int) string {
	if i < 0 || i >= len(pos) {
		return ""
	}
	return pos[i]
}

func joinLines(lines []string) string {
	out := ""
	for i, line := range lines {
		if i > 0 {
			out += "\n"
		}
		out += line
	}
	return out
}
