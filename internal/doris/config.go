package doris

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

// ClusterConfig 定义单套 Doris 集群连接参数。
type ClusterConfig struct {
	Host       string `json:"host"`
	QueryPort  int    `json:"query_port"`
	HTTPPort   int    `json:"http_port"`
	User       string `json:"user"`
	Password   string `json:"password"`
	Database   string `json:"database"`
	TimeoutSec int    `json:"timeout_sec"`
}

// Timeout 计算执行上下文的超时时限。
func (c ClusterConfig) Timeout() time.Duration {
	if c.TimeoutSec <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.TimeoutSec) * time.Second
}

// FileConfig 支持多集群 Profiles 管理。
// 通过 default_profile 指定未显式传参时的默认集群环境。
type FileConfig struct {
	DefaultProfile string                   `json:"default_profile"`
	Profiles       map[string]ClusterConfig `json:"profiles"`
}

// DefaultClusterConfig 提供单机环境默认值。
func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		Host:       "127.0.0.1",
		QueryPort:  9030,
		HTTPPort:   8030,
		User:       "root",
		Password:   "",
		Database:   "",
		TimeoutSec: 10,
	}
}

// ResolveConfigFile 获取配置文件实际物理路径，对齐 dsn-cli 的 XDG 规则。
func ResolveConfigFile(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	root, err := xdg.ConfigRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "doris-cli", "config.json"), nil
}

// LoadFileConfig 读取并反序列化完整的配置文件。
func LoadFileConfig(cfgPath string) (FileConfig, error) {
	path, err := ResolveConfigFile(cfgPath)
	if err != nil {
		return FileConfig{}, err
	}

	buf, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && cfgPath == "" {
			// 未显式指定且默认文件不存在时，提供兜底空配置，允许由环境变量驱动
			return FileConfig{
				DefaultProfile: "default",
				Profiles: map[string]ClusterConfig{
					"default": DefaultClusterConfig(),
				},
			}, nil
		}
		return FileConfig{}, fmt.Errorf("read config file: %w", err)
	}

	var fc FileConfig
	if err := json.Unmarshal(buf, &fc); err != nil {
		return FileConfig{}, fmt.Errorf("parse config file: %w", err)
	}

	return fc, nil
}

// LoadConfig 解析指定 profile 的集群配置。
// 优先级为：显式 Profile > 环境变量 DORIS_PROFILE > default_profile。
func LoadConfig(cfgPath, profileName string) (ClusterConfig, string, error) {
	fc, err := LoadFileConfig(cfgPath)
	if err != nil {
		return ClusterConfig{}, "", err
	}

	targetProf := profileName
	if targetProf == "" {
		targetProf = os.Getenv("DORIS_PROFILE")
	}
	if targetProf == "" {
		targetProf = fc.DefaultProfile
	}
	if targetProf == "" && len(fc.Profiles) > 0 {
		// 取第一个 profile
		for k := range fc.Profiles {
			targetProf = k
			break
		}
	}

	cfg, exists := fc.Profiles[targetProf]
	if !exists {
		// 若没有任何配置命中，且无显式传 profile，则使用默认单机配置
		if targetProf == "" || targetProf == "default" {
			cfg = DefaultClusterConfig()
			targetProf = "default"
		} else {
			return ClusterConfig{}, "", fmt.Errorf("profile not found: %s", targetProf)
		}
	}

	// 环境变量覆盖当前生效 profile
	if v := os.Getenv("DORIS_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("DORIS_QUERY_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.QueryPort = p
		}
	}
	if v := os.Getenv("DORIS_HTTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.HTTPPort = p
		}
	}
	if v := os.Getenv("DORIS_USER"); v != "" {
		cfg.User = v
	}
	if v := os.Getenv("DORIS_PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("DORIS_DATABASE"); v != "" {
		cfg.Database = v
	}

	return cfg, targetProf, nil
}

// ListProfileNames 提取配置中所有 profile 名字，供 shell 自动补全使用。
func ListProfileNames(cfgPath string) []string {
	fc, err := LoadFileConfig(cfgPath)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(fc.Profiles))
	for name := range fc.Profiles {
		names = append(names, name)
	}
	return names
}
