package main

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"git.internal.linran.top/linran/devkit-cli/internal/olly"
)

func newGrafana() *cobra.Command {
	cmd := bare("grafana [dashboard]")
	cmd.Args = cobra.ArbitraryArgs
	addGrafFlags(cmd)
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("grafana requires dashboard url or uid")
		}
		return runGrafana(c, args[0])
	}
	analyze := bare("analyze [dashboard]")
	analyze.Args = cobra.ArbitraryArgs
	addGrafFlags(analyze)
	analyze.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("grafana analyze requires dashboard url or uid")
		}
		return runGrafana(c, args[0])
	}
	cmd.AddCommand(sectionHelp(olly.GrafanaHelp), analyze)
	return cmd
}

func addGrafFlags(cmd *cobra.Command) {
	addStr(cmd, "from", "to", "org-id")
}

func runGrafana(cmd *cobra.Command, source string) error {
	mode, err := outputMode(cmd)
	if err != nil {
		return err
	}
	app, err := olly.Load(cfgPath(cmd))
	if err != nil {
		return err
	}
	cfg, err := olly.RequireGrafana(app)
	if err != nil {
		return err
	}
	org, err := numPtr(cmd, "org-id")
	if err != nil {
		return err
	}
	var orgID *int
	if org != nil {
		if *org != float64(int(*org)) {
			return errors.New("flag --orgId must be a number")
		}
		id := int(*org)
		orgID = &id
	}
	web := cfg.BaseURL
	if cfg.HasWeb {
		web = cfg.WebBaseURL
	}
	result, err := olly.AnalyzeDashboard(cmd.Context(), nil, cfg, olly.AnalyzeInput{
		Source:     source,
		WebBaseURL: web,
		From:       strPtr(cmd, "from"),
		To:         strPtr(cmd, "to"),
		OrgID:      orgID,
	})
	if err != nil {
		return err
	}
	text, err := olly.FormatGrafana(result, mode)
	if err != nil {
		return err
	}
	_, err = io.WriteString(cmd.OutOrStdout(), text)
	return err
}
