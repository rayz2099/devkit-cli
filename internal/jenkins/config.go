package jenkins

import (
	"fmt"
	"path/filepath"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

const (
	appName    = "jenkins-cli"
	cfgFile    = "config.json"
	cfgExample = "jenkins-cli/config.example.json"
)

// LoadFileCfg 读取配置。-c 优先于 XDG，是因为旧 CLI 允许把配置指到任意文件，不能改成环境变量覆盖。
func LoadFileCfg(flag string) (FileCfg, error) {
	dirs, err := xdg.Resolve(appName)
	if err != nil {
		return FileCfg{}, err
	}
	path := filepath.Join(dirs.Config, cfgFile)
	if strings.TrimSpace(flag) != "" {
		path = flag
	}
	return readCfg(path)
}

func readCfg(path string) (FileCfg, error) {
	buf, err := cfgfile.Read(path, cfgExample)
	if err != nil {
		return FileCfg{}, err
	}
	var raw map[string]any
	if err := cfgfile.ParseJSONC(buf, &raw); err != nil {
		return FileCfg{}, fmt.Errorf("invalid config file %s", path)
	}
	return parseCfg(raw, path)
}

func parseCfg(raw map[string]any, path string) (FileCfg, error) {
	def, ok := raw["defaultProfile"].(string)
	if !ok || strings.TrimSpace(def) == "" {
		return FileCfg{}, fmt.Errorf("invalid config file %s: defaultProfile is required", path)
	}
	arr, ok := raw["profiles"].([]any)
	if !ok {
		return FileCfg{}, fmt.Errorf("invalid config file %s: profiles must be an array", path)
	}
	profs := make([]Profile, 0, len(arr))
	for i, item := range arr {
		prof, err := parseProfile(item, path, i)
		if err != nil {
			return FileCfg{}, err
		}
		profs = append(profs, prof)
	}
	return FileCfg{Default: strings.TrimSpace(def), Profiles: profs}, nil
}

func parseProfile(item any, path string, index int) (Profile, error) {
	raw, ok := item.(map[string]any)
	if !ok {
		return Profile{}, fmt.Errorf("invalid config file %s: profiles[%d] must be an object", path, index)
	}
	name := readStr(raw["name"])
	if name == "" {
		return Profile{}, fmt.Errorf("invalid config file %s: profiles[%d].name is required", path, index)
	}
	url := strings.TrimRight(readStr(raw["url"]), "/")
	if url == "" {
		return Profile{}, fmt.Errorf("invalid config file %s: url is required for profile %s", path, name)
	}
	user := readStr(raw["username"])
	if user == "" {
		return Profile{}, fmt.Errorf("invalid config file %s: username is required for profile %s", path, name)
	}
	tok := readStr(raw["apiToken"])
	if tok == "" {
		tok = readStr(raw["api-token"])
	}
	return Profile{
		Name:     name,
		URL:      url,
		User:     user,
		Password: readStr(raw["password"]),
		APIToken: tok,
	}, nil
}

// PickProfile 按名字选 profile。找不到就列出已有名字，避免用户对着空错误猜。
func PickProfile(cfg FileCfg, name string) (Profile, error) {
	want := strings.TrimSpace(name)
	if want == "" {
		want = cfg.Default
	}
	for _, prof := range cfg.Profiles {
		if prof.Name == want {
			return prof, nil
		}
	}
	extra := ""
	if len(cfg.Profiles) > 0 {
		names := make([]string, 0, len(cfg.Profiles))
		for _, prof := range cfg.Profiles {
			names = append(names, prof.Name)
		}
		extra = ". available: " + strings.Join(names, ", ")
	}
	return Profile{}, fmt.Errorf("profile not found: %s%s", want, extra)
}

// ResolveSecret 取 Basic 用的密钥。apiToken 优先是点名的顺序，两个都空必须失败，不再去第三处找。
func ResolveSecret(prof Profile) (string, error) {
	if strings.TrimSpace(prof.APIToken) != "" {
		return prof.APIToken, nil
	}
	if strings.TrimSpace(prof.Password) != "" {
		return prof.Password, nil
	}
	return "", fmt.Errorf("profile %s has empty apiToken and password", prof.Name)
}

type runtime struct {
	aud    string
	prof   Profile
	secret string
}

func resolveRT(cfg FileCfg, aud, profile string) (runtime, error) {
	prof, err := PickProfile(cfg, profile)
	if err != nil {
		return runtime{}, err
	}
	secret, err := ResolveSecret(prof)
	if err != nil {
		return runtime{}, err
	}
	return runtime{aud: aud, prof: prof, secret: secret}, nil
}

func readStr(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
