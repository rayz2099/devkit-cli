package main

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"git.internal.linran.top/linran/devkit-cli/internal/olly"
)

func newLogs() *cobra.Command {
	cmd := bare("logs [query]")
	cmd.Aliases = []string{"graylog"}
	cmd.Args = cobra.ArbitraryArgs
	addLogFlags(cmd)
	cmd.RunE = func(c *cobra.Command, args []string) error {
		return runLogs(c, args, false)
	}
	agg := bare("aggregate [query]")
	agg.Aliases = []string{"agg"}
	agg.Args = cobra.ArbitraryArgs
	addLogFlags(agg)
	agg.RunE = func(c *cobra.Command, args []string) error {
		return runLogs(c, args, true)
	}
	cmd.AddCommand(sectionHelp(olly.GraylogHelp), agg)
	return cmd
}

func addLogFlags(cmd *cobra.Command) {
	addStr(cmd, "query", "relative", "from", "to", "keyword", "limit", "offset", "sort", "fields", "filter", "group-by", "field")
	addBool(cmd, "decorate", "show-fields")
}

func runLogs(cmd *cobra.Command, args []string, agg bool) error {
	mode, err := outputMode(cmd)
	if err != nil {
		return err
	}
	app, err := olly.Load(cfgPath(cmd))
	if err != nil {
		return err
	}
	cfg, err := olly.RequireGraylog(app)
	if err != nil {
		return err
	}
	src, err := graySource(cmd, args, agg)
	if err != nil {
		return err
	}
	in, err := olly.ToGrayInput(src)
	if err != nil {
		if err.Error() == "graylog logs requires query text or Graylog search URL" {
			if agg {
				return errors.New("logs aggregate requires query text, Graylog search URL, or --query")
			}
			return errors.New("logs command requires query text, Graylog search URL, or --query")
		}
		return err
	}
	if agg {
		field := strPtr(cmd, "field")
		if field == nil || *field == "" {
			return errors.New("logs aggregate requires --field")
		}
		body, err := olly.GrayPostJSON(cmd.Context(), nil, cfg, olly.BuildAggRequest(cfg, olly.GrayAgg{
			Query:     in.Query,
			RangeType: in.RangeType,
			Field:     *field,
			Relative:  in.Relative,
			From:      in.From,
			To:        in.To,
			Keyword:   in.Keyword,
			Limit:     in.Limit,
		}))
		if err != nil {
			return err
		}
		text, err := olly.FormatGrayAgg(olly.ReadAgg(body, olly.GrayAgg{Query: in.Query, Field: *field}), mode)
		if err != nil {
			return err
		}
		_, err = io.WriteString(cmd.OutOrStdout(), text)
		return err
	}
	spec := olly.BuildSearchRequest(cfg, in)
	body, err := olly.GrayGet(cmd.Context(), nil, cfg, spec)
	if err != nil {
		return err
	}
	ctx := olly.GrayCtx{
		Query:     in.Query,
		RangeType: in.RangeType,
		Relative:  in.Relative,
		From:      in.From,
		To:        in.To,
		Keyword:   in.Keyword,
		GroupBy:   strPtr(cmd, "group-by"),
	}
	show, err := boolPtr(cmd, "show-fields")
	if err != nil {
		return err
	}
	var text string
	if show != nil && *show {
		text, err = olly.FormatGrayFields(body, mode, ctx)
	} else {
		text, err = olly.FormatGrayResult(body, mode, ctx)
	}
	if err != nil {
		return err
	}
	_, err = io.WriteString(cmd.OutOrStdout(), text)
	return err
}

func graySource(cmd *cobra.Command, args []string, agg bool) (olly.GraySource, error) {
	rel, err := numPtr(cmd, "relative")
	if err != nil {
		return olly.GraySource{}, err
	}
	limit, err := numPtr(cmd, "limit")
	if err != nil {
		return olly.GraySource{}, err
	}
	offset, err := numPtr(cmd, "offset")
	if err != nil {
		return olly.GraySource{}, err
	}
	decorate, err := boolPtr(cmd, "decorate")
	if err != nil {
		return olly.GraySource{}, err
	}
	var source *string
	if text := strPtr(cmd, "query"); text != nil {
		source = text
	} else if !agg && len(args) > 0 {
		source = &args[0]
	} else if agg && len(args) > 0 {
		source = &args[0]
	}
	return olly.GraySource{
		Source:   source,
		Relative: rel,
		From:     strPtr(cmd, "from"),
		To:       strPtr(cmd, "to"),
		Keyword:  strPtr(cmd, "keyword"),
		Limit:    limit,
		Sort:     strPtr(cmd, "sort"),
		Fields:   strPtr(cmd, "fields"),
		Filter:   strPtr(cmd, "filter"),
		Offset:   offset,
		Decorate: decorate,
	}, nil
}
