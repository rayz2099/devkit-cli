package rclone

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Deps 是一次命令要用的外部依赖。单测换掉 HOME、时钟和 rclone，不能打到真实网盘。
type Deps struct {
	Home   string
	Spawn  SpawnFn
	Now    func() time.Time
	ID     func() (string, error)
	Cwd    string
	Stdout io.Writer
	Stderr io.Writer
}

// CmdOut 是给 stdout 的正文和进程码。传输失败仍走 stdout，不能跟用法错误混在 stderr。
type CmdOut struct {
	Text string
	Code int
}

func runCmd(argv []string, deps Deps) (CmdOut, error) {
	cmd, err := parseArgs(argv)
	if err != nil {
		return CmdOut{}, err
	}
	switch cmd.kind {
	case "help":
		return CmdOut{Text: helpText()}, nil
	case "version":
		return CmdOut{Text: "rclone-cli 0.1.0"}, nil
	case "doctor":
		return CmdOut{}, runDoctor(deps.Stdout, deps.Home, cmd.cfgPath)
	case "completion-fish":
		return CmdOut{Text: fishScript}, nil
	case "complete":
		text := completeLines(cmd.tokens, cmd.current, deps.Home, "")
		return CmdOut{Text: text}, nil
	case "usage":
		return CmdOut{}, failUsage(deps, cmd.cfgPath, cmd.detail)
	}
	if deps.Home == "" {
		return CmdOut{}, fail(1, "HOME is required")
	}
	path, err := cfgFile(cmd.cfgPath)
	if err != nil {
		return CmdOut{}, err
	}
	cfg, err := Load(path, deps.Home)
	if err != nil {
		return CmdOut{}, err
	}
	switch cmd.kind {
	case "ls":
		return runLs(cfg, cmd, deps)
	case "runs":
		recs, err := ReadRuns()
		if err != nil {
			return CmdOut{}, err
		}
		text, err := renderRuns(recs, cmd.audience)
		if err != nil {
			return CmdOut{}, err
		}
		return CmdOut{Text: text}, nil
	case "xfer":
		if cmd.action == "check" {
			report, err := runInv(cfg, cmd, deps, true)
			if err != nil {
				return CmdOut{}, err
			}
			text, err := renderInv(report, cmd.audience, "check")
			if err != nil {
				return CmdOut{}, err
			}
			code := 0
			if checkFailed(report) {
				code = 1
			}
			return CmdOut{Text: text, Code: code}, nil
		}
		rec, err := runXfer(cfg, cmd, deps)
		if err != nil {
			return CmdOut{}, err
		}
		text, err := renderXfer(rec, cmd.audience)
		if err != nil {
			return CmdOut{}, err
		}
		return CmdOut{Text: text, Code: rec.Code}, nil
	default:
		return CmdOut{}, failUsage(deps, cmd.cfgPath, "")
	}
}

func runLs(cfg *FileCfg, cmd cliCmd, deps Deps) (CmdOut, error) {
	if !cmd.hasTask {
		text, err := RenderLs(cfg, cmd.audience)
		if err != nil {
			return CmdOut{}, err
		}
		return CmdOut{Text: text}, nil
	}
	inv := cliCmd{
		kind: "xfer", action: "check", audience: cmd.audience, task: cmd.task,
		hasPath: cmd.hasPath, path: cmd.path,
	}
	report, err := runInv(cfg, inv, deps, false)
	if err != nil {
		return CmdOut{}, err
	}
	limit := cmd.limit
	if limit == 0 {
		limit = 10
	}
	clipped := clipInv(report, limit)
	text, err := renderInv(clipped, cmd.audience, "ls")
	if err != nil {
		return CmdOut{}, err
	}
	return CmdOut{Text: text}, nil
}

func failUsage(deps Deps, cfgFlag, detail string) error {
	names := []string{}
	extra := detail
	path, err := cfgFile(cfgFlag)
	if err != nil {
		extra = joinDetail(extra, err.Error())
		return fail(2, "%s", usageText(names, extra))
	}
	cfg, err := Load(path, deps.Home)
	if err != nil {
		extra = joinDetail(extra, err.Error())
	} else {
		names = taskNames(cfg)
	}
	return fail(2, "%s", usageText(names, extra))
}

func joinDetail(extra, msg string) string {
	if extra == "" {
		return msg
	}
	return extra + "\n" + msg
}

func runXfer(cfg *FileCfg, cmd cliCmd, deps Deps) (RunRec, error) {
	started := isoTime(deps.Now())
	got := doXfer(cfg, cmd, deps)
	id, err := deps.ID()
	if err != nil {
		return RunRec{}, fail(1, "id: %s", err.Error())
	}
	taskName := cmd.task
	local := ""
	label := ""
	if got.plan != nil {
		taskName = got.plan.Task.Name
		local = got.plan.Local
		label = got.plan.Label
	}
	rec := RunRec{
		ID: id, Task: taskName, Action: cmd.action, StartedAt: started,
		FinishedAt: isoTime(deps.Now()), Code: got.code, Local: local, Remote: label,
		DryRun: cmd.dryRun, Args: redactArgs(got.args, label),
		Bytes: got.bytes, Checks: got.checks, Transfers: got.transfers,
	}
	if got.errText != "" {
		rec.Error = got.errText
	}
	if rec.Args == nil {
		rec.Args = []string{}
	}
	if err := appendRun(rec); err != nil {
		return RunRec{}, err
	}
	return rec, nil
}

type gotXfer struct {
	plan      *Plan
	code      int
	errText   string
	args      []string
	bytes     *int64
	checks    *int64
	transfers *int64
}

func doXfer(cfg *FileCfg, cmd cliCmd, deps Deps) gotXfer {
	in := planIn{
		cfg: cfg, action: cmd.action, task: cmd.task, given: cmd.path, hasGiven: cmd.hasPath,
		dryRun: cmd.dryRun, audience: cmd.audience, dest: cmd.dest, hasDest: cmd.hasDest,
		cwd: deps.Cwd, verbose: cmd.verbose,
	}
	plan, err := makePlan(in)
	if err != nil {
		return gotXfer{code: codeOf(err), errText: err.Error()}
	}
	if err := assertCreds(plan.Target); err != nil {
		return gotXfer{plan: plan, code: codeOf(err), errText: err.Error()}
	}
	empty, err := ensureEmptyConf()
	if err != nil {
		return gotXfer{plan: plan, code: codeOf(err), errText: err.Error()}
	}
	sec, err := obscureTarget(cfg.RcloneBin, empty, plan.Target, deps.Spawn)
	if err != nil {
		return gotXfer{plan: plan, code: codeOf(err), errText: err.Error()}
	}
	env := xferEnv{
		bin: cfg.RcloneBin, cfg: empty, plan: plan, sec: sec,
		spawn: deps.Spawn, sink: xferSink(plan, deps.Stderr),
	}
	out, err := runAttempts(env)
	if err != nil {
		msg := RedactText(err.Error(), secretBag(plan.Target, sec))
		return gotXfer{plan: plan, code: codeOf(err), errText: msg}
	}
	got := gotXfer{
		plan: plan, code: out.code, args: out.args,
		bytes: out.bytes, checks: out.checks, transfers: out.transfers,
	}
	if out.code != 0 {
		cleaned := RedactText(strings.TrimSpace(out.stderr), secretBag(plan.Target, sec))
		if cleaned == "" {
			cleaned = fmt.Sprintf("rclone exited %d", out.code)
		}
		got.errText = cleaned
	}
	return got
}

func runInv(cfg *FileCfg, cmd cliCmd, deps Deps, record bool) (*InvReport, error) {
	started := isoTime(deps.Now())
	got := doInv(cfg, cmd, deps)
	if record {
		id, err := deps.ID()
		if err != nil {
			return nil, fail(1, "id: %s", err.Error())
		}
		taskName := cmd.task
		local := ""
		label := ""
		if got.plan != nil {
			taskName = got.plan.Task.Name
			local = got.plan.Local
			label = got.plan.Label
		}
		rec := RunRec{
			ID: id, Task: taskName, Action: "check", StartedAt: started,
			FinishedAt: isoTime(deps.Now()), Code: got.code, Local: local, Remote: label,
			DryRun: false, Args: redactArgs(got.args, label),
		}
		if got.report != nil {
			n := int64(len(got.report.Rows))
			rec.Checks = &n
		}
		if got.errText != "" {
			rec.Error = got.errText
		}
		if rec.Args == nil {
			rec.Args = []string{}
		}
		if err := appendRun(rec); err != nil {
			return nil, err
		}
	}
	if got.report == nil {
		msg := got.errText
		if msg == "" {
			msg = "inventory failed"
		}
		return nil, &Err{Code: got.code, Msg: msg}
	}
	return got.report, nil
}

type gotInv struct {
	plan    *Plan
	report  *InvReport
	args    []string
	code    int
	errText string
}

func doInv(cfg *FileCfg, cmd cliCmd, deps Deps) gotInv {
	in := planIn{
		cfg: cfg, action: "check", task: cmd.task, given: cmd.path, hasGiven: cmd.hasPath,
		audience: cmd.audience, cwd: deps.Cwd,
	}
	plan, err := makePlan(in)
	if err != nil {
		return gotInv{code: codeOf(err), errText: err.Error()}
	}
	if err := assertCreds(plan.Target); err != nil {
		return gotInv{plan: plan, code: codeOf(err), errText: err.Error()}
	}
	empty, err := ensureEmptyConf()
	if err != nil {
		return gotInv{plan: plan, code: codeOf(err), errText: err.Error()}
	}
	sec, err := obscureTarget(cfg.RcloneBin, empty, plan.Target, deps.Spawn)
	if err != nil {
		return gotInv{plan: plan, code: codeOf(err), errText: err.Error()}
	}
	remote, err := BuildRemote(plan.Target, sec, plan.Rel, plan.NameEnc, "none", "")
	if err != nil {
		msg := RedactText(err.Error(), secretBag(plan.Target, sec))
		return gotInv{plan: plan, code: codeOf(err), errText: msg}
	}
	localArgs := buildListArgv(plan, empty, plan.Local, false)
	remoteArgs := buildListArgv(plan, empty, remote, false)
	report, err := loadInv(cfg.RcloneBin, localArgs, remoteArgs, deps.Spawn, plan.Task.Name, plan.Rel, usesLegacyBin(plan))
	if err != nil {
		msg := RedactText(err.Error(), secretBag(plan.Target, sec))
		return gotInv{plan: plan, args: remoteArgs, code: codeOf(err), errText: msg}
	}
	code := 0
	errText := ""
	if checkFailed(report) {
		code = 1
		errText = fmt.Sprintf("missing=%d differ=%d", report.Counts.Missing, report.Counts.Differ)
	}
	return gotInv{plan: plan, report: report, args: remoteArgs, code: code, errText: errText}
}

func xferSink(plan *Plan, stderr io.Writer) ErrSink {
	if plan.Verbose {
		return stderrSink(stderr)
	}
	if plan.Action == "get" || plan.Action == "download" {
		return noopSink()
	}
	if plan.Progress {
		return stderrSink(stderr)
	}
	return noopSink()
}

func assertCreds(target Target) error {
	if target.User == "" {
		return fail(1, "targets.%s.user is required", target.Name)
	}
	if target.Pass == "" {
		return fail(1, "targets.%s.pass is required", target.Name)
	}
	return nil
}

func secretBag(target Target, sec Secrets) []string {
	bag := []string{target.Pass, sec.Pass}
	for _, pair := range target.Crypt {
		bag = append(bag, pair.Key, pair.Salt)
	}
	for _, pair := range sec.Crypt {
		bag = append(bag, pair.Password, pair.Password2)
	}
	return bag
}

func codeOf(err error) int {
	var re *Err
	if errors.As(err, &re) && re.Code != 0 {
		return re.Code
	}
	return 1
}

func isoTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}
