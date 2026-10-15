package jenkins

import (
	"io"

	"github.com/spf13/cobra"
)

const versionLine = "jenkins-cli 0.1.0\n"

// NewRoot 是 cobra 入口。关掉用法和错误前缀，退出码和 stderr 文案才能跟旧 CLI 对齐，而不是再被 cobra 打印一遍。
// 关掉默认分词，是因为 Audience 前缀和 flag 位置要自己解析。
func NewRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "jenkins-cli",
		Short:         "profile-based Jenkins client",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return dispatch(cmd.OutOrStdout(), args)
		},
	}
	cmd.DisableFlagParsing = true
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.DisableSuggestions = true
	return cmd
}

// Run 执行一次 CLI。__complete 在 cobra 之前拦住，避免它的隐藏补全协议把候选和退出指令写出去。
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && (args[0] == "__complete" || args[0] == "__completeNoDesc") {
		toks, cur := splitComplete(args[1:])
		_, err := io.WriteString(stdout, completeLines(toks, cur))
		return err
	}
	// cobra 把 nil 参数当成「去读 os.Args」，空调用和测试都会误吃进程参数。
	if args == nil {
		args = []string{}
	}
	cmd := NewRoot()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	return cmd.Execute()
}
