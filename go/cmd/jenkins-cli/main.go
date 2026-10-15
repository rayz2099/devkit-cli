package main

import (
	"fmt"
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/jenkins"
)

func main() {
	err := jenkins.Run(os.Args[1:], os.Stdout, os.Stderr)
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s\n", err.Error())
	os.Exit(jenkins.ExitCode(err))
}
