package main

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"git.internal.linran.top/linran/devkit-cli/internal/olly"
)

func newPrometheus() *cobra.Command {
	cmd := bare("prometheus")
	cmd.Aliases = []string{"prom"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return errors.New("unknown prometheus command: <missing>")
	}
	cmd.PersistentFlags().String("base-url", "", "")
	cmd.AddCommand(sectionHelp(olly.PrometheusHelp))
	cmd.AddCommand(newPromQuery(), newPromStatus("ready"), newPromStatus("healthy"), newPromInfo("build-info"), newPromInfo("runtime-info"))
	return cmd
}

func newPromQuery() *cobra.Command {
	cmd := bare("query")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return errors.New("unknown prometheus command: query")
	}
	cmd.AddCommand(newInstant(), newRange(), newLabels(), newSeries())
	return cmd
}

func newInstant() *cobra.Command {
	cmd := bare("instant <promql>")
	cmd.Args = cobra.ArbitraryArgs
	addStr(cmd, "time", "timeout", "limit")
	addBool(cmd, "include-values")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("prometheus query requires promql")
		}
		mode, err := outputMode(c)
		if err != nil {
			return err
		}
		cfg, err := promCfg(c)
		if err != nil {
			return err
		}
		in := olly.PromQuery{Query: args[0], Time: strPtr(c, "time")}
		in.Timeout = strPtr(c, "timeout")
		in.Limit, err = numPtr(c, "limit")
		if err != nil {
			return err
		}
		spec, err := olly.BuildQueryRequest(cfg, in)
		if err != nil {
			return err
		}
		include, err := boolPtr(c, "include-values")
		if err != nil {
			return err
		}
		return writeProm(c, mode, spec, olly.PromFormatCtx{Command: "query instant", Query: in.Query, HasQuery: true, IncludeValues: include != nil && *include})
	}
	return cmd
}

func newRange() *cobra.Command {
	cmd := bare("range <promql>")
	cmd.Args = cobra.ArbitraryArgs
	addStr(cmd, "start", "end", "step", "timeout", "limit")
	addBool(cmd, "include-values")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("prometheus query requires promql")
		}
		mode, err := outputMode(c)
		if err != nil {
			return err
		}
		cfg, err := promCfg(c)
		if err != nil {
			return err
		}
		in := olly.PromRange{Query: args[0], Start: strPtr(c, "start"), End: strPtr(c, "end"), Step: strPtr(c, "step")}
		in.Timeout = strPtr(c, "timeout")
		in.Limit, err = numPtr(c, "limit")
		if err != nil {
			return err
		}
		spec, err := olly.BuildRangeRequest(cfg, in)
		if err != nil {
			return err
		}
		include, err := boolPtr(c, "include-values")
		if err != nil {
			return err
		}
		on := include != nil && *include
		return writeProm(c, mode, spec, olly.PromFormatCtx{Command: "query range", Query: in.Query, HasQuery: true, IncludeValues: on})
	}
	return cmd
}

func newLabels() *cobra.Command {
	cmd := bare("labels [name]")
	cmd.Args = cobra.ArbitraryArgs
	addStr(cmd, "start", "end", "limit")
	cmd.Flags().StringArray("match", nil, "")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		mode, err := outputMode(c)
		if err != nil {
			return err
		}
		cfg, err := promCfg(c)
		if err != nil {
			return err
		}
		meta, err := promMeta(c)
		if err != nil {
			return err
		}
		var spec olly.PromSpec
		if len(args) > 0 {
			spec, err = olly.BuildLabelValuesRequest(cfg, args[0], meta)
		} else {
			spec = olly.BuildLabelsRequest(cfg, meta)
		}
		if err != nil {
			return err
		}
		return writeProm(c, mode, spec, olly.PromFormatCtx{Command: "query labels"})
	}
	return cmd
}

func newSeries() *cobra.Command {
	cmd := bare("series")
	addStr(cmd, "start", "end", "limit")
	cmd.Flags().StringArray("match", nil, "")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		mode, err := outputMode(c)
		if err != nil {
			return err
		}
		cfg, err := promCfg(c)
		if err != nil {
			return err
		}
		meta, err := promMeta(c)
		if err != nil {
			return err
		}
		spec, err := olly.BuildSeriesRequest(cfg, meta)
		if err != nil {
			return err
		}
		return writeProm(c, mode, spec, olly.PromFormatCtx{Command: "query series"})
	}
	return cmd
}

func newPromStatus(name string) *cobra.Command {
	cmd := bare(name)
	cmd.RunE = func(c *cobra.Command, args []string) error {
		cfg, err := promCfg(c)
		if err != nil {
			return err
		}
		spec := olly.BuildReadyRequest(cfg)
		if name == "healthy" {
			spec = olly.BuildHealthyRequest(cfg)
		}
		text, err := olly.PromText(c.Context(), nil, spec)
		if err != nil {
			return err
		}
		_, err = io.WriteString(c.OutOrStdout(), text)
		return err
	}
	return cmd
}

func newPromInfo(name string) *cobra.Command {
	cmd := bare(name)
	cmd.RunE = func(c *cobra.Command, args []string) error {
		mode, err := outputMode(c)
		if err != nil {
			return err
		}
		cfg, err := promCfg(c)
		if err != nil {
			return err
		}
		spec := olly.BuildBuildInfoRequest(cfg)
		if name == "runtime-info" {
			spec = olly.BuildRuntimeInfoRequest(cfg)
		}
		return writeProm(c, mode, spec, olly.PromFormatCtx{Command: name})
	}
	return cmd
}

func promCfg(cmd *cobra.Command) (olly.PrometheusConfig, error) {
	explicit := cfgPath(cmd) != ""
	app, found, err := olly.LoadOptional(cfgPath(cmd), explicit)
	if err != nil {
		return olly.PrometheusConfig{}, err
	}
	cfg := olly.PrometheusConfig{BaseURL: olly.DefaultPrometheusBase}
	if found {
		cfg = app.Prometheus
	}
	if text, ok := changed(cmd, "base-url"); ok {
		cfg.BaseURL = text
	}
	return cfg, nil
}

func promMeta(cmd *cobra.Command) (olly.PromMeta, error) {
	limit, err := numPtr(cmd, "limit")
	if err != nil {
		return olly.PromMeta{}, err
	}
	return olly.PromMeta{
		Matches: matches(cmd),
		Start:   strPtr(cmd, "start"),
		End:     strPtr(cmd, "end"),
		Limit:   limit,
	}, nil
}

func writeProm(cmd *cobra.Command, mode string, spec olly.PromSpec, ctx olly.PromFormatCtx) error {
	status, raw, err := olly.PromDo(cmd.Context(), nil, spec)
	if err != nil {
		return err
	}
	if mode == "plain" {
		text, err := olly.PromPlain(status, raw)
		if err != nil {
			return err
		}
		_, err = io.WriteString(cmd.OutOrStdout(), text)
		return err
	}
	data, err := olly.ReadPromData(status, raw)
	if err != nil {
		return err
	}
	text, err := olly.FormatPromData(data, mode, ctx)
	if err != nil {
		return err
	}
	_, err = io.WriteString(cmd.OutOrStdout(), text)
	return err
}
