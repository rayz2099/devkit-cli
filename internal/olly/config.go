package olly

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
	"git.internal.linran.top/linran/devkit-cli/internal/grafana"
	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

const configExample = "olly-cli/config.example.json"

// DefaultPrometheusBase 是零配置时的 Prometheus 地址，沿用原 prom-cli 约定。
const DefaultPrometheusBase = "http://127.0.0.1:9090"

// App 是一份已校验的配置。Prometheus 始终有值，其余后端缺段就是 nil。
type App struct {
	Uptrace    *UptraceConfig
	Prometheus PrometheusConfig
	Graylog    *GraylogConfig
	Grafana    *GrafanaConfig
}

// UptraceConfig 是 Uptrace JSON API 的连接参数。jwt 和 auth token 二选一，同时有时优先 jwt。
type UptraceConfig struct {
	BaseURL        string
	WebBaseURL     string
	HasWeb         bool
	ProjectID      float64
	AuthToken      string
	JWTToken       string
	HasAuth        bool
	HasJWT         bool
	DefaultEnv     string
	HasDefaultEnv  bool
	DefaultTimeDur *float64
}

// PrometheusConfig 可以没有配置文件。base URL 有内置默认值。
type PrometheusConfig struct {
	BaseURL        string
	DefaultStep    string
	HasStep        bool
	DefaultTimeout string
	HasTimeout     bool
}

// GraylogConfig 走 basic auth。没 scheme 的内网地址要补成 http URL，否则请求无法发出。
type GraylogConfig struct {
	BaseURL  string
	Username string
	Password string
}

// GrafanaConfig 走内网 basic。字段校验在 grafana 包，这里保留原来的类型名。
type GrafanaConfig = grafana.Config

// Guard 确认缓存和状态不会落到配置目录里。本命令不写缓存，但路径非法时必须先失败。
func Guard() error {
	_, err := xdg.Resolve("olly-cli")
	return err
}

// DefaultPath 是 ~/.config/olly-cli/config.json，并认 XDG_CONFIG_HOME。
func DefaultPath() (string, error) {
	root, err := xdg.ConfigRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "olly-cli", "config.json"), nil
}

// Load 读取并校验配置。显式路径和默认路径缺失都是错误，Prometheus 零配置不走这里。
func Load(path string) (App, error) {
	app, found, err := readApp(path, true)
	if err != nil {
		return App{}, err
	}
	if !found {
		return App{}, fmt.Errorf("config file not found: %s", path)
	}
	return app, nil
}

// LoadOptional 给 Prometheus 用。默认路径可以不存在；-c 指向的文件缺失仍然是错误。
func LoadOptional(path string, explicit bool) (App, bool, error) {
	return readApp(path, explicit)
}

func readApp(path string, explicit bool) (App, bool, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return App{}, false, err
		}
	}
	buf, err := cfgfile.Read(path, configExample)
	if err != nil {
		if errors.Is(err, cfgfile.ErrMissing) && !explicit {
			return App{Prometheus: normalizeProm(nil)}, false, nil
		}
		if errors.Is(err, cfgfile.ErrMissing) {
			return App{}, false, fmt.Errorf("config file not found: %s", path)
		}
		return App{}, false, err
	}
	app, err := normalize(buf)
	if err != nil {
		return App{}, false, err
	}
	return app, true, nil
}

func normalize(buf []byte) (App, error) {
	sections, err := splitSections(buf)
	if err != nil {
		return App{}, err
	}
	var app App
	if raw, ok := present(sections, "uptrace"); ok {
		cfg, err := normalizeUp(raw)
		if err != nil {
			return App{}, err
		}
		app.Uptrace = &cfg
	}
	prom, err := normalizePromRaw(sections["prometheus"])
	if err != nil {
		return App{}, err
	}
	app.Prometheus = prom
	if raw, ok := present(sections, "graylog"); ok {
		cfg, err := normalizeGray(raw)
		if err != nil {
			return App{}, err
		}
		app.Graylog = &cfg
	}
	if raw, ok := present(sections, "grafana"); ok {
		cfg, err := normalizeGraf(raw)
		if err != nil {
			return App{}, err
		}
		app.Grafana = &cfg
	}
	return app, nil
}

// RequireUptrace 在命令真正打 Uptrace 前拦住缺段配置。
func RequireUptrace(app App) (UptraceConfig, error) {
	if app.Uptrace == nil {
		return UptraceConfig{}, errors.New("config missing uptrace section")
	}
	return *app.Uptrace, nil
}

// RequireGraylog 在命令真正打 Graylog 前拦住缺段配置。
func RequireGraylog(app App) (GraylogConfig, error) {
	if app.Graylog == nil {
		return GraylogConfig{}, errors.New("config missing graylog section")
	}
	return *app.Graylog, nil
}

// RequireGrafana 在命令真正打 Grafana 前拦住缺段配置。
func RequireGrafana(app App) (GrafanaConfig, error) {
	if app.Grafana == nil {
		return GrafanaConfig{}, errors.New("config missing grafana section")
	}
	return *app.Grafana, nil
}

// BasicAuth 是 Grafana 和 Graylog 共用的 basic 头。实现放在 grafana，避免两套编码。
func BasicAuth(user, pass string) string {
	return grafana.BasicAuth(user, pass)
}

// Doctor 只检查已有配置能否解析、已出现的后端必填字段是否合法。不联网。
// Prometheus 没有配置也能跑，所以缺文件或没写 prometheus 段都不能把它判失败。
func Doctor(path string, w io.Writer) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	buf, err := cfgfile.Read(path, configExample)
	if err != nil {
		items := []doctor.Item{{Name: "config", Err: err}, {Name: "prometheus"}}
		return doctor.Write(w, items)
	}
	sections, err := splitSections(buf)
	if err != nil {
		items := []doctor.Item{{Name: "config", Err: err}, {Name: "prometheus"}}
		return doctor.Write(w, items)
	}
	items := []doctor.Item{{Name: "config"}}
	if raw, ok := present(sections, "uptrace"); ok {
		_, err := normalizeUp(raw)
		items = append(items, doctor.Item{Name: "uptrace", Err: err})
	}
	_, promErr := normalizePromRaw(sections["prometheus"])
	items = append(items, doctor.Item{Name: "prometheus", Err: promErr})
	if raw, ok := present(sections, "graylog"); ok {
		_, err := normalizeGray(raw)
		items = append(items, doctor.Item{Name: "graylog", Err: err})
	}
	if raw, ok := present(sections, "grafana"); ok {
		_, err := normalizeGraf(raw)
		items = append(items, doctor.Item{Name: "grafana", Err: err})
	}
	return doctor.Write(w, items)
}

func splitSections(buf []byte) (map[string]json.RawMessage, error) {
	var sections map[string]json.RawMessage
	if err := cfgfile.ParseJSON(buf, &sections); err != nil {
		return nil, err
	}
	if sections == nil {
		return nil, errors.New("invalid config: expected object")
	}
	return sections, nil
}

func present(sections map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	raw, ok := sections[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	return raw, true
}

type rawUp struct {
	BaseURL    *string  `json:"base_url"`
	WebBaseURL *string  `json:"web_base_url"`
	ProjectID  *float64 `json:"project_id"`
	AuthToken  *string  `json:"auth_token"`
	JWTToken   *string  `json:"jwt_token"`
	DefaultEnv *string  `json:"default_env"`
	DefaultDur *float64 `json:"default_time_dur_seconds"`
}

func normalizeUp(raw json.RawMessage) (UptraceConfig, error) {
	var in rawUp
	if err := json.Unmarshal(raw, &in); err != nil {
		return UptraceConfig{}, errors.New("config uptrace is invalid")
	}
	if in.BaseURL == nil || *in.BaseURL == "" {
		return UptraceConfig{}, errors.New("config missing uptrace.base_url")
	}
	if in.ProjectID == nil || *in.ProjectID == 0 {
		return UptraceConfig{}, errors.New("config missing uptrace.project_id")
	}
	hasAuth := in.AuthToken != nil && *in.AuthToken != ""
	hasJWT := in.JWTToken != nil && *in.JWTToken != ""
	if !hasAuth && !hasJWT {
		return UptraceConfig{}, errors.New("config missing uptrace auth: expected auth_token or jwt_token")
	}
	cfg := UptraceConfig{
		BaseURL:        *in.BaseURL,
		ProjectID:      *in.ProjectID,
		HasAuth:        hasAuth,
		HasJWT:         hasJWT,
		DefaultTimeDur: in.DefaultDur,
	}
	if in.WebBaseURL != nil {
		cfg.WebBaseURL = *in.WebBaseURL
		cfg.HasWeb = true
	}
	if hasAuth {
		cfg.AuthToken = *in.AuthToken
	}
	if hasJWT {
		cfg.JWTToken = *in.JWTToken
	}
	if in.DefaultEnv != nil {
		cfg.DefaultEnv = *in.DefaultEnv
		cfg.HasDefaultEnv = true
	}
	return cfg, nil
}

type rawProm struct {
	BaseURL *string `json:"base_url"`
	Step    *string `json:"default_step"`
	Timeout *string `json:"default_timeout"`
}

func normalizeProm(raw *rawProm) PrometheusConfig {
	base := DefaultPrometheusBase
	if raw != nil && raw.BaseURL != nil {
		base = *raw.BaseURL
	}
	cfg := PrometheusConfig{BaseURL: trimRightSlash(base)}
	if raw != nil && raw.Step != nil {
		cfg.DefaultStep = *raw.Step
		cfg.HasStep = true
	}
	if raw != nil && raw.Timeout != nil {
		cfg.DefaultTimeout = *raw.Timeout
		cfg.HasTimeout = true
	}
	return cfg
}

func normalizePromRaw(raw json.RawMessage) (PrometheusConfig, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return normalizeProm(nil), nil
	}
	var in rawProm
	if err := json.Unmarshal(raw, &in); err != nil {
		return PrometheusConfig{}, errors.New("config prometheus is invalid")
	}
	return normalizeProm(&in), nil
}

type rawGray struct {
	BaseURL  *string `json:"base_url"`
	Username *string `json:"username"`
	Password *string `json:"password"`
}

func normalizeGray(raw json.RawMessage) (GraylogConfig, error) {
	var in rawGray
	if err := json.Unmarshal(raw, &in); err != nil {
		return GraylogConfig{}, errors.New("config graylog is invalid")
	}
	if in.BaseURL == nil || *in.BaseURL == "" {
		return GraylogConfig{}, errors.New("config missing graylog.base_url")
	}
	if in.Username == nil || *in.Username == "" {
		return GraylogConfig{}, errors.New("config missing graylog.username")
	}
	if in.Password == nil || *in.Password == "" {
		return GraylogConfig{}, errors.New("config missing graylog.password")
	}
	return GraylogConfig{
		BaseURL:  trimRightSlash(withScheme(*in.BaseURL)),
		Username: *in.Username,
		Password: *in.Password,
	}, nil
}

func normalizeGraf(raw json.RawMessage) (GrafanaConfig, error) {
	return grafana.Normalize(raw)
}
