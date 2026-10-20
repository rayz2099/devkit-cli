package main

import (
	"github.com/spf13/cobra"

	"git.internal.linran.top/linran/devkit-cli/internal/doris"
)

type profileItem struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	QueryPort int    `json:"query_port"`
	HTTPPort  int    `json:"http_port"`
	User      string `json:"user"`
	IsDefault bool   `json:"is_default"`
	IsActive  bool   `json:"is_active"`
}

type profilesListReport struct {
	DefaultProfile string        `json:"default_profile"`
	ActiveProfile  string        `json:"active_profile"`
	Profiles       []profileItem `json:"profiles"`
}

// newProfilesCmd 创建列出所有集群配置的环境切换指令。
// 帮助 Agent 和开发者清晰感知多环境配置（如生产 super_doris 与本地 local）。
func newProfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "List all configured Doris cluster profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			fc, err := doris.LoadFileConfig(gFlags.configPath)
			if err != nil {
				return err
			}

			_, activeName, _ := doris.LoadConfig(gFlags.configPath, gFlags.profile)

			report := profilesListReport{
				DefaultProfile: fc.DefaultProfile,
				ActiveProfile:  activeName,
			}

			for name, p := range fc.Profiles {
				report.Profiles = append(report.Profiles, profileItem{
					Name:      name,
					Host:      p.Host,
					QueryPort: p.QueryPort,
					HTTPPort:  p.HTTPPort,
					User:      p.User,
					IsDefault: name == fc.DefaultProfile,
					IsActive:  name == activeName,
				})
			}

			return outputData(cmd.OutOrStdout(), report)
		},
	}
	return cmd
}
