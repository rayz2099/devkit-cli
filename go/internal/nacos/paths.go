package nacos

import (
	"fmt"
	"path/filepath"

	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

// runtimeFile 返回缓存或日志路径。
// 这两类文件以前写在配置目录里；现在只要会落到配置根下就直接拒绝，不去另找目录。
func runtimeFile(kind string) (string, error) {
	dirs, err := xdg.Resolve(appName)
	if err != nil {
		return "", err
	}
	root, err := xdg.ConfigRoot()
	if err != nil {
		return "", err
	}
	var path string
	switch kind {
	case "cache":
		path = filepath.Join(dirs.Cache, "config-list.json")
	case "log":
		path = filepath.Join(dirs.State, "log", "nacos-cli.log")
	default:
		return "", fmt.Errorf("unknown runtime path %s", kind)
	}
	if xdg.Inside(root, path) {
		return "", fmt.Errorf("%w: %s", xdg.ErrInsideConfig, path)
	}
	return path, nil
}
