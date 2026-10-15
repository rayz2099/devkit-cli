package main

import (
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/dsn"
)

func main() {
	os.Exit(dsn.Execute(os.Args[1:], dsn.IO{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
		TTY: isTTY(),
	}))
}

// isTTY 同时看 stdin 和 stdout，和官方客户端的交互前提一致。
func isTTY() bool {
	inSt, err := os.Stdin.Stat()
	if err != nil || inSt.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	outSt, err := os.Stdout.Stat()
	if err != nil || outSt.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return true
}
