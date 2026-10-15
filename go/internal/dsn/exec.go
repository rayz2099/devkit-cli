package dsn

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// IO 是一次执行的输入输出。TTY 只看调用方，测试不必真的占有终端。
type IO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
	TTY bool
}

// Execute 跑完一条命令并返回进程码。配置缺失和门禁拒绝走不同的码。
func Execute(args []string, streams IO) int {
	if len(args) > 0 && args[0] == "__complete" {
		parsed := parseComplete(args[1:])
		fmt.Fprint(streams.Out, completeLines(parsed.tokens, parsed.current))
		return 0
	}
	parsed, err := parseArgs(args)
	if err != nil {
		writeErr(streams.Err, err)
		return exitCode(err)
	}
	// -h 的 topic 不是 cobra 的帮助页。help 在进 cobra 之前处理，避免未知 topic 被当成子命令。
	if parsed.kind == "help" {
		err = execCmd(parsed, streams)
		if err != nil {
			writeErr(streams.Err, err)
			return exitCode(err)
		}
		return 0
	}
	root := NewCommand()
	root.SetArgs(args)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	bindRun(root, func() error {
		return execCmd(parsed, streams)
	})
	err = root.Execute()
	if err != nil {
		writeErr(streams.Err, err)
		return exitCode(err)
	}
	return 0
}

func bindRun(root *cobra.Command, run func() error) {
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.RunE = func(cmd *cobra.Command, args []string) error {
		return run()
	}
	for _, child := range root.Commands() {
		child.SilenceUsage = true
		child.SilenceErrors = true
		child.RunE = root.RunE
		for _, grand := range child.Commands() {
			grand.SilenceUsage = true
			grand.SilenceErrors = true
			grand.RunE = root.RunE
		}
	}
}

func writeErr(w io.Writer, err error) {
	if err == nil || err.Error() == "" {
		return
	}
	fmt.Fprintf(w, "%s\n", err.Error())
}

func execCmd(parsed cmd, io IO) error {
	switch parsed.kind {
	case "help":
		return execHelp(parsed, io)
	case "completion-fish":
		fmt.Fprint(io.Out, fishScript())
		return nil
	case "version":
		fmt.Fprint(io.Out, "dsn-cli 0.1.0\n")
		return nil
	case "complete":
		fmt.Fprint(io.Out, completeLines(parsed.tokens, parsed.current))
		return nil
	}
	path, err := ConfigPath(parsed.config)
	if err != nil {
		return exitf(1, "%s", err.Error())
	}
	switch parsed.kind {
	case "doctor":
		profile := ""
		if parsed.hasProf {
			profile = strings.TrimSpace(parsed.profile)
		}
		err := RunDoctor(io.Out, path, profile)
		if err != nil {
			return &Exit{Code: 1}
		}
		return nil
	case "ds":
		return execDs(parsed, path, io)
	}
	file, err := Load(path)
	if err != nil {
		return exitf(1, "%s", err.Error())
	}
	prof, err := Pick(file, parsed.profile)
	if err != nil {
		return err
	}
	if parsed.kind == "console" {
		return execConsole(parsed, prof, io)
	}
	if err := gateQuery(prof.Kind, prof.Access, parsed.stmt); err != nil {
		return err
	}
	limit := timeouts{connect: ms(parsed.connect), exec: ms(parsed.exec)}
	raw, err := runDriver(context.Background(), prof.Kind, prof.URL, parsed.stmt, limit)
	if err != nil {
		return err
	}
	if prof.Kind == "kafka" && isKafkaTopics(parsed.stmt) {
		names := make([]string, 0, len(raw.Rows))
		for _, row := range raw.Rows {
			names = append(names, fmt.Sprint(row["name"]))
		}
		if err := writeTopicCache(prof.Name, names); err != nil {
			return exitf(1, "%s", err.Error())
		}
	}
	rowLimit := 0
	if parsed.audience == audAgent {
		rowLimit = agentLimit(parsed.limit)
	}
	rows, truncated := capRows(raw.Rows, rowLimit)
	output := parsed.output
	if prof.Kind == "kafka" && parsed.audience == audHuman && isKafkaListen(parsed.stmt) && output == "table" {
		output = "json"
	}
	body := Render(string(parsed.audience), output, QueryOut{Columns: raw.Columns, Rows: rows}, truncated, parsed.pretty)
	fmt.Fprint(io.Out, body)
	return nil
}

func execHelp(parsed cmd, io IO) error {
	if parsed.topic != "ds" {
		fmt.Fprint(io.Out, renderText(helpText(parsed.topic)))
		return nil
	}
	path, err := ConfigPath(parsed.config)
	if err != nil {
		return exitf(1, "%s", err.Error())
	}
	ds := cmd{kind: "ds", audience: audHuman, output: "table", profile: parsed.profile, hasProf: parsed.hasProf, config: parsed.config}
	return execDs(ds, path, io)
}

func execDs(parsed cmd, path string, io IO) error {
	file, err := Load(path)
	if err != nil {
		return exitf(1, "%s", err.Error())
	}
	profs := file.Profiles
	if parsed.hasProf {
		one, err := Pick(file, parsed.profile)
		if err != nil {
			return err
		}
		profs = []Profile{one}
	}
	fmt.Fprint(io.Out, Render(string(parsed.audience), parsed.output, dsOut(profs), false, parsed.pretty))
	return nil
}

func dsOut(profs []Profile) QueryOut {
	rows := make([]map[string]any, len(profs))
	for i, item := range profs {
		rows[i] = map[string]any{
			"name":        item.Name,
			"kind":        item.Kind,
			"access":      item.Access,
			"description": item.Description,
		}
	}
	return QueryOut{
		Columns: []string{"name", "kind", "access", "description"},
		Rows:    rows,
	}
}

func execConsole(parsed cmd, prof Profile, io IO) error {
	if parsed.audience == audAgent {
		return exitf(1, "agent cannot enter Console")
	}
	if !io.TTY {
		return exitf(1, "non-TTY requires query")
	}
	return runConsole(prof.Kind, prof.URL)
}
