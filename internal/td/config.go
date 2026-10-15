package td

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

const cfgExample = "td-cli/config.example.json"

// Profile 是一条只读连接. 令牌不进配置, 每次命令用 username 和 password 现登.
type Profile struct {
	Name     string
	URL      string
	Username string
	Password string
}

// FileCfg 是配置文件. defaultProfile 决定不带 -p 时用哪一条.
type FileCfg struct {
	DefaultProfile string
	Profiles       []Profile
}

// cfgPath 走 XDG 配置根, 不创建目录. -c 显式路径不经过这里.
func cfgPath() (string, error) {
	root, err := xdg.ConfigRoot()
	if err != nil {
		return "", fmt.Errorf("HOME is required")
	}
	return filepath.Join(root, "td-cli", "config.json"), nil
}

func resolveCfg(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	return cfgPath()
}

func loadFileCfg(path string) (FileCfg, error) {
	buf, err := cfgfile.Read(path, cfgExample)
	if err != nil {
		return FileCfg{}, err
	}
	return parseFileCfg(buf, path)
}

func parseFileCfg(content []byte, path string) (FileCfg, error) {
	v, err := decodeCfg(content)
	if err != nil {
		return FileCfg{}, fmt.Errorf("invalid config file %s", path)
	}
	obj, ok := asMap(v)
	if !ok {
		return FileCfg{}, fmt.Errorf("invalid config file %s", path)
	}
	defRaw, ok := obj["defaultProfile"]
	def, isStr := defRaw.(string)
	if !ok || !isStr || strings.TrimSpace(def) == "" {
		return FileCfg{}, fmt.Errorf("invalid config file %s: defaultProfile is required", path)
	}
	items, ok := obj["profiles"].([]any)
	if !ok {
		return FileCfg{}, fmt.Errorf("invalid config file %s: profiles must be an array", path)
	}
	profiles := make([]Profile, 0, len(items))
	for i, item := range items {
		p, err := parseProfile(item, path, i)
		if err != nil {
			return FileCfg{}, err
		}
		profiles = append(profiles, p)
	}
	return FileCfg{DefaultProfile: strings.TrimSpace(def), Profiles: profiles}, nil
}

func decodeCfg(content []byte) (any, error) {
	var raw json.RawMessage
	if err := cfgfile.ParseJSONC(content, &raw); err != nil {
		return nil, err
	}
	return decodeJSON(raw)
}

func parseProfile(item any, path string, index int) (Profile, error) {
	obj, ok := asMap(item)
	if !ok {
		return Profile{}, fmt.Errorf("invalid config file %s: profiles[%d] must be an object", path, index)
	}
	name, err := needStr(obj["name"], path, fmt.Sprintf("profiles[%d].name", index))
	if err != nil {
		return Profile{}, err
	}
	url, err := needStr(obj["url"], path, fmt.Sprintf("profiles[%d].url", index))
	if err != nil {
		return Profile{}, err
	}
	user, err := readStr(obj["username"], path, fmt.Sprintf("profiles[%d].username", index))
	if err != nil {
		return Profile{}, err
	}
	pass, err := readStr(obj["password"], path, fmt.Sprintf("profiles[%d].password", index))
	if err != nil {
		return Profile{}, err
	}
	return Profile{Name: name, URL: url, Username: user, Password: pass}, nil
}

func needStr(v any, path, field string) (string, error) {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("invalid config file %s: %s is required", path, field)
	}
	return strings.TrimSpace(s), nil
}

func readStr(v any, path, field string) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("invalid config file %s: %s is required", path, field)
	}
	return s, nil
}

func pickProfile(file FileCfg, name string) (Profile, error) {
	want := name
	if want == "" {
		want = file.DefaultProfile
	}
	for _, item := range file.Profiles {
		if item.Name == want {
			return item, nil
		}
	}
	return Profile{}, fmt.Errorf("profile not found: %s", want)
}

func profileNames(file FileCfg) []string {
	out := make([]string, 0, len(file.Profiles))
	for _, item := range file.Profiles {
		out = append(out, item.Name)
	}
	return out
}
