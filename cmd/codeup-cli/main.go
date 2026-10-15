package main

import (
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/codeup"
)

// main 只负责进程码。RunE 不能 os.Exit，否则库测试无法接住退出码。
func main() {
	os.Exit(codeup.Execute(os.Args[1:], os.Stdout, os.Stderr))
}
