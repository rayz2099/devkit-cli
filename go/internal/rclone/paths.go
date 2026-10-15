package rclone

import (
	"fmt"
	"os"
	"path/filepath"

	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

const appName = "rclone-cli"

// cfgFile 是配置文件路径。-c 只选择文件，不覆盖文件里的字段；没给就用 XDG 配置根。
func cfgFile(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	dirs, err := xdg.Resolve(appName)
	if err != nil {
		return "", fail(1, "%s", err.Error())
	}
	return filepath.Join(dirs.Config, "config.json"), nil
}

// statePath 把运行时文件放进 XDG state。落进配置根就拒绝，避免 empty.conf 或记录写进 ~/.config。
func statePath(name string) (string, error) {
	dirs, err := xdg.Resolve(appName)
	if err != nil {
		return "", fail(1, "%s", err.Error())
	}
	root, err := xdg.ConfigRoot()
	if err != nil {
		return "", fail(1, "%s", err.Error())
	}
	path := filepath.Join(dirs.State, name)
	if xdg.Inside(root, path) {
		return "", fail(1, "%s", fmt.Errorf("%w: %s", xdg.ErrInsideConfig, path).Error())
	}
	return path, nil
}

func ensureEmptyConf() (string, error) {
	path, err := statePath("empty.conf")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fail(1, "state dir: %s", err.Error())
	}
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		return "", fail(1, "empty config: %s", err.Error())
	}
	// 空 rclone 配置在 state 里，每次重写并收紧权限。不能对用户的 config.json 做同样的事。
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fail(1, "empty config: %s", err.Error())
	}
	return path, nil
}

func runsPath() (string, error) {
	return statePath(filepath.Join("runs", "runs.jsonl"))
}
