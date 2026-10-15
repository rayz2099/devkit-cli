// Package ghimage 是 gh-image-cli 的命令实现。
// 镜像台账在 Git 仓库根的 images.yaml，用户配置只覆盖仓库、命名空间和等待时间。
package ghimage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const usageText = `usage:
  gh-image-cli [-c config.json] add <source> <alias> [version]
  gh-image-cli [-c config.json] add-dockerfile <script> <alias> [version]
  gh-image-cli [-c config.json] build <alias> [version]
  gh-image-cli [-c config.json] list [alias]
  gh-image-cli completion fish`

// coded 携带和用法错误不同的退出码。msg 为空时不再往 stderr 补一行。
type coded struct {
	n   int
	msg string
}

func (e *coded) Error() string {
	if e.msg == "" {
		return fmt.Sprintf("exit %d", e.n)
	}
	return e.msg
}

type app struct {
	out io.Writer
	err io.Writer
	cfg *string
	rt  runtime
}

// Main 跑一次命令并返回退出码。version 不读配置；配置缺失只在 doctor 里算失败。
func Main(args []string, stdout, stderr io.Writer) int {
	return runCLI(args, stdout, stderr, defaultRT())
}

func runCLI(args []string, stdout, stderr io.Writer, rt runtime) int {
	cfg, rest, err := takeGlobals(args)
	if err != nil {
		fmt.Fprintf(stderr, "gh-image-cli: %s\n", err.Error())
		return 2
	}
	a := &app{out: stdout, err: stderr, cfg: cfg, rt: rt}
	// cobra 自己会占掉 __complete。补全协议必须在交给它之前处理，输出才是别名列表而不是指令尾缀。
	if len(rest) > 0 && rest[0] == "__complete" {
		a.complete(rest[1:])
		return 0
	}
	root := newRoot(a)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(rest)
	return finish(stderr, root.Execute())
}

func finish(stderr io.Writer, err error) int {
	if err == nil {
		return 0
	}
	var code *coded
	if errors.As(err, &code) {
		if code.msg != "" {
			fmt.Fprintf(stderr, "gh-image-cli: %s\n", code.msg)
		}
		return code.n
	}
	msg := err.Error()
	if name, ok := unknownName(msg); ok {
		msg = "unknown command: " + name + "\n" + usageText
	}
	fmt.Fprintf(stderr, "gh-image-cli: %s\n", msg)
	return 2
}

func unknownName(msg string) (string, bool) {
	const prefix = `unknown command "`
	if !strings.HasPrefix(msg, prefix) {
		return "", false
	}
	name, _, ok := strings.Cut(strings.TrimPrefix(msg, prefix), `"`)
	if !ok || name == "" {
		return "", false
	}
	return name, true
}

// takeGlobals 抽出 -c/--config。它只决定读哪一个文件，不和环境变量叠成另一层配置。
func takeGlobals(args []string) (*string, []string, error) {
	var cfg *string
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if tok != "-c" && tok != "--config" {
			rest = append(rest, tok)
			continue
		}
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
			return nil, nil, fmt.Errorf("%s requires a value", tok)
		}
		val := args[i+1]
		cfg = &val
		i++
	}
	return cfg, rest, nil
}

func newRoot(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:                "gh-image-cli",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,
		DisableSuggestions: true,
		CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.noCmd(args)
		},
	}
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(a.out, usageText)
	})
	root.SetHelpCommand(&cobra.Command{
		Use:                "help",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(a.out, usageText)
			return nil
		},
	})
	root.AddCommand(
		a.cmdAdd(),
		a.cmdAddFile(),
		a.cmdBuild(),
		a.cmdList(),
		a.cmdCompletion(),
		a.cmdVersion(),
		a.cmdDoctor(),
	)
	return root
}

func (a *app) noCmd(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Fprintln(a.out, usageText)
		return nil
	}
	return fmt.Errorf("unknown command: %s\n%s", args[0], usageText)
}

func isHelp(tok string) bool {
	return tok == "help" || tok == "-h" || tok == "--help"
}

func leaf(use string, run func([]string) error) *cobra.Command {
	return &cobra.Command{
		Use:                use,
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(args)
		},
	}
}

func (a *app) cmdAdd() *cobra.Command {
	return leaf("add <source> <alias> [version]", a.add)
}

func (a *app) cmdAddFile() *cobra.Command {
	return leaf("add-dockerfile <script> <alias> [version]", a.addDockerfile)
}

func (a *app) cmdBuild() *cobra.Command {
	return leaf("build <alias> [version]", a.build)
}

func (a *app) cmdList() *cobra.Command {
	return leaf("list [alias]", a.list)
}

func (a *app) cmdCompletion() *cobra.Command {
	return leaf("completion fish", a.completion)
}

func (a *app) cmdVersion() *cobra.Command {
	return leaf("version", func(args []string) error {
		if len(args) != 0 {
			return errors.New(usageText)
		}
		fmt.Fprintln(a.out, "gh-image-cli 0.1.0")
		return nil
	})
}

func (a *app) cmdDoctor() *cobra.Command {
	return leaf("doctor", func(args []string) error {
		if len(args) != 0 {
			return errors.New(usageText)
		}
		return a.doctor()
	})
}

func need(n, min, max int) error {
	if n < min || n > max {
		return errors.New(usageText)
	}
	return nil
}

func (a *app) add(args []string) error {
	if err := need(len(args), 2, 3); err != nil {
		return err
	}
	source, tag, hasTag, err := splitSource(args[0])
	if err != nil {
		return err
	}
	raw := ""
	hasRaw := false
	if len(args) == 3 {
		raw = args[2]
		hasRaw = true
	}
	if hasTag && hasRaw && tag != raw {
		return fmt.Errorf("source tag %s conflicts with version %s", tag, raw)
	}
	version := "latest"
	if hasRaw {
		version = raw
	} else if hasTag {
		version = tag
	}
	if err := validateVersion(version); err != nil {
		return err
	}
	local, err := resolveLocal(a.rt)
	if err != nil {
		return err
	}
	book, err := loadStore(local.imagesPath)
	if err != nil {
		return err
	}
	if err := book.addMirror(source, args[1], version); err != nil {
		return err
	}
	if err := book.save(); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "added: %s:%s <- %s:%s\n", args[1], version, source, version)
	return nil
}

func (a *app) addDockerfile(args []string) error {
	if err := need(len(args), 2, 3); err != nil {
		return err
	}
	script := args[0]
	alias := args[1]
	version := "latest"
	if len(args) == 3 {
		version = args[2]
	}
	local, err := resolveLocal(a.rt)
	if err != nil {
		return err
	}
	if _, err := os.Stat(local.root + "/" + script); err != nil {
		return fmt.Errorf("Dockerfile build script not found: %s", script)
	}
	book, err := loadStore(local.imagesPath)
	if err != nil {
		return err
	}
	if err := book.addDockerfile(script, alias, version); err != nil {
		return err
	}
	if err := book.save(); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "added: %s:%s <- %s\n", alias, version, script)
	return nil
}

func (a *app) list(args []string) error {
	if err := need(len(args), 0, 1); err != nil {
		return err
	}
	local, err := resolveLocal(a.rt)
	if err != nil {
		return err
	}
	book, err := loadStore(local.imagesPath)
	if err != nil {
		return err
	}
	filter := ""
	if len(args) == 1 {
		filter = args[0]
		if _, err := book.require(filter); err != nil {
			return err
		}
	}
	cfg, err := a.loadConfig(false)
	if err != nil {
		return err
	}
	names := book.aliases()
	if filter != "" {
		names = []string{filter}
	}
	fmt.Fprintf(a.out, "ALIAS\tTYPE\tSOURCE/SCRIPT\tTARGET\n")
	for _, alias := range names {
		img, err := book.require(alias)
		if err != nil {
			return err
		}
		label := img.script
		if img.kind == "mirror" {
			label = img.source
		}
		fmt.Fprintf(a.out, "%s\t%s\t%s\t%s\n", alias, img.kind, label, imageRef(cfg, alias, ""))
	}
	return nil
}

func (a *app) build(args []string) error {
	if err := need(len(args), 1, 2); err != nil {
		return err
	}
	alias := args[0]
	local, err := resolveRepo(a.rt)
	if err != nil {
		return err
	}
	book, err := loadStore(local.imagesPath)
	if err != nil {
		return err
	}
	img, err := book.require(alias)
	if err != nil {
		return err
	}
	vers, err := book.versions(alias)
	if err != nil {
		return err
	}
	requested := ""
	hasReq := false
	if len(args) == 2 {
		requested = args[1]
		hasReq = true
	}
	version := "latest"
	if hasReq {
		version = requested
	} else if len(vers) > 0 {
		version = vers[0]
	}
	if err := validateVersion(version); err != nil {
		return err
	}
	if _, ok := workflowName(img.kind); !ok {
		return fmt.Errorf("invalid image record: %s", alias)
	}
	move := hasReq || len(vers) == 0
	if err := book.setStatus(alias, version, "init", move); err != nil {
		return err
	}
	if err := book.save(); err != nil {
		return err
	}
	cfg, err := a.loadConfig(false)
	if err != nil {
		return err
	}
	labelKind := "script"
	label := img.script
	if img.kind == "mirror" {
		labelKind = "source"
		label = img.source
	}
	wf, _ := workflowName(img.kind)
	ref := imageRef(cfg, alias, version)
	fmt.Fprintf(a.out, "alias: %s\n%s: %s\nversion: %s\nworkflow: %s\ntarget: %s\n", alias, labelKind, label, version, wf, ref)
	in := buildIn{local: local, img: img, alias: alias, version: version, cfg: cfg}
	res, err := runBuild(a.rt, in, a.out, a.err)
	if err != nil {
		if uerr := writeStatus(local.imagesPath, alias, version, "failed"); uerr != nil {
			return uerr
		}
		return err
	}
	if err := writeStatus(local.imagesPath, alias, version, res.status); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "status: %s\nrun: %s\ntarget: %s\n", res.status, res.runURL, ref)
	switch res.status {
	case "done":
		return nil
	case "timeout":
		return &coded{n: 124}
	default:
		return &coded{n: 1}
	}
}

func writeStatus(path, alias, version, status string) error {
	book, err := loadStore(path)
	if err != nil {
		return err
	}
	if err := book.setStatus(alias, version, status, false); err != nil {
		return err
	}
	return book.save()
}

func (a *app) completion(args []string) error {
	if len(args) != 1 || args[0] != "fish" {
		return errors.New("only fish completion is supported")
	}
	a.printFish()
	return nil
}

func fmtPrintln(w io.Writer, text string) {
	fmt.Fprintln(w, text)
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}
