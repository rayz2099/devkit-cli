// Command grafana-cli 按 profile 读取或更新 Grafana 看板。
// 入口只负责把错误打到 stderr，业务命令不能在 RunE 里 os.Exit。
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
