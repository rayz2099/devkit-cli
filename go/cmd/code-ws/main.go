package main

import (
	"errors"
	"fmt"
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/codews"
)

func main() {
	err := codews.Execute(os.Args[1:])
	if err == nil {
		return
	}
	var ex *codews.Exit
	if errors.As(err, &ex) {
		if ex.Msg != "" {
			fmt.Fprintf(os.Stderr, "code-ws: %s\n", ex.Msg)
		}
		os.Exit(ex.Code)
	}
	fmt.Fprintf(os.Stderr, "code-ws: %s\n", err.Error())
	os.Exit(1)
}
