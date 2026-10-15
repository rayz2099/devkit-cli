package har

import (
	"fmt"
	"io"
	"math"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
	"github.com/spf13/cobra"
)

// Execute 跑一次 har-cli 并返回退出码。
// help 比子命令优先，和原来的单层解析一致；错误只写 stderr，不在 RunE 里退出进程。
func Execute(argv []string, stdout, stderr io.Writer) int {
	if isHelp(argv) {
		_, _ = io.WriteString(stdout, Help())
		return 0
	}
	root := newRoot()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(argv)
	if err := root.Execute(); err != nil {
		msg := err.Error()
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		_, _ = io.WriteString(stderr, msg)
		return 1
	}
	return 0
}

func isHelp(argv []string) bool {
	if len(argv) == 0 || argv[0] == "help" {
		return true
	}
	for _, arg := range argv {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "har-cli",
		SilenceUsage:  true,
		SilenceErrors: true,
		// 根命令有子命令时，cobra 会把文件路径当成未知子命令。显式收任意参数，HAR 才能和 version 并存。
		Args: cobra.ArbitraryArgs,
		// 扫描参数是原来的手写语法，等号形式和未知 flag 都不能交给 cobra。
		DisableFlagParsing: true,
		CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := runScan(args)
			if err != nil {
				return err
			}
			_, err = io.WriteString(cmd.OutOrStdout(), out)
			return err
		},
	}
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		_, _ = io.WriteString(cmd.OutOrStdout(), Help())
	})
	root.SetHelpCommand(&cobra.Command{
		Use:                "help",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := io.WriteString(cmd.OutOrStdout(), Help())
			return err
		},
	})
	root.AddCommand(newCompletions(), newVersion(), newDoctor())
	return root
}

func newCompletions() *cobra.Command {
	return &cobra.Command{
		Use:                "completions",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := ""
			if len(args) > 0 {
				shell = args[0]
			}
			if shell != "fish" {
				return fmt.Errorf("Only fish completion is supported.")
			}
			_, err := io.WriteString(cmd.OutOrStdout(), Fish()+"\n")
			return err
		},
	}
}

func newVersion() *cobra.Command {
	return &cobra.Command{
		Use:                "version",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("version does not take arguments")
			}
			// 版本号固定，不读配置，避免为了打版本去碰用户目录。
			_, err := io.WriteString(cmd.OutOrStdout(), "har-cli 0.1.0\n")
			return err
		},
	}
}

func newDoctor() *cobra.Command {
	return &cobra.Command{
		Use:                "doctor",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("doctor does not take arguments")
			}
			// 原命令没有配置文件。这里不编造配置检查，缺文件不能当成失败。
			return doctor.Write(cmd.OutOrStdout(), nil)
		},
	}
}

type scanOpt struct {
	files  []string
	filter *string
	sel    string
	depth  int
	format string
	dedupe bool
}

func runScan(argv []string) (string, error) {
	opt, err := parseScan(argv)
	if err != nil {
		return "", err
	}
	entries, err := readFiles(opt.files)
	if err != nil {
		return "", err
	}
	if opt.filter != nil {
		matched := make([]entry, 0, len(entries))
		for _, item := range entries {
			ok, err := Eval(*opt.filter, filterCtx(item))
			if err != nil {
				return "", err
			}
			if ok {
				matched = append(matched, item)
			}
		}
		entries = matched
	}
	return Format(entries, opt.sel, opt.format, opt.dedupe, opt.depth), nil
}

func filterCtx(item entry) jVal {
	m := newMap()
	if item.body.k != kindUndef {
		m.put("body", item.body)
	}
	http := newMap()
	http.put("method", jStr(item.method))
	http.put("status", jNum(item.status))
	m.put("http", jObj(http))
	req := newMap()
	req.put("path", jStr(item.path))
	req.put("url", jStr(item.url))
	m.put("request", jObj(req))
	resp := newMap()
	if item.mime != nil {
		resp.put("mimeType", jStr(*item.mime))
	}
	m.put("response", jObj(resp))
	uri := newMap()
	uri.put("path", jStr(item.path))
	uri.put("url", jStr(item.url))
	m.put("uri", jObj(uri))
	return jObj(m)
}

func parseScan(argv []string) (scanOpt, error) {
	opt := scanOpt{sel: "http", depth: 2, format: "text", dedupe: true}
	for i := 0; i < len(argv); i++ {
		cur := argv[i]
		if !strings.HasPrefix(cur, "-") {
			opt.files = append(opt.files, cur)
			continue
		}
		switch cur {
		case "--filter", "-f":
			i++
			val, err := requireVal(argv, i, cur)
			if err != nil {
				return scanOpt{}, err
			}
			opt.filter = &val
		case "--select", "-s":
			i++
			val, err := requireVal(argv, i, cur)
			if err != nil {
				return scanOpt{}, err
			}
			opt.sel = val
		case "--prefix-depth", "-p":
			i++
			val, err := requireVal(argv, i, cur)
			if err != nil {
				return scanOpt{}, err
			}
			depth, err := parseDepth(val, cur)
			if err != nil {
				return scanOpt{}, err
			}
			opt.depth = depth
		case "--format", "-o":
			i++
			val, err := requireVal(argv, i, cur)
			if err != nil {
				return scanOpt{}, err
			}
			mode, err := parseFormat(val)
			if err != nil {
				return scanOpt{}, err
			}
			opt.format = mode
		case "--no-dedupe":
			opt.dedupe = false
		default:
			return scanOpt{}, fmt.Errorf("Unknown option: %s", cur)
		}
	}
	if len(opt.files) == 0 {
		return scanOpt{}, fmt.Errorf("At least one HAR file is required.")
	}
	return opt, nil
}

func requireVal(argv []string, i int, opt string) (string, error) {
	if i >= len(argv) || strings.HasPrefix(argv[i], "-") && argv[i] != "" {
		return "", fmt.Errorf("Missing value for %s", opt)
	}
	return argv[i], nil
}

func parseFormat(val string) (string, error) {
	if val == "text" || val == "json" {
		return val, nil
	}
	return "", fmt.Errorf("Unsupported format: %s", val)
}

func parseDepth(val, opt string) (int, error) {
	n := jsNumber(val)
	if math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) || n < 1 || n > float64(math.MaxInt) {
		return 0, fmt.Errorf("Invalid value for %s: %s", opt, val)
	}
	return int(n), nil
}
