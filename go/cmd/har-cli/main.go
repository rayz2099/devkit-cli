package main

import (
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/har"
)

// main 只把退出码交回进程。
// 业务错误留在 Execute 的返回值里，RunE 不能 os.Exit，否则测试和调用方拿不到失败原因。
func main() {
	os.Exit(har.Execute(os.Args[1:], os.Stdout, os.Stderr))
}
