package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type queryFlags struct {
	slowOnly bool
	minMs    int64
}

// newQueryCmd 创建查看当前活跃查询与慢查询的指令。
// 帮助 Agent 排查是否因长事务锁表或超大查询霸占 BE 资源导致的集群抖动。
func newQueryCmd() *cobra.Command {
	var flags queryFlags

	cmd := &cobra.Command{
		Use:   "query",
		Short: "List active or slow queries running in Doris",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := buildClient()
			if err != nil {
				return fmt.Errorf("build client: %w", err)
			}
			defer cli.Close()

			report, err := cli.GetQueries(context.Background(), flags.slowOnly, flags.minMs)
			if err != nil {
				return fmt.Errorf("query running tasks: %w", err)
			}

			return outputData(cmd.OutOrStdout(), report)
		},
	}

	cmd.Flags().BoolVar(&flags.slowOnly, "slow", false, "Only display slow queries exceeding threshold")
	cmd.Flags().Int64Var(&flags.minMs, "min-ms", 3000, "Minimum execution time threshold in milliseconds for slow queries")

	return cmd
}
