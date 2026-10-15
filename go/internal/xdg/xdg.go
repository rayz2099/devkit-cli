// Package xdg 解析配置、缓存和状态目录。
// 配置目录只读：调用方读完即止。缓存和状态若落到配置目录里，直接拒绝，避免验收或运行改到用户配置。
package xdg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Dirs 是一个应用的三类 XDG 目录，都已经拼上应用名。
type Dirs struct {
	Config string
	Cache  string
	State  string
}

// ErrInsideConfig 表示缓存或状态路径落进了配置目录。
var ErrInsideConfig = errors.New("refusing path inside config dir")

// ConfigRoot 返回 XDG 配置根，不含应用名。安装和运行都用它判断「能不能写」。
func ConfigRoot() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config")), nil
}

// Resolve 返回应用的配置、缓存、状态目录。
// 缓存和状态必须落在配置根之外，否则后续写入会改到 ~/.config。
func Resolve(app string) (Dirs, error) {
	if app == "" {
		return Dirs{}, errors.New("app name is empty")
	}
	home, err := homeDir()
	if err != nil {
		return Dirs{}, err
	}
	cfgRoot, err := ConfigRoot()
	if err != nil {
		return Dirs{}, err
	}
	cacheRoot := envOr("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	stateRoot := envOr("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	dirs := Dirs{
		Config: filepath.Join(cfgRoot, app),
		Cache:  filepath.Join(cacheRoot, app),
		State:  filepath.Join(stateRoot, app),
	}
	if err := outsideConfig(cfgRoot, dirs.Cache); err != nil {
		return Dirs{}, err
	}
	if err := outsideConfig(cfgRoot, dirs.State); err != nil {
		return Dirs{}, err
	}
	return dirs, nil
}

// Inside 判断 child 是否等于 parent，或在 parent 里面。
func Inside(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, "..")
}

func outsideConfig(cfgRoot, path string) error {
	if Inside(cfgRoot, path) {
		return fmt.Errorf("%w: %s", ErrInsideConfig, path)
	}
	return nil
}

func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home: %w", err)
	}
	if home == "" {
		return "", errors.New("home is empty")
	}
	return home, nil
}

func envOr(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
