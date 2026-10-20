package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// newClusterCmd 创建 cluster 拓扑诊断指令。
// 帮助 Agent 快速感知 Master FE 分布、挂掉的节点以及分片损坏情况。
func newClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Inspect Doris cluster topology, FE/BE nodes, and tablet health",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := buildClient()
			if err != nil {
				return fmt.Errorf("build client: %w", err)
			}
			defer cli.Close()

			report, err := cli.GetCluster(context.Background())
			if err != nil {
				return fmt.Errorf("inspect cluster: %w", err)
			}

			return outputData(cmd.OutOrStdout(), report)
		},
	}
	return cmd
}
