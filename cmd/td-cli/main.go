package main

import (
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/td"
)

func main() {
	os.Exit(td.Execute(os.Args[1:], os.Stdout, os.Stderr))
}
