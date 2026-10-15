package main

import (
	"os"

	"git.internal.linran.top/linran/devkit-cli/internal/rclone"
)

func main() {
	os.Exit(rclone.Main(os.Args[1:]))
}
