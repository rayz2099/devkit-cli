package main

import (
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/ghimage"
)

// main 只负责把退出码交还给进程。
// RunE 里不能 os.Exit，否则 build 的 1/124 和用法错误的 2 无法分开测。
func main() {
	os.Exit(ghimage.Main(os.Args[1:], os.Stdout, os.Stderr))
}
