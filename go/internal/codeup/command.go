package codeup

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Execute 跑一次命令并返回进程码。os.Exit 留在 main，RunE 只返回错误。
func Execute(args []string, stdout, stderr io.Writer) int {
	root := newRoot()
	root.SetOut(stdout)
	root.SetErr(stderr)
	if args == nil {
		args = []string{}
	}
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return 0
	}
	var exit *ExitErr
	if errors.As(err, &exit) {
		if !exit.Silent {
			fmt.Fprintf(stderr, "%s\n", exit.Msg)
		}
		if exit.Code == 0 {
			return 1
		}
		return exit.Code
	}
	fmt.Fprintf(stderr, "%s\n", err.Error())
	return 1
}

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:                "codeup-cli",
		Short:              "Codeup repositories, change requests, and webhooks",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArgs(cmd.OutOrStdout(), args)
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpCommand(newHelp())
	root.AddCommand(newComplete())
	return root
}

func newHelp() *cobra.Command {
	return &cobra.Command{
		Use:                "help [topic]",
		Short:              "show help",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := ""
			if len(args) > 0 && args[0] != "-h" && args[0] != "--help" {
				topic = args[0]
			}
			fmt.Fprint(cmd.OutOrStdout(), HelpText(topic))
			return nil
		},
	}
}

func newComplete() *cobra.Command {
	return &cobra.Command{
		Use:                "__complete",
		Hidden:             true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tokens := append([]string{}, args...)
			current := ""
			if len(tokens) > 0 {
				current = tokens[len(tokens)-1]
				tokens = tokens[:len(tokens)-1]
			}
			text, err := CompleteLines(tokens, current)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), text)
			return nil
		},
	}
}

func runArgs(stdout io.Writer, args []string) error {
	cmd, err := ParseArgs(args)
	if err != nil {
		return err
	}
	switch cmd.Kind {
	case kindHelp:
		fmt.Fprint(stdout, HelpText(cmd.Topic))
		return nil
	case kindVersion:
		fmt.Fprint(stdout, "codeup-cli 0.1.0\n")
		return nil
	case kindDoctor:
		return Doctor(stdout, cmd.Config)
	case kindFish:
		fmt.Fprint(stdout, FishScript())
		return nil
	case kindComplete:
		text, err := CompleteLines(cmd.Tokens, cmd.Current)
		if err != nil {
			return err
		}
		fmt.Fprint(stdout, text)
		return nil
	default:
		text, err := eval(cmd)
		if err != nil {
			return err
		}
		fmt.Fprint(stdout, text)
		return nil
	}
}

func eval(cmd Cmd) (string, error) {
	cfg, err := LoadFileCfg(cmd.Config)
	if err != nil {
		return "", err
	}
	rt, err := ResolveRuntime(cfg, cmd.Audience, cmd.Profile)
	if err != nil {
		return "", err
	}
	return Dispatch(cmd, rt, runEnv{Client: &Client{RT: rt}})
}
