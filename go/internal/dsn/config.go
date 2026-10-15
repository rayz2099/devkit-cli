package dsn

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

// 示例路径写进缺文件错误里，让用户自己复制，程序不创建配置目录。
const exampleCfg = "dsn-cli/config.example.json"

// Profile 是一条连接。Description 为空表示配置里没写用途，ds 不补默认文案。
type Profile struct {
	Name        string
	Kind        string
	URL         string
	Access      string
	Description string
}

// File 是读入后还能用的 profile。坏掉的单条在 Problems 里，不挡其它 kind。
type File struct {
	Profiles []Profile
	Problems []ProfileProblem
}

// ProfileProblem 是被跳过的单条。doctor 要报出来，query 继续用剩下的。
type ProfileProblem struct {
	Name string
	Err  error
}

var kinds = map[string]struct{}{
	"mysql":         {},
	"doris":         {},
	"postgres":      {},
	"redis":         {},
	"mongodb":       {},
	"elasticsearch": {},
	"kafka":         {},
}

// ConfigPath 只在没给 -c 时落到 XDG 配置根。调用方只读，这里不 mkdir。
func ConfigPath(flag string) (string, error) {
	if strings.TrimSpace(flag) != "" {
		return flag, nil
	}
	root, err := xdg.ConfigRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "dsn-cli", "config.json"), nil
}

// Load 读取并解析配置。文件级错误直接失败；单条坏配置记进 Problems。
func Load(path string) (File, error) {
	buf, err := cfgfile.Read(path, exampleCfg)
	if err != nil {
		return File{}, err
	}
	return Parse(buf, path)
}

// Parse 接受 JSONC。未知 kind 或坏 URL 跳过该条，避免一个 profile 挡死其它 kind。
func Parse(buf []byte, path string) (File, error) {
	var raw map[string]any
	if err := cfgfile.ParseJSONC(buf, &raw); err != nil {
		return File{}, fmt.Errorf("invalid config file %s", path)
	}
	if raw == nil {
		return File{}, fmt.Errorf("invalid config file %s", path)
	}
	arr, ok := raw["profiles"].([]any)
	if !ok {
		return File{}, fmt.Errorf("invalid config file %s: profiles must be an array", path)
	}
	var file File
	for i, item := range arr {
		prof, err := parseProfile(item, path, i)
		if err != nil {
			name := problemName(item, i)
			file.Problems = append(file.Problems, ProfileProblem{Name: name, Err: err})
			continue
		}
		file.Profiles = append(file.Profiles, prof)
	}
	return file, nil
}

func problemName(item any, index int) string {
	obj, ok := item.(map[string]any)
	if !ok {
		return fmt.Sprintf("profiles[%d]", index)
	}
	name, _ := obj["name"].(string)
	if name == "" {
		return fmt.Sprintf("profiles[%d]", index)
	}
	return name
}

func parseProfile(item any, path string, index int) (Profile, error) {
	obj, ok := item.(map[string]any)
	if !ok {
		return Profile{}, fmt.Errorf("invalid config file %s: profiles[%d] must be an object", path, index)
	}
	name, _ := obj["name"].(string)
	if name == "" {
		return Profile{}, fmt.Errorf("invalid config file %s: profiles[%d].name is required", path, index)
	}
	kind, err := readKind(obj["kind"], path, name)
	if err != nil {
		return Profile{}, err
	}
	rawURL, _ := obj["url"].(string)
	if rawURL == "" {
		return Profile{}, fmt.Errorf("invalid config file %s: url is required for profile %s", path, name)
	}
	if err := assertURL(kind, rawURL, name); err != nil {
		return Profile{}, err
	}
	access, err := readAccess(obj["access"], path, name)
	if err != nil {
		return Profile{}, err
	}
	desc, err := readDesc(obj["description"], path, name)
	if err != nil {
		return Profile{}, err
	}
	return Profile{Name: name, Kind: kind, URL: rawURL, Access: access, Description: desc}, nil
}

func readKind(value any, path, name string) (string, error) {
	kind, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("invalid config file %s: kind is required for profile %s", path, name)
	}
	if _, known := kinds[kind]; !known {
		return "", fmt.Errorf("invalid config file %s: unknown kind %s for profile %s", path, kind, name)
	}
	return kind, nil
}

func readDesc(value any, path, name string) (string, error) {
	if value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("invalid config file %s: description must be a string for profile %s", path, name)
	}
	text = strings.TrimSpace(text)
	return text, nil
}

func readAccess(value any, path, name string) (string, error) {
	if value == nil {
		return "read", nil
	}
	access, ok := value.(string)
	if !ok || (access != "read" && access != "write") {
		return "", fmt.Errorf("invalid config file %s: access must be read or write for profile %s", path, name)
	}
	return access, nil
}

// assertURL 把 kind 和 scheme 绑死，避免驱动按错误协议拨号还当配置合法。
func assertURL(kind, raw, name string) error {
	if kind == "mongodb" {
		if !strings.HasPrefix(raw, "mongodb://") && !strings.HasPrefix(raw, "mongodb+srv://") {
			return fmt.Errorf("profile %s: mongodb url must use mongodb://", name)
		}
		if !mongoURLOk(raw) {
			return fmt.Errorf("invalid url for profile %s", name)
		}
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" && kind != "elasticsearch" {
		// elasticsearch 必须有 host；url.Parse 对 mysql:// 也要求能解析。
		if err != nil || parsed.Scheme == "" {
			return fmt.Errorf("invalid url for profile %s", name)
		}
	}
	if err != nil {
		return fmt.Errorf("invalid url for profile %s", name)
	}
	proto := parsed.Scheme
	switch kind {
	case "mysql", "doris":
		if proto != "mysql" {
			return fmt.Errorf("profile %s: %s url must use mysql://", name, kind)
		}
	case "redis":
		if proto != "redis" && proto != "rediss" {
			return fmt.Errorf("profile %s: redis url must use redis:// or rediss://", name)
		}
	case "elasticsearch":
		if proto != "http" && proto != "https" {
			return fmt.Errorf("profile %s: elasticsearch url must use http:// or https://", name)
		}
		if parsed.Host == "" {
			return fmt.Errorf("invalid url for profile %s", name)
		}
	case "postgres":
		if proto != "postgres" && proto != "postgresql" {
			return fmt.Errorf("profile %s: postgres url must use postgres:// or postgresql://", name)
		}
	case "kafka":
		if proto != "kafka" && proto != "kafkas" {
			return fmt.Errorf("profile %s: kafka url must use kafka:// or kafkas://", name)
		}
	default:
		return fmt.Errorf("invalid config file: unknown kind %s for profile %s", kind, name)
	}
	return nil
}

// Pick 按去掉首尾空白后的名字找 profile。找不到时带上可用名字，避免 agent 盲试。
func Pick(file File, name string) (Profile, error) {
	wanted := strings.TrimSpace(name)
	for _, item := range file.Profiles {
		if item.Name == wanted {
			return item, nil
		}
	}
	names := make([]string, 0, len(file.Profiles))
	for _, item := range file.Profiles {
		names = append(names, item.Name)
	}
	msg := "profile not found: " + wanted
	if len(names) > 0 {
		msg += ". available: " + strings.Join(names, ", ")
	}
	return Profile{}, exitf(1, "%s", msg)
}
