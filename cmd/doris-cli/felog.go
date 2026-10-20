package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type feLogFlags struct {
	level   string
	limit   int
	keyword string
}

// newFELogCmd 创建拉取 FE 运行日志（fe.warn.log）的指令。
// 优先拉取 WARN 及以上级别的异常日志，避免拉取海量 INFO 引起 Agent 上下文超限。
func newFELogCmd() *cobra.Command {
	var flags feLogFlags

	cmd := &cobra.Command{
		Use:   "fe-log",
		Short: "Fetch and filter FE warning/error logs via HTTP API",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := buildClient()
			if err != nil {
				return fmt.Errorf("build client: %w", err)
			}
			defer cli.Close()

			report, err := cli.GetFELog(context.Background(), flags.level, flags.limit, flags.keyword)
			if err != nil {
				return fmt.Errorf("fetch fe log: %w", err)
			}

			return outputData(cmd.OutOrStdout(), report)
		},
	}

	cmd.Flags().StringVar(&flags.level, "level", "WARN", "Filter log level (WARN|ERROR|FATAL|ALL)")
	cmd.Flags().IntVar(&flags.limit, "limit", 100, "Maximum number of recent log lines to return")
	cmd.Flags().StringVar(&flags.keyword, "grep", "", "Filter logs containing keyword")

	return cmd
}
