package main

import (
	"github.com/spf13/cobra"

	"git.internal.linran.top/linran/devkit-cli/internal/doris"
)

// newDoctorCmd 创建健康体检与配置自检命令。
// 对齐 devkit-cli 的 doctor 标准契约，输出 pass/fail 列表。
func newDoctorCmd() *cobra.Command {
	var ping bool

	cmd := &cobra.Command{
		Use:           "doctor",
		Short:         "Check configuration and optional network connectivity to Doris FE",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return doris.RunDoctor(cmd.OutOrStdout(), gFlags.configPath, gFlags.profile, ping)
		},
	}

	cmd.Flags().BoolVar(&ping, "ping", false, "Test network connectivity to Doris FE MySQL and HTTP ports")

	return cmd
}
