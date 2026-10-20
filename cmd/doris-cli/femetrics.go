package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// newFEMetricsCmd 创建抓取 FE Prometheus 指标的指令。
// 提炼 JVM 堆占比、Full GC 次数以及 EditLog 延迟等对 FE 稳定性至关重要的指标。
func newFEMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fe-metrics",
		Short: "Fetch and summarize FE JVM heap, GC, and transaction metrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := buildClient()
			if err != nil {
				return fmt.Errorf("build client: %w", err)
			}
			defer cli.Close()

			report, err := cli.GetFEMetrics(context.Background())
			if err != nil {
				return fmt.Errorf("fetch fe metrics: %w", err)
			}

			return outputData(cmd.OutOrStdout(), report)
		},
	}
	return cmd
}
