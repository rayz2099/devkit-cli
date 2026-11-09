// Package issue 提供单空间个人工作项客户端；配置只读，避免自动初始化覆盖凭据。
package issue

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

// Config 不支持 profile；Token 已绑定用户和空间，不能另选身份。
type Config struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// LoadConfig 不创建文件，也不在错误中回显配置内容。
func LoadConfig(path string) (Config, error) {
	if path == "" {
		root, err := xdg.ConfigRoot()
		if err != nil {
			return Config{}, err
		}
		path = filepath.Join(root, "issue-cli", "config.json")
	}
	buf, err := cfgfile.Read(path, "README.md#issue-cli")
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, errors.New("配置必须是仅含 url/token 的 JSON 对象")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return Config{}, errors.New("配置存在多余内容")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	return cfg, nil
}

// Validate 拒绝 URL 内嵌凭据和路径，防止 Token 发往非预期接口。
func (c Config) Validate() error {
	u, err := url.Parse(c.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("url 必须是 HTTP(S) 服务根地址，不含凭据、路径、查询或片段")
	}
	if strings.TrimSpace(c.Token) == "" || strings.ContainsAny(c.Token, " \r\n\t") {
		return errors.New("token 必填且不能含空白")
	}
	return nil
}
