package cli

import (
	"fmt"
	"strings"
)

// SplitConfigFlag 把 -c/--config 从 argv 抽出, 才能在启动时换配置而不改 XDG 默认路径.
func SplitConfigFlag(args []string) (string, []string, error) {
	path := ""
	rest := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-c" || arg == "--config":
			if index+1 >= len(args) {
				return "", nil, fmt.Errorf("%s requires a value", arg)
			}
			path = args[index+1]
			index++
		case strings.HasPrefix(arg, "--config="):
			path = strings.TrimPrefix(arg, "--config=")
			if path == "" {
				return "", nil, fmt.Errorf("--config requires a value")
			}
		default:
			rest = append(rest, arg)
		}
	}
	return path, rest, nil
}
