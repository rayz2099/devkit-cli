package grafana

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

const configExample = "grafana-cli/config.example.json"

// Config 是一套 Grafana 的内网入口。公网 web_base_url 只用来生成给人点的链接。
type Config struct {
	BaseURL    string
	WebBaseURL string
	HasWeb     bool
	Username   string
	Password   string
}

// File 是多套集群。default_profile 必须指向 profiles 里的一个名字，省略 -P 时用它。
type File struct {
	Default  string
	Profiles map[string]Config
}

type rawGraf struct {
	BaseURL    *string `json:"base_url"`
	WebBaseURL *string `json:"web_base_url"`
	Username   *string `json:"username"`
	Password   *string `json:"password"`
}

type rawFile struct {
	Default  *string                    `json:"default_profile"`
	Profiles map[string]json.RawMessage `json:"profiles"`
}

// Normalize 校验 olly-cli 那份 grafana 连接字段。没写 scheme 的内网地址补成 http，否则客户端拒绝发请求。
func Normalize(raw json.RawMessage) (Config, error) {
	var in rawGraf
	if err := json.Unmarshal(raw, &in); err != nil {
		return Config{}, errors.New("config grafana is invalid")
	}
	if in.BaseURL == nil || *in.BaseURL == "" {
		return Config{}, errors.New("config missing grafana.base_url")
	}
	if in.Username == nil || *in.Username == "" {
		return Config{}, errors.New("config missing grafana.username")
	}
	if in.Password == nil || *in.Password == "" {
		return Config{}, errors.New("config missing grafana.password")
	}
	cfg := Config{
		BaseURL:  trimRight(withScheme(*in.BaseURL)),
		Username: *in.Username,
		Password: *in.Password,
	}
	if in.WebBaseURL != nil && *in.WebBaseURL != "" {
		cfg.WebBaseURL = trimRight(withScheme(*in.WebBaseURL))
		cfg.HasWeb = true
	}
	return cfg, nil
}

func withScheme(val string) string {
	if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") {
		return val
	}
	return "http://" + val
}

// DefaultPath 是 ~/.config/grafana-cli/config.json。安装不写这个文件。
func DefaultPath() (string, error) {
	root, err := xdg.ConfigRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "grafana-cli", "config.json"), nil
}

// Load 读取配置并选中 profile。name 为空时用 default_profile，找不到就失败。
func Load(path, name string) (string, Config, error) {
	file, err := LoadFile(path)
	if err != nil {
		return "", Config{}, err
	}
	return file.Pick(name)
}

// LoadFile 只解析配置，不联网。
func LoadFile(path string) (File, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return File{}, err
		}
	}
	buf, err := cfgfile.Read(path, configExample)
	if err != nil {
		if errors.Is(err, cfgfile.ErrMissing) {
			return File{}, fmt.Errorf("config file not found: %s", path)
		}
		return File{}, err
	}
	return parseFile(buf)
}

func parseFile(buf []byte) (File, error) {
	var in rawFile
	if err := cfgfile.ParseJSON(buf, &in); err != nil {
		return File{}, err
	}
	if in.Default == nil || strings.TrimSpace(*in.Default) == "" {
		return File{}, errors.New("config missing default_profile")
	}
	if in.Profiles == nil {
		return File{}, errors.New("config profiles must be an object")
	}
	def := strings.TrimSpace(*in.Default)
	profs := make(map[string]Config, len(in.Profiles))
	for name, raw := range in.Profiles {
		cfg, err := normalizeProfile(raw)
		if err != nil {
			return File{}, fmt.Errorf("profile %s: %w", name, err)
		}
		profs[name] = cfg
	}
	if _, ok := profs[def]; !ok {
		return File{}, fmt.Errorf("profile not found: %s", def)
	}
	return File{Default: def, Profiles: profs}, nil
}

func normalizeProfile(raw json.RawMessage) (Config, error) {
	var in rawGraf
	if err := json.Unmarshal(raw, &in); err != nil {
		return Config{}, errors.New("invalid")
	}
	if in.BaseURL == nil || *in.BaseURL == "" {
		return Config{}, errors.New("missing base_url")
	}
	if in.Username == nil || *in.Username == "" {
		return Config{}, errors.New("missing username")
	}
	if in.Password == nil || *in.Password == "" {
		return Config{}, errors.New("missing password")
	}
	cfg := Config{
		BaseURL:  trimRight(withScheme(*in.BaseURL)),
		Username: *in.Username,
		Password: *in.Password,
	}
	if in.WebBaseURL != nil && *in.WebBaseURL != "" {
		cfg.WebBaseURL = trimRight(withScheme(*in.WebBaseURL))
		cfg.HasWeb = true
	}
	return cfg, nil
}

// Pick 按名字取连接。空名字表示当前默认集群。
func (f File) Pick(name string) (string, Config, error) {
	want := strings.TrimSpace(name)
	if want == "" {
		want = f.Default
	}
	cfg, ok := f.Profiles[want]
	if !ok {
		return "", Config{}, fmt.Errorf("profile not found: %s", want)
	}
	return want, cfg, nil
}

// Names 按字典序返回 profile 名，list 输出才稳定。
func (f File) Names() []string {
	names := make([]string, 0, len(f.Profiles))
	for name := range f.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Doctor 只检查本地配置。不创建文件，也不探测 Grafana。
func Doctor(path, profile string, w io.Writer) error {
	file, err := LoadFile(path)
	if err != nil {
		return doctor.Write(w, []doctor.Item{{Name: "config", Err: err}})
	}
	items := []doctor.Item{{Name: "config"}, {Name: "default_profile"}}
	for _, name := range file.Names() {
		items = append(items, doctor.Item{Name: "profile " + name})
	}
	if strings.TrimSpace(profile) != "" {
		_, _, err := file.Pick(profile)
		items = append(items, doctor.Item{Name: "profile", Err: err})
	}
	return doctor.Write(w, items)
}

// ProfileView 是给人和 Agent 看的连接摘要。密码不能出现在这里。
type ProfileView struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	WebBase   string `json:"web_base_url,omitempty"`
	Username  string `json:"username"`
	IsDefault bool   `json:"is_default"`
	IsActive  bool   `json:"is_active"`
}

// ProfilesReport 是 profiles 命令的 Agent 形状。
type ProfilesReport struct {
	DefaultProfile string        `json:"default_profile"`
	ActiveProfile  string        `json:"active_profile"`
	Profiles       []ProfileView `json:"profiles"`
}

// Report 组装 profiles 输出。active 是本次 -P 解析后的名字。
func Report(file File, active string) ProfilesReport {
	views := make([]ProfileView, 0, len(file.Profiles))
	for _, name := range file.Names() {
		cfg := file.Profiles[name]
		views = append(views, ProfileView{
			Name:      name,
			BaseURL:   cfg.BaseURL,
			WebBase:   cfg.WebBaseURL,
			Username:  cfg.Username,
			IsDefault: name == file.Default,
			IsActive:  name == active,
		})
	}
	return ProfilesReport{DefaultProfile: file.Default, ActiveProfile: active, Profiles: views}
}
