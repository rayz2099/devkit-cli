package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCompletionCmd 生成针对不同 Shell 的自动补全脚本。
// 优先保障对 fish 和 bash 的完整动态补全支持（包括集群 profile 名字联想）。
func newCompletionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script for fish, bash, zsh, or powershell",
		Long: `To load completions:

Bash:
  $ source <(doris-cli completion bash)

Zsh:
  $ source <(doris-cli completion zsh)

Fish:
  $ doris-cli completion fish | source
  # 或持久化保存到 fish 配置目录：
  $ doris-cli completion fish > ~/.config/fish/completions/doris-cli.fish
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell type: %s", args[0])
			}
		},
	}

	return cmd
}
