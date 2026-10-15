package rclone

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// Main 是进程入口。退出码由返回值决定，RunE 里不能 os.Exit。
func Main(args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	deps := Deps{
		Home:   os.Getenv("HOME"),
		Spawn:  SpawnProc,
		Now:    time.Now,
		ID:     newID,
		Cwd:    cwd,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return Execute(args, deps)
}

// Execute 跑一轮命令。version 不读配置；其它命令的配置只来自文件，不用环境变量盖字段。
func Execute(args []string, deps Deps) int {
	if args == nil {
		args = []string{}
	}
	if len(args) > 0 && args[0] == "__complete" {
		// 最后一个词是正在输入的片段，前面的才是已确定的 token。cobra 自己的 __complete 协议不同，不能交给它。
		cmd := parseComplete(args[1:])
		text := completeLines(cmd.tokens, cmd.current, deps.Home, "")
		writeOut(deps.Stdout, text)
		return 0
	}
	root := newRoot(deps)
	root.SetArgs(args)
	root.SetOut(deps.Stdout)
	root.SetErr(deps.Stderr)
	err := root.Execute()
	return finish(err, deps.Stderr)
}

func newRoot(deps Deps) *cobra.Command {
	// 标志可以出现在子命令前面。交给 cobra 的话，布尔标志会把下一个词当成值吃掉。
	root := &cobra.Command{
		Use:                "rclone-cli",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,
		CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			deps.Stdout = cmd.OutOrStdout()
			deps.Stderr = cmd.ErrOrStderr()
			out, err := runCmd(args, deps)
			if err != nil {
				return err
			}
			writeOut(deps.Stdout, out.Text)
			if out.Code != 0 {
				return &Err{Code: out.Code}
			}
			return nil
		},
	}
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		writeOut(cmd.OutOrStdout(), helpText())
	})
	return root
}

func finish(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	var re *Err
	if errors.As(err, &re) {
		if re.Msg != "" {
			fmt.Fprintln(stderr, re.Msg)
		}
		if re.Code == 0 {
			return 1
		}
		return re.Code
	}
	fmt.Fprintln(stderr, err.Error())
	return 1
}

func writeOut(w io.Writer, text string) {
	if w == nil || text == "" {
		return
	}
	if len(text) > 0 && text[len(text)-1] == '\n' {
		_, _ = io.WriteString(w, text)
		return
	}
	_, _ = io.WriteString(w, text+"\n")
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(raw[:])
	return hexed[0:8] + "-" + hexed[8:12] + "-" + hexed[12:16] + "-" + hexed[16:20] + "-" + hexed[20:], nil
}
