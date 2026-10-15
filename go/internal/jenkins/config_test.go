package jenkins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
)

const sampleCfg = `{
  "defaultProfile": "prod",
  "profiles": [
    {
      "name": "prod",
      "url": "http://10.0.0.1:8080/",
      "username": "admin",
      "password": "pass",
      "apiToken": "token",
    }
  ]
}`

func TestParseFileCfg(t *testing.T) {
	cfg, err := parseText(sampleCfg, "/tmp/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Default != "prod" || cfg.Profiles[0].URL != "http://10.0.0.1:8080" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.Profiles[0].APIToken != "token" {
		t.Fatalf("token = %q", cfg.Profiles[0].APIToken)
	}
	commented := sampleCfg + "\n// note\n"
	if _, err := parseText(commented, "/tmp/config.json"); err != nil {
		t.Fatal(err)
	}
}

func TestResolveSecretOrder(t *testing.T) {
	tok, err := ResolveSecret(Profile{Name: "prod", APIToken: "token", Password: "pass"})
	if err != nil || tok != "token" {
		t.Fatalf("token = %q %v", tok, err)
	}
	pass, err := ResolveSecret(Profile{Name: "prod", APIToken: "", Password: "pass"})
	if err != nil || pass != "pass" {
		t.Fatalf("pass = %q %v", pass, err)
	}
	_, err = ResolveSecret(Profile{Name: "prod"})
	if err == nil || !strings.Contains(err.Error(), "empty apiToken and password") {
		t.Fatalf("empty = %v", err)
	}
}

func TestAPITokenAlias(t *testing.T) {
	raw := `{
	  "defaultProfile": "prod",
	  "profiles": [{"name": "prod", "url": "http://x", "username": "u", "api-token": "alt"}]
	}`
	cfg, err := parseText(raw, "/tmp/c.json")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles[0].APIToken != "alt" {
		t.Fatalf("alias = %q", cfg.Profiles[0].APIToken)
	}
}

func TestConfigPathHonorsXDGAndFlag(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	path := filepath.Join(xdg, "jenkins-cli", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.ReplaceAll(sampleCfg, "http://10.0.0.1:8080/", "http://127.0.0.1:9")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFileCfg("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles[0].URL != "http://127.0.0.1:9" {
		t.Fatalf("xdg url = %s", cfg.Profiles[0].URL)
	}

	flag := filepath.Join(t.TempDir(), "other.json")
	if err := os.WriteFile(flag, []byte(sampleCfg), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadFileCfg(flag)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles[0].URL != "http://10.0.0.1:8080" {
		t.Fatalf("flag url = %s", cfg.Profiles[0].URL)
	}
}

func TestCacheInsideConfigRejected(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(cfg, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	_, err := LoadFileCfg("")
	if err == nil || !strings.Contains(err.Error(), "refusing path inside config dir") {
		t.Fatalf("err = %v", err)
	}
}

func parseText(text, path string) (FileCfg, error) {
	var raw map[string]any
	if err := cfgfile.ParseJSONC([]byte(text), &raw); err != nil {
		return FileCfg{}, err
	}
	return parseCfg(raw, path)
}
