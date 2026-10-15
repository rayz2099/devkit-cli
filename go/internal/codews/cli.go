package codews

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const helpText = `code-ws

Usage:
  code-ws init <branch> -t <profile> [-p|--project <repo>] [--from <repo=branch>] [-v|--verbose] [-c|--config <path>]
  code-ws init <branch> -p|--project <repo> [--from <repo=branch>] [-v|--verbose] [-c|--config <path>]
  code-ws init <branch> <project> [--from <repo=branch>] [-v|--verbose] [-c|--config <path>]
  code-ws add project <repo> [-b|--branch <branch>] [-v|--verbose] [-c|--config <path>]
  code-ws remove project <repo> [-v|--verbose] [-c|--config <path>]
  code-ws sync master [--from <repo=branch>] [-v|--verbose] [-c|--config <path>]
  code-ws destroy [-v|--verbose] [-c|--config <path>]
  code-ws fork <branch> [-v|--verbose] [-c|--config <path>]
  code-ws list [-c|--config <path>]
  code-ws projects [-t <profile> | --workspace] [-c|--config <path>]
  code-ws branches <repo> [-c|--config <path>]
  code-ws config check [-c|--config <path>]
  code-ws serve [path] [--lan|--local] [--port <n>] [--no-watch] [--cf|--cloudflare]  # default LAN + watch + port 7001, next if busy
  code-ws completion fish
`

// Parsed 是一次无副作用的参数结果。测试可以直接比字段，不必启动命令。
type Parsed struct {
	Cmd       string
	Branch    string
	Profile   string
	HasProf   bool
	Project   string
	HasProj   bool
	From      []string
	Workspace bool
	Config    string
	Verbose   bool
	Repo      string
	HasBranch bool
	Path      string
	HasPath   bool
	Lan       bool
	Watch     bool
	Port      int
	HasPort   bool
	Cf        bool
}

type optSet struct {
	rest      []string
	profile   string
	hasProf   bool
	project   string
	hasProj   bool
	branch    string
	hasBranch bool
	from      []string
	workspace bool
	config    string
	help      bool
	verbose   bool
	lan       bool
	watch     bool
	port      int
	hasPort   bool
	cf        bool
}

func valueAfter(args []string, idx int, flag string) (string, error) {
	if idx+1 >= len(args) || strings.HasPrefix(args[idx+1], "-") {
		return "", fmt.Errorf("%s requires a value", flag)
	}
	return args[idx+1], nil
}

func parsePort(raw string) (int, error) {
	if raw == "" {
		return 0, errors.New("--port requires an integer")
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(raw, 64)
		if ferr != nil || f != float64(int64(f)) {
			return 0, errors.New("--port requires an integer")
		}
		return int(f), nil
	}
	return int(n), nil
}

func parseOpts(args []string) (optSet, error) {
	opt := optSet{lan: true, watch: true}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-t", "--template":
			val, err := valueAfter(args, i, arg)
			if err != nil {
				return optSet{}, err
			}
			opt.profile = val
			opt.hasProf = true
			i++
		case "-p", "--project":
			if opt.hasProj {
				return optSet{}, errors.New("--project may only be specified once; use add project for additional repos")
			}
			val, err := valueAfter(args, i, arg)
			if err != nil {
				return optSet{}, err
			}
			opt.project = val
			opt.hasProj = true
			i++
		case "-b", "--branch":
			val, err := valueAfter(args, i, arg)
			if err != nil {
				return optSet{}, err
			}
			opt.branch = val
			opt.hasBranch = true
			i++
		case "--from":
			val, err := valueAfter(args, i, arg)
			if err != nil {
				return optSet{}, err
			}
			opt.from = append(opt.from, val)
			i++
		case "--workspace":
			opt.workspace = true
		case "-c", "--config":
			val, err := valueAfter(args, i, arg)
			if err != nil {
				return optSet{}, err
			}
			opt.config = val
			i++
		case "--port":
			raw, err := valueAfter(args, i, arg)
			if err != nil {
				return optSet{}, err
			}
			n, err := parsePort(raw)
			if err != nil {
				return optSet{}, err
			}
			opt.port = n
			opt.hasPort = true
			i++
		case "--lan":
			opt.lan = true
		case "--local":
			opt.lan = false
		case "--no-watch":
			opt.watch = false
		case "--cf", "--cloudflare":
			opt.cf = true
		case "-h", "--help":
			opt.help = true
		case "-v", "--verbose":
			opt.verbose = true
		default:
			opt.rest = append(opt.rest, arg)
		}
	}
	return opt, nil
}

// ParseArgs 对齐 TypeScript 的参数规则，包括旗标可以出现在子命令前后。
func ParseArgs(args []string) (Parsed, error) {
	opt, err := parseOpts(args)
	if err != nil {
		return Parsed{}, err
	}
	cmd, sub, tail := "", "", []string{}
	if len(opt.rest) > 0 {
		cmd = opt.rest[0]
	}
	if len(opt.rest) > 1 {
		sub = opt.rest[1]
	}
	if len(opt.rest) > 2 {
		tail = opt.rest[2:]
	}
	if opt.help {
		return Parsed{Cmd: "help"}, nil
	}
	fromOK := cmd == "init" || (cmd == "sync" && sub == "master")
	if len(opt.from) > 0 && !fromOK {
		return Parsed{}, errors.New("--from is only supported by init and sync master")
	}
	if opt.hasProj && cmd != "init" {
		return Parsed{}, errors.New("--project is only supported by init")
	}
	if cmd == "init" {
		return parseInit(opt, sub, tail)
	}
	if cmd == "projects" {
		if sub != "" {
			return Parsed{}, errors.New("usage: code-ws projects [-t <profile> | --workspace]")
		}
		if opt.hasProf && opt.workspace {
			return Parsed{}, errors.New("usage: code-ws projects [-t <profile> | --workspace]")
		}
		out := Parsed{Cmd: "projects", Config: opt.config, Workspace: opt.workspace}
		if opt.hasProf {
			out.HasProf = true
			out.Profile = opt.profile
		}
		return out, nil
	}
	if cmd == "branches" {
		if sub == "" || len(tail) > 0 {
			return Parsed{}, errors.New("usage: code-ws branches <repo>")
		}
		return Parsed{Cmd: "branches", Repo: sub, Config: opt.config}, nil
	}
	if cmd == "completion" && sub == "fish" && len(tail) == 0 {
		return Parsed{Cmd: "completion-fish"}, nil
	}
	if cmd == "list" {
		if sub != "" {
			return Parsed{}, errors.New("usage: code-ws list")
		}
		return Parsed{Cmd: "list", Config: opt.config}, nil
	}
	if cmd == "add" && sub == "project" {
		if len(tail) != 1 {
			return Parsed{}, errors.New("usage: code-ws add project <repo> [-b|--branch <branch>] [-v|--verbose] [-c|--config <path>]")
		}
		out := Parsed{Cmd: "add-project", Repo: tail[0], Config: opt.config, Verbose: opt.verbose}
		if opt.hasBranch {
			out.Branch = opt.branch
			out.HasBranch = true
		}
		return out, nil
	}
	if cmd == "remove" && sub == "project" {
		if len(tail) != 1 {
			return Parsed{}, errors.New("usage: code-ws remove project <repo> [-v|--verbose] [-c|--config <path>]")
		}
		return Parsed{Cmd: "remove-project", Repo: tail[0], Config: opt.config, Verbose: opt.verbose}, nil
	}
	if cmd == "sync" && sub == "master" {
		if len(tail) > 0 {
			return Parsed{}, errors.New("usage: code-ws sync master [-v|--verbose] [-c|--config <path>]")
		}
		return Parsed{Cmd: "sync-master", From: opt.from, Config: opt.config, Verbose: opt.verbose}, nil
	}
	if cmd == "destroy" {
		if sub != "" {
			return Parsed{}, errors.New("usage: code-ws destroy [-v|--verbose] [-c|--config <path>]")
		}
		return Parsed{Cmd: "destroy-workspace", Config: opt.config, Verbose: opt.verbose}, nil
	}
	if cmd == "fork" {
		if sub == "" || len(tail) > 0 {
			return Parsed{}, errors.New("usage: code-ws fork <branch> [-v|--verbose] [-c|--config <path>]")
		}
		return Parsed{Cmd: "fork-workspace", Branch: sub, Config: opt.config, Verbose: opt.verbose}, nil
	}
	if cmd == "serve" {
		if len(tail) > 0 {
			return Parsed{}, errors.New("usage: code-ws serve [path] [--lan|--local] [--port <n>] [--no-watch] [--cf|--cloudflare]")
		}
		out := Parsed{
			Cmd:     "serve",
			Lan:     opt.lan,
			Watch:   opt.watch,
			Port:    opt.port,
			HasPort: opt.hasPort,
			Cf:      opt.cf,
		}
		if sub != "" {
			out.Path = sub
			out.HasPath = true
		}
		return out, nil
	}
	if cmd == "config" && sub == "check" && len(tail) == 0 {
		return Parsed{Cmd: "config-check", Config: opt.config}, nil
	}
	if cmd == "doctor" && sub == "" {
		return Parsed{Cmd: "doctor", Config: opt.config}, nil
	}
	if cmd == "version" && sub == "" {
		return Parsed{Cmd: "version"}, nil
	}
	if cmd == "" || cmd == "help" || cmd == "--help" || cmd == "-h" {
		return Parsed{Cmd: "help"}, nil
	}
	return Parsed{}, fmt.Errorf("unknown command: %s", strings.Join(opt.rest, " "))
}

func parseInit(opt optSet, sub string, tail []string) (Parsed, error) {
	usage := "usage: code-ws init <branch> <project> or code-ws init <branch> [-t <profile>] [--project <project>]"
	if sub == "" || len(tail) > 1 {
		return Parsed{}, errors.New(usage)
	}
	var repo string
	if len(tail) == 1 {
		repo = tail[0]
	}
	if repo != "" && (opt.hasProf || opt.hasProj) {
		return Parsed{}, errors.New(usage)
	}
	out := Parsed{Cmd: "init", Branch: sub, Config: opt.config, Verbose: opt.verbose, From: opt.from}
	if opt.hasProf {
		out.HasProf = true
		out.Profile = opt.profile
		if opt.hasProj {
			out.HasProj = true
			out.Project = opt.project
		}
		return out, nil
	}
	project := repo
	if project == "" && opt.hasProj {
		project = opt.project
	}
	if project == "" {
		return Parsed{}, errors.New("project or profile is required: <project> or -t <profile>")
	}
	out.HasProj = true
	out.Project = project
	return out, nil
}

// Execute 是进程入口。错误交给 main 打印，RunE 里不退出。
func Execute(args []string) error {
	return Run(os.Stdin, os.Stdout, os.Stderr, args)
}

// Run 用 cobra 接参数，但旗标规则仍按 TypeScript：先扫完全部参数再派发。
func Run(in io.Reader, out, errw io.Writer, args []string) error {
	cmd := &cobra.Command{
		Use:           "code-ws",
		Short:         "code workspace tools",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return dispatch(in, out, errw, args)
		},
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.DisableFlagParsing = true
	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		fmt.Fprint(c.OutOrStdout(), helpText+"\n")
	})
	cmd.SetIn(in)
	cmd.SetOut(out)
	cmd.SetErr(errw)
	if args == nil {
		args = []string{}
	}
	cmd.SetArgs(args)
	return cmd.Execute()
}

func dispatch(in io.Reader, out, errw io.Writer, args []string) error {
	parsed, err := ParseArgs(args)
	if err != nil {
		return err
	}
	switch parsed.Cmd {
	case "help":
		fmt.Fprint(out, helpText+"\n")
		return nil
	case "version":
		fmt.Fprintln(out, "code-ws 0.1.0")
		return nil
	case "completion-fish":
		fmt.Fprintln(out, FishCompletion())
		return nil
	case "doctor":
		return runDoctor(out, parsed.Config)
	case "init":
		return runInit(in, out, parsed)
	case "list":
		return runList(out, parsed)
	case "projects":
		return runProjects(out, parsed)
	case "branches":
		return runBranches(out, parsed)
	case "add-project":
		return runAdd(out, parsed)
	case "remove-project":
		return runRemove(in, out, parsed)
	case "sync-master":
		return runSync(out, errw, parsed)
	case "destroy-workspace":
		return runDestroy(out, parsed)
	case "fork-workspace":
		return runFork(in, out, parsed)
	case "serve":
		return runServe(out, errw, parsed)
	case "config-check":
		return runCheck(out, parsed)
	default:
		return fmt.Errorf("unknown command: %s", parsed.Cmd)
	}
}

func cfgOf(flag string) (Cfg, error) {
	path, err := ConfigPath(flag)
	if err != nil {
		return Cfg{}, err
	}
	return LoadConfig(path)
}
