package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"git.internal.linran.top/linran/devkit-cli/internal/grafana"
)

func execute(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	aud, rest := peelAudience(args)
	root := newRoot(aud, stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(rest)
	return root.Execute()
}

func newRoot(aud string, stdin io.Reader) *cobra.Command {
	var cfgPath string
	var profile string
	cmd := &cobra.Command{
		Use:           "grafana-cli",
		Short:         "read and update Grafana dashboards",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "config file path")
	cmd.PersistentFlags().StringVarP(&profile, "profile", "P", "", "profile name")
	cmd.AddCommand(newVersion(), newDoctor(&cfgPath, &profile), newProfiles(aud, &cfgPath, &profile), newEdit(aud, stdin, &cfgPath, &profile))
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
			_, err := io.WriteString(cmd.OutOrStdout(), "grafana-cli 0.1.0\n")
			return err
		},
	}
}

func newDoctor(cfgPath, profile *string) *cobra.Command {
	return &cobra.Command{
		Use:           "doctor",
		Short:         "check local config",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return grafana.Doctor(*cfgPath, *profile, cmd.OutOrStdout())
		},
	}
}

func newProfiles(aud string, cfgPath, profile *string) *cobra.Command {
	return &cobra.Command{
		Use:           "profiles",
		Short:         "list configured Grafana profiles",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := grafana.LoadFile(*cfgPath)
			if err != nil {
				return err
			}
			active, _, err := file.Pick(*profile)
			if err != nil {
				return err
			}
			report := grafana.Report(file, active)
			if aud == "agent" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				return enc.Encode(report)
			}
			_, err = io.WriteString(cmd.OutOrStdout(), humanProfiles(report))
			return err
		},
	}
}

func newEdit(aud string, stdin io.Reader, cfgPath, profile *string) *cobra.Command {
	var file string
	var overwrite bool
	cmd := &cobra.Command{
		Use:           "edit <dashboard>",
		Short:         "show or update one dashboard",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			_, cfg, err := grafana.Load(*cfgPath, *profile)
			if err != nil {
				return err
			}
			body, write, err := readBody(stdin, file)
			if err != nil {
				return err
			}
			if overwrite && !write {
				return errors.New("--overwrite requires dashboard json")
			}
			text, err := grafana.Edit(c.Context(), nil, cfg, grafana.Input{
				Aud:       aud,
				Source:    args[0],
				Body:      body,
				Write:     write,
				Overwrite: overwrite,
			})
			if err != nil {
				return err
			}
			_, err = io.WriteString(c.OutOrStdout(), text)
			return err
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "dashboard json file, or - for stdin")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "update an existing dashboard uid")
	return cmd
}

func humanProfiles(report grafana.ProfilesReport) string {
	var buf strings.Builder
	buf.WriteString("NAME\tBASE_URL\tWEB\tUSER\tDEFAULT\tACTIVE\n")
	for _, item := range report.Profiles {
		def := "no"
		act := "no"
		if item.IsDefault {
			def = "yes"
		}
		if item.IsActive {
			act = "yes"
		}
		buf.WriteString(item.Name)
		buf.WriteByte('\t')
		buf.WriteString(item.BaseURL)
		buf.WriteByte('\t')
		buf.WriteString(item.WebBase)
		buf.WriteByte('\t')
		buf.WriteString(item.Username)
		buf.WriteByte('\t')
		buf.WriteString(def)
		buf.WriteByte('\t')
		buf.WriteString(act)
		buf.WriteByte('\n')
	}
	return buf.String()
}

// readBody 区分查看和写入。终端上的 edit 只读；--file 或管道才携带看板 JSON。
func readBody(in io.Reader, file string) ([]byte, bool, error) {
	if file != "" {
		if file == "-" {
			if in == nil {
				return nil, true, errors.New("dashboard json is empty")
			}
			buf, err := io.ReadAll(in)
			return buf, true, err
		}
		buf, err := os.ReadFile(file)
		return buf, true, err
	}
	if !piped(in) {
		return nil, false, nil
	}
	buf, err := io.ReadAll(in)
	return buf, true, err
}

func piped(in io.Reader) bool {
	if in == nil {
		return false
	}
	f, ok := in.(*os.File)
	if !ok {
		return true
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice == 0
}

// peelAudience 吃掉命令前的 agent/human。省略时是人，Agent 必须显式写前缀。
func peelAudience(args []string) (string, []string) {
	aud := "human"
	out := make([]string, 0, len(args))
	skip := false
	seen := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if skip {
			out = append(out, arg)
			skip = false
			continue
		}
		if arg == "--" {
			out = append(out, args[i:]...)
			break
		}
		if strings.HasPrefix(arg, "-") {
			out = append(out, arg)
			if takesValue(arg) && !strings.Contains(arg, "=") {
				skip = true
			}
			continue
		}
		if !seen && (arg == "agent" || arg == "human") {
			aud = arg
			continue
		}
		seen = true
		out = append(out, arg)
	}
	return aud, out
}

func takesValue(arg string) bool {
	switch arg {
	case "-c", "--config", "-P", "--profile", "--file":
		return true
	default:
		return false
	}
}
