// Command devkit-cli 是仓库根控制面。
// 只做 list、clean、build、install。不汇总 doctor，也不写配置、不装 fish 补全。
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "devkit-cli: %s\n", err.Error())
		os.Exit(1)
	}
}
