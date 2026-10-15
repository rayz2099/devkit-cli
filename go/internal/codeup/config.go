package codeup

import (
	"errors"
	"path/filepath"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

const (
	appName    = "codeup-cli"
	cfgExample = "codeup-cli/config.example.json"
	cfgFile    = "config.json"
)

// DefaultCfgPath 用 XDG 配置根拼 config.json。-c 没给时才走这里，不另读环境变量里的 token。
func DefaultCfgPath() (string, error) {
	dirs, err := xdg.Resolve(appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs.Config, cfgFile), nil
}

func resolveCfgPath(flag string) (string, error) {
	if strings.TrimSpace(flag) != "" {
		return flag, nil
	}
	return DefaultCfgPath()
}

// ParseFileCfg 校验必填字段。组织 URL 不合法就失败，不默默补一个 Organization。
func ParseFileCfg(content, path string) (FileCfg, error) {
	var raw any
	if err := cfgfile.ParseJSONC([]byte(content), &raw); err != nil {
		return FileCfg{}, failf(1, "invalid config file %s", path)
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return FileCfg{}, failf(1, "invalid config file %s", path)
	}
	name, ok := obj["defaultProfile"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return FileCfg{}, failf(1, "invalid config file %s: defaultProfile is required", path)
	}
	items, ok := obj["profiles"].([]any)
	if !ok {
		return FileCfg{}, failf(1, "invalid config file %s: profiles must be an array", path)
	}
	profiles := make([]Profile, 0, len(items))
	for i, item := range items {
		prof, err := parseProfile(item, path, i)
		if err != nil {
			return FileCfg{}, err
		}
		profiles = append(profiles, prof)
	}
	return FileCfg{DefaultProfile: strings.TrimSpace(name), Profiles: profiles}, nil
}

func parseProfile(item any, path string, index int) (Profile, error) {
	obj, ok := item.(map[string]any)
	if !ok {
		return Profile{}, failf(1, "invalid config file %s: profiles[%d] must be an object", path, index)
	}
	name := readStr(obj["name"])
	if name == "" {
		return Profile{}, failf(1, "invalid config file %s: profiles[%d].name is required", path, index)
	}
	rawURL := strings.TrimRight(readStr(obj["url"]), "/")
	if rawURL == "" {
		return Profile{}, failf(1, "invalid config file %s: url is required for profile %s", path, name)
	}
	token := readStr(obj["token"])
	if token == "" {
		return Profile{}, failf(1, "invalid config file %s: token is required for profile %s", path, name)
	}
	if _, err := ParseOrgID(rawURL); err != nil {
		return Profile{}, err
	}
	return Profile{Name: name, URL: rawURL, Token: token}, nil
}

// LoadFileCfg 每次从磁盘读。补全和正式命令必须看到同一份文件。
func LoadFileCfg(path string) (FileCfg, error) {
	resolved, err := resolveCfgPath(path)
	if err != nil {
		return FileCfg{}, err
	}
	buf, err := cfgfile.Read(resolved, cfgExample)
	if err != nil {
		if errors.Is(err, cfgfile.ErrMissing) {
			return FileCfg{}, failf(1, "config file not found: %s", resolved)
		}
		return FileCfg{}, err
	}
	return ParseFileCfg(string(buf), resolved)
}

// PickProfile 按名字取连接。找不到就列出已有名字，不退回第一条。
func PickProfile(cfg FileCfg, name string) (Profile, error) {
	wanted := name
	if strings.TrimSpace(wanted) == "" {
		wanted = cfg.DefaultProfile
	}
	wanted = strings.TrimSpace(wanted)
	names := make([]string, 0, len(cfg.Profiles))
	for _, item := range cfg.Profiles {
		if item.Name == wanted {
			return item, nil
		}
		names = append(names, item.Name)
	}
	if len(names) == 0 {
		return Profile{}, failf(1, "profile not found: %s", wanted)
	}
	return Profile{}, failf(1, "profile not found: %s. available: %s", wanted, strings.Join(names, ", "))
}

// ResolveRuntime 把选中的 profile 收成一次调用的连接参数。
func ResolveRuntime(cfg FileCfg, audience, profileName string) (Runtime, error) {
	prof, err := PickProfile(cfg, profileName)
	if err != nil {
		return Runtime{}, err
	}
	orgID, err := ParseOrgID(prof.URL)
	if err != nil {
		return Runtime{}, err
	}
	if audience == "" {
		audience = audienceHuman
	}
	return Runtime{
		Audience: audience,
		Profile:  prof,
		OrgID:    orgID,
		Token:    prof.Token,
		APIBase:  APIBase,
	}, nil
}

func readStr(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}
