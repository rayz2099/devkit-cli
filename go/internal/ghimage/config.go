package ghimage

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

const configExample = "gh-image-cli/README.md"

// config 是镜像仓库的三个稳定参数。项目里的镜像状态不放这里，归 images.yaml。
type config struct {
	registry       string
	namespace      string
	timeoutSeconds int
}

func defaultConfig() config {
	return config{
		registry:       "registry.cn-shanghai.aliyuncs.com",
		namespace:      "linran-pub",
		timeoutSeconds: 600,
	}
}

func (a *app) configPath() (string, bool, error) {
	if a.cfg != nil {
		return *a.cfg, true, nil
	}
	dirs, err := xdg.Resolve("gh-image-cli")
	if err != nil {
		return "", false, err
	}
	return filepath.Join(dirs.Config, "config.json"), false, nil
}

// loadConfig 读配置。strict 只给 doctor：文件必须存在，空的 registry/namespace 不算通过。
// 普通命令在默认路径缺文件时用内置默认值，这是原 CLI 的行为，不是另找一份配置。
func (a *app) loadConfig(strict bool) (config, error) {
	path, explicit, err := a.configPath()
	if err != nil {
		return config{}, err
	}
	buf, err := cfgfile.Read(path, configExample)
	if err != nil {
		if errors.Is(err, cfgfile.ErrMissing) && !explicit && !strict {
			return defaultConfig(), nil
		}
		if errors.Is(err, cfgfile.ErrMissing) && explicit && !strict {
			return config{}, fmt.Errorf("config file not found: %s", path)
		}
		return config{}, err
	}
	cfg, err := parseConfig(buf)
	if err != nil {
		return config{}, err
	}
	if strict {
		if cfg.registry == "" {
			return config{}, errors.New("registry is required")
		}
		if cfg.namespace == "" {
			return config{}, errors.New("namespace is required")
		}
	}
	return cfg, nil
}

func parseConfig(buf []byte) (config, error) {
	var raw map[string]json.RawMessage
	if err := cfgfile.ParseJSON(buf, &raw); err != nil {
		return config{}, err
	}
	if raw == nil {
		return config{}, errors.New("invalid config: expected object")
	}
	cfg := defaultConfig()
	if val, ok := raw["registry"]; ok {
		text, err := jsonString(val, "registry")
		if err != nil {
			return config{}, err
		}
		cfg.registry = text
	}
	if val, ok := raw["namespace"]; ok {
		text, err := jsonString(val, "namespace")
		if err != nil {
			return config{}, err
		}
		cfg.namespace = text
	}
	if val, ok := raw["timeoutSeconds"]; ok {
		sec, err := positiveSeconds(val)
		if err != nil {
			return config{}, err
		}
		cfg.timeoutSeconds = sec
	}
	return cfg, nil
}

func jsonString(val json.RawMessage, name string) (string, error) {
	var text string
	if err := json.Unmarshal(val, &text); err != nil {
		return "", fmt.Errorf("invalid config: %s must be a string", name)
	}
	return text, nil
}

func positiveSeconds(val json.RawMessage) (int, error) {
	var num json.Number
	if err := json.Unmarshal(val, &num); err != nil {
		return 0, errors.New("timeoutSeconds must be a positive integer")
	}
	sec, err := num.Float64()
	if err != nil || sec != math.Trunc(sec) || sec <= 0 || sec > float64(math.MaxInt) {
		return 0, errors.New("timeoutSeconds must be a positive integer")
	}
	return int(sec), nil
}

func (a *app) doctor() error {
	_, err := a.loadConfig(true)
	items := []doctor.Item{{Name: "config", Err: err}}
	werr := doctor.Write(a.out, items)
	if werr == nil {
		return nil
	}
	if err != nil {
		return &coded{n: 1}
	}
	return werr
}

func imageRef(cfg config, alias, version string) string {
	base := cfg.registry + "/" + cfg.namespace + "/" + alias
	if version == "" {
		return base
	}
	return base + ":" + version
}
