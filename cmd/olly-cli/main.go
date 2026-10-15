// Command olly-cli 查询 Uptrace、Prometheus、Graylog 和 Grafana。
// 入口只负责把错误打到 stderr，业务命令不能在 RunE 里 os.Exit。
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := execute(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
