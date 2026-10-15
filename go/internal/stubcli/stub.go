// Package stubcli 是模块命令被替换前的占位入口。
// 只保证 help、version、doctor 有退出行为，业务命令一律失败。模块 agent 必须替换掉对它的调用。
package stubcli

import (
	"fmt"
	"os"
)

// Run 按第一个参数给出占位行为。version 不读配置。doctor 在没有实现时视为配置检查失败。
func Run(name string) {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Printf("usage: %s <command>\n", name)
		return
	}
	if args[0] == "version" {
		fmt.Printf("%s 0.1.0\n", name)
		return
	}
	if args[0] == "doctor" {
		fmt.Println("fail config: config missing")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "%s: not implemented\n", name)
	os.Exit(1)
}
