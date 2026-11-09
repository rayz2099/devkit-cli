package main

import (
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/issue"
)

// main 不安装或生成配置，保证命令只执行用户明确请求的操作。
func main() {
	opts := issue.Options{Out: os.Stdout, Err: os.Stderr}
	code := issue.Execute(os.Args[1:], opts)
	os.Exit(code)
}
