package main

import (
	"fmt"
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/nacos"
)

func main() {
	err := nacos.Execute(os.Args[1:], os.Stdout, os.Stderr)
	if err == nil {
		return
	}
	if !nacos.Quiet(err) {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
	}
	os.Exit(1)
}
