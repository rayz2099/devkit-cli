// Command doris-cli 提供针对 Apache Doris 集群的只读诊断工具。
// 专为 AI Agent 和排障脚本提供结构化 JSON 输出，涵盖集群拓扑、FE 错误日志与核心指标。
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
