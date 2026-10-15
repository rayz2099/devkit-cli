package nacos

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type app struct {
	out io.Writer
}

// Execute 跑一次 nacos-cli。错误返回给调用方，RunE 里面不退出进程。
func Execute(args []string, stdout, stderr io.Writer) error {
	a := &app{out: stdout}
	root := a.root()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(rewriteNS(args))
	return root.Execute()
}

// rewriteNS 把 -ns 收成 --namespace。
// cobra 会把 -ns 拆成两个短旗标，namespace 又必须能写在子命令前面。
func rewriteNS(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)
	if len(out) > 0 && (out[0] == "__complete" || out[0] == "__completeNoDesc") {
		return out
	}
	for i, arg := range out {
		if arg == "-ns" {
			out[i] = "--namespace"
		}
	}
	return out
}

func (a *app) root() *cobra.Command {
	root := newCmd("nacos-cli", a.runRoot)
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.DisableSuggestions = true
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpFunc(a.helpFunc)
	// --dev 不带值。不声明的话，cobra 找子命令时会把下一个词吃掉。
	root.PersistentFlags().Bool("dev", false, "write HTTP debug log")

	config := newCmd("config", a.runConfig)
	config.AddCommand(
		newCmd("get", a.runGet),
		newCmd("put", a.runPut),
		newCmd("delete", a.runDelete),
		newCmd("list", a.runList),
	)
	naming := newCmd("naming", a.runNaming)
	naming.AddCommand(
		newCmd("register", a.runRegister),
		newCmd("deregister", a.runDeregister),
		newCmd("instances", a.runInstances),
	)
	completion := newCmd("completion", a.runCompletion)
	completion.AddCommand(newCmd("fish", a.runFishScript))

	complete := newCmd("__complete", a.runComplete)
	complete.Hidden = true
	complete.Aliases = []string{"__completeNoDesc"}
	fish := newCmd("__fish_complete", a.runFishHidden)
	fish.Hidden = true

	root.AddCommand(config, naming, completion, newCmd("version", a.runVersion), newCmd("doctor", a.runDoctor), complete, fish)
	root.SetHelpCommand(newCmd("help", func([]string) error {
		return a.write(helpText(""))
	}))
	return root
}

func newCmd(use string, run func([]string) error) *cobra.Command {
	return &cobra.Command{
		Use:                use,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			return run(args)
		},
	}
}

func (a *app) helpFunc(cmd *cobra.Command, _ []string) {
	topic := ""
	for cur := cmd; cur != nil; cur = cur.Parent() {
		if cur.Name() == "config" {
			topic = "config"
			break
		}
		if cur.Name() == "naming" {
			topic = "naming"
			break
		}
	}
	_, _ = io.WriteString(a.out, helpText(topic))
}

func (a *app) runRoot(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		return a.write(helpText(""))
	}
	_, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 || rest[0] == "help" || rest[0] == "-h" || rest[0] == "--help" {
		return a.write(helpText(""))
	}
	return fmt.Errorf("unknown command: %s", rest[0])
}

func (a *app) runConfig(args []string) error {
	_, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 || rest[0] == "help" || rest[0] == "-h" || rest[0] == "--help" || hasHelp(args) {
		return a.write(helpText("config"))
	}
	return fmt.Errorf("unknown config command: %s", rest[0])
}

func (a *app) runNaming(args []string) error {
	_, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 || rest[0] == "help" || rest[0] == "-h" || rest[0] == "--help" || hasHelp(args) {
		return a.write(helpText("naming"))
	}
	return fmt.Errorf("unknown naming command: %s", rest[0])
}

func (a *app) runCompletion(args []string) error {
	_, rest, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 || rest[0] != "fish" {
		return errors.New("usage: nacos-cli completion fish")
	}
	return a.write(fishScript())
}

func (a *app) runFishScript(args []string) error {
	if _, _, err := splitGlobal(args); err != nil {
		return err
	}
	return a.write(fishScript())
}

func (a *app) runVersion(args []string) error {
	if _, _, err := splitGlobal(args); err != nil {
		return err
	}
	if hasHelp(args) {
		return a.write(helpText(""))
	}
	return a.write("nacos-cli 0.1.0\n")
}

func (a *app) runDoctor(args []string) error {
	g, _, err := splitGlobal(args)
	if err != nil {
		return err
	}
	if hasHelp(args) {
		return a.write(helpText(""))
	}
	return runDoctor(g.configValue(), a.out)
}
