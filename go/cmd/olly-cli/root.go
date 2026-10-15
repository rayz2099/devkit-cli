package main

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"git.internal.linran.top/linran/devkit-cli/internal/olly"
)

func execute(args []string, stdout, stderr io.Writer) error {
	root := newRoot()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	return root.Execute()
}

func newRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "olly-cli",
		Short:         "query uptrace, prometheus, graylog and grafana",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return errors.New("usage: olly-cli [-c config.json] [--output human|agent|plain] {uptrace|logs|prometheus|grafana} <command>. Run olly-cli --help for details.")
			}
			_, err := io.WriteString(cmd.OutOrStdout(), olly.RootHelp)
			return err
		},
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SetHelpCommand(&cobra.Command{
		Use:           "help",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := io.WriteString(cmd.OutOrStdout(), olly.RootHelp)
			return err
		},
	})
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		_, _ = io.WriteString(c.OutOrStdout(), helpText(c))
	})
	fs := cmd.PersistentFlags()
	fs.StringP("config", "c", "", "config file path")
	fs.StringP("file", "f", "", "config file path")
	fs.StringP("output", "o", "human", "output mode")
	_ = cmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"human", "agent", "plain"}, cobra.ShellCompDirectiveNoFileComp
	})
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if skipGuard(c) {
			return nil
		}
		return olly.Guard()
	}
	cmd.AddCommand(newVersion(), newDoctor(), newUptrace(), newLogs(), newPrometheus(), newGrafana())
	return cmd
}

func newVersion() *cobra.Command {
	return &cobra.Command{
		Use:           "version",
		Short:         "print version",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := io.WriteString(cmd.OutOrStdout(), "olly-cli 0.1.0\n")
			return err
		},
	}
}

func newDoctor() *cobra.Command {
	return &cobra.Command{
		Use:           "doctor",
		Short:         "check config",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return olly.Doctor(cfgPath(cmd), cmd.OutOrStdout())
		},
	}
}

func helpText(cmd *cobra.Command) string {
	cur := cmd
	for cur.Parent() != nil && cur.Parent().Name() != "olly-cli" {
		cur = cur.Parent()
	}
	name := cur.Name()
	switch name {
	case "version":
		return "olly-cli version\n\nPrint olly-cli 0.1.0.\n"
	case "doctor":
		return "olly-cli doctor\n\nCheck that the config file parses. No network.\n"
	default:
		return olly.HelpFor(name)
	}
}

func skipGuard(cmd *cobra.Command) bool {
	name := cmd.Name()
	return name == "version" || name == "help" || name == "olly-cli"
}

func cfgPath(cmd *cobra.Command) string {
	if text, ok := changed(cmd, "config"); ok {
		return text
	}
	if text, ok := changed(cmd, "file"); ok {
		return text
	}
	return ""
}

func outputMode(cmd *cobra.Command) (string, error) {
	text, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", err
	}
	if text != "human" && text != "agent" && text != "plain" {
		return "", errors.New("--output must be human, agent, or plain")
	}
	return text, nil
}

func changed(cmd *cobra.Command, name string) (string, bool) {
	for cur := cmd; cur != nil; cur = cur.Parent() {
		flag := cur.Flags().Lookup(name)
		if flag != nil && flag.Changed {
			return flag.Value.String(), true
		}
	}
	return "", false
}

func strPtr(cmd *cobra.Command, name string) *string {
	text, ok := changed(cmd, name)
	if !ok {
		return nil
	}
	return &text
}

func numPtr(cmd *cobra.Command, name string) (*float64, error) {
	text, ok := changed(cmd, name)
	if !ok {
		return nil, nil
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		zero := 0.0
		return &zero, nil
	}
	num, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil, flagNumErr(name)
	}
	return &num, nil
}

func boolPtr(cmd *cobra.Command, name string) (*bool, error) {
	text, ok := changed(cmd, name)
	if !ok {
		return nil, nil
	}
	switch text {
	case "true":
		val := true
		return &val, nil
	case "false":
		val := false
		return &val, nil
	default:
		return nil, flagBoolErr(name)
	}
}

func flagNumErr(name string) error {
	return errors.New("flag --" + camel(name) + " must be a number")
}

func flagBoolErr(name string) error {
	return errors.New("flag --" + camel(name) + " must be true or false")
}

func camel(name string) string {
	parts := strings.Split(name, "-")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

func addStr(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		cmd.Flags().String(name, "", "")
	}
}

func addBool(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		cmd.Flags().String(name, "", "")
		cmd.Flags().Lookup(name).NoOptDefVal = "true"
	}
}

func bare(use string) *cobra.Command {
	return &cobra.Command{
		Use:           use,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func sectionHelp(text string) *cobra.Command {
	return &cobra.Command{
		Use:           "help",
		Hidden:        true,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := io.WriteString(cmd.OutOrStdout(), text)
			return err
		},
	}
}

func matches(cmd *cobra.Command) []string {
	flag := cmd.Flags().Lookup("match")
	if flag == nil || !flag.Changed {
		return nil
	}
	arr, ok := flag.Value.(pflag.SliceValue)
	if !ok {
		return nil
	}
	return arr.GetSlice()
}
