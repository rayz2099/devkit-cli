package main

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"git.internal.linran.top/linran/devkit-cli/internal/olly"
)

func newUptrace() *cobra.Command {
	cmd := bare("uptrace")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return errors.New("unknown uptrace command: <missing>")
	}
	cmd.AddCommand(sectionHelp(olly.UptraceHelp))
	cmd.AddCommand(newGroups("groups"), newGroups("group-stats"), newSpans(), newTrace("trace"), newTrace("context"), newGroups("diagnose"))
	return cmd
}

func newGroups(name string) *cobra.Command {
	cmd := bare(name)
	addStr(cmd, "service", "env", "query", "search", "limit", "duration-gte", "duration-lt", "time-start", "time-end", "time-gte", "time-lt", "time-dur", "time-dur-seconds")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		mode, err := outputMode(c)
		if err != nil {
			return err
		}
		app, err := olly.Load(cfgPath(c))
		if err != nil {
			return err
		}
		cfg, err := olly.RequireUptrace(app)
		if err != nil {
			return err
		}
		in, err := groupsInput(c, cfg)
		if err != nil {
			return err
		}
		var spec olly.UpSpec
		if name == "group-stats" {
			spec, err = olly.BuildGroupStatsRequest(cfg, in)
		} else {
			spec, err = olly.BuildGroupsRequest(cfg, in)
		}
		if err != nil {
			return err
		}
		body, err := olly.UpGet(c.Context(), nil, spec)
		if err != nil {
			return err
		}
		var text string
		if name == "diagnose" && mode == "plain" {
			text, err = olly.FormatJSON(body)
		} else {
			text, err = olly.FormatGroups(body, mode)
		}
		if err != nil {
			return err
		}
		_, err = io.WriteString(c.OutOrStdout(), text)
		return err
	}
	return cmd
}

func newSpans() *cobra.Command {
	cmd := bare("spans")
	addStr(cmd, "trace-id", "span-id", "parent-id", "limit", "duration-gte", "duration-lt", "query", "sort-by", "page", "time-start", "time-end", "time-gte", "time-lt", "time-dur", "time-dur-seconds")
	addBool(cmd, "sort-desc")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if _, err := outputMode(c); err != nil {
			return err
		}
		app, err := olly.Load(cfgPath(c))
		if err != nil {
			return err
		}
		cfg, err := olly.RequireUptrace(app)
		if err != nil {
			return err
		}
		in, err := spansInput(c, cfg)
		if err != nil {
			return err
		}
		spec, err := olly.BuildSpansRequest(cfg, in)
		if err != nil {
			return err
		}
		body, err := olly.UpGet(c.Context(), nil, spec)
		if err != nil {
			return err
		}
		text, err := olly.FormatJSON(map[string]any{"spans": olly.ExtractSpans(body)})
		if err != nil {
			return err
		}
		_, err = io.WriteString(c.OutOrStdout(), text)
		return err
	}
	return cmd
}

func newTrace(name string) *cobra.Command {
	cmd := bare(name + " [trace]")
	cmd.Args = cobra.ArbitraryArgs
	addStr(cmd, "trace-id", "service", "env", "uri", "limit", "time-gte", "time-dur", "time-start", "time-end", "time-lt", "duration-gte", "duration-lt")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		mode, err := outputMode(c)
		if err != nil {
			return err
		}
		app, err := olly.Load(cfgPath(c))
		if err != nil {
			return err
		}
		cfg, err := olly.RequireUptrace(app)
		if err != nil {
			return err
		}
		input := ""
		if text := strPtr(c, "trace-id"); text != nil {
			input = *text
		} else if len(args) > 0 {
			input = args[0]
		}
		if input == "" {
			return errors.New("trace command requires trace id, trace URL, or --trace-id")
		}
		parsed, err := olly.ParseTraceInput(input)
		if err != nil {
			return err
		}
		reqCfg := olly.WithProject(cfg, parsed.ProjectID)
		spec, err := olly.BuildTraceRequest(reqCfg, parsed.TraceID)
		if err != nil {
			return err
		}
		body, err := olly.UpGet(c.Context(), nil, spec)
		if err != nil {
			return err
		}
		spans := spanMaps(olly.ExtractTraceSpans(body))
		if mode == "plain" {
			text, err := olly.FormatJSON(map[string]any{"spans": spans})
			if err != nil {
				return err
			}
			_, err = io.WriteString(c.OutOrStdout(), text)
			return err
		}
		query := olly.AgentQuery{}
		if text := strPtr(c, "service"); text != nil {
			query.Service = *text
		}
		if text := strPtr(c, "env"); text != nil {
			query.Env = *text
		} else if reqCfg.HasDefaultEnv {
			query.Env = reqCfg.DefaultEnv
		}
		if text := strPtr(c, "uri"); text != nil {
			query.URI = *text
		}
		ctx, err := olly.BuildContext(reqCfg, input, spans, query)
		if err != nil {
			return err
		}
		text := olly.FormatContextAgent(ctx)
		if mode == "human" && name == "trace" {
			text = olly.FormatContextHuman(ctx)
		}
		_, err = io.WriteString(c.OutOrStdout(), text)
		return err
	}
	return cmd
}

func groupsInput(cmd *cobra.Command, cfg olly.UptraceConfig) (olly.GroupsInput, error) {
	timeIn, err := timeInput(cmd)
	if err != nil {
		return olly.GroupsInput{}, err
	}
	limit, err := numPtr(cmd, "limit")
	if err != nil {
		return olly.GroupsInput{}, err
	}
	gte, err := numPtr(cmd, "duration-gte")
	if err != nil {
		return olly.GroupsInput{}, err
	}
	lt, err := numPtr(cmd, "duration-lt")
	if err != nil {
		return olly.GroupsInput{}, err
	}
	in := olly.GroupsInput{
		TimeInput:   timeIn,
		Service:     strPtr(cmd, "service"),
		Query:       strPtr(cmd, "query"),
		Search:      strPtr(cmd, "search"),
		Limit:       limit,
		DurationGte: gte,
		DurationLt:  lt,
	}
	if env := strPtr(cmd, "env"); env != nil {
		in.Env = env
	} else if cfg.HasDefaultEnv {
		in.Env = &cfg.DefaultEnv
	}
	return in, nil
}

func spansInput(cmd *cobra.Command, cfg olly.UptraceConfig) (olly.SpansInput, error) {
	timeIn, err := timeInput(cmd)
	if err != nil {
		return olly.SpansInput{}, err
	}
	nums := map[string]*float64{}
	for _, name := range []string{"span-id", "parent-id", "limit", "duration-gte", "duration-lt", "page"} {
		num, err := numPtr(cmd, name)
		if err != nil {
			return olly.SpansInput{}, err
		}
		nums[name] = num
	}
	desc, err := boolPtr(cmd, "sort-desc")
	if err != nil {
		return olly.SpansInput{}, err
	}
	_ = cfg
	return olly.SpansInput{
		TimeInput:   timeIn,
		TraceID:     strPtr(cmd, "trace-id"),
		SpanID:      nums["span-id"],
		ParentID:    nums["parent-id"],
		Limit:       nums["limit"],
		DurationGte: nums["duration-gte"],
		DurationLt:  nums["duration-lt"],
		Query:       strPtr(cmd, "query"),
		SortBy:      strPtr(cmd, "sort-by"),
		SortDesc:    desc,
		Page:        nums["page"],
	}, nil
}

func timeInput(cmd *cobra.Command) (olly.TimeInput, error) {
	dur, err := numPtr(cmd, "time-dur")
	if err != nil {
		return olly.TimeInput{}, err
	}
	if dur == nil {
		dur, err = numPtr(cmd, "time-dur-seconds")
		if err != nil {
			return olly.TimeInput{}, err
		}
	}
	return olly.TimeInput{
		TimeStart: strPtr(cmd, "time-start"),
		TimeEnd:   strPtr(cmd, "time-end"),
		TimeGte:   strPtr(cmd, "time-gte"),
		TimeLt:    strPtr(cmd, "time-lt"),
		TimeDur:   dur,
	}, nil
}

func spanMaps(list []any) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if rec, ok := item.(map[string]any); ok {
			out = append(out, rec)
		}
	}
	return out
}
