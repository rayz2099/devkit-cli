package td

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
)

// Execute 跑一次 td-cli. 退出码交给 main, RunE 里不调用 os.Exit.
func Execute(args []string, stdout, stderr io.Writer) int {
	return executeAt(args, stdout, stderr, time.Now())
}

func executeAt(args []string, stdout, stderr io.Writer, now time.Time) int {
	// nil 会被 cobra 换成 os.Args, 测试进程的旗标就会被当成命令.
	if args == nil {
		args = []string{}
	}
	// __complete 必须在 cobra 之前接住. cobra 会把自己的补全协议装到同名隐藏命令上.
	if len(args) > 0 && args[0] == "__complete" {
		return finish(args, stdout, stderr, dispatch(args, stdout, stderr, now))
	}
	cmd := newRoot(stdout, stderr, now)
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	err := cmd.Execute()
	return finish(args, stdout, stderr, err)
}

func newRoot(stdout, stderr io.Writer, now time.Time) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "td-cli [-c config.json] [-p profile] [agent|human] <command>",
		Short: "只读查询 ThinkingData",
		// 语法和 TypeScript 一致: 受众词和旗标可以插在任意位置, cobra 的旗标解析对不上.
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(c *cobra.Command, args []string) error {
			return dispatch(args, c.OutOrStdout(), c.ErrOrStderr(), now)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd
}

func finish(args []string, stdout, stderr io.Writer, err error) int {
	if err == nil {
		return 0
	}
	if _, ok := err.(quietExit); ok {
		return 1
	}
	msg := err.Error()
	if audienceOf(args) == "agent" {
		_, _ = io.WriteString(stdout, errBody(msg))
		return 1
	}
	fmt.Fprintf(stderr, "%s\n", msg)
	return 1
}
