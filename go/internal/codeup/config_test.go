package codeup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileCfgTrailingComma(t *testing.T) {
	sample := `{
  "defaultProfile": "work",
  "profiles": [
    {
      "name": "work",
      "url": "https://codeup.aliyun.com/org-example/",
      "token": "tok",
    }
  ]
}`
	cfg, err := ParseFileCfg(sample, "/tmp/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProfile != "work" || cfg.Profiles[0].URL != "https://codeup.aliyun.com/org-example" || cfg.Profiles[0].Token != "tok" {
		t.Fatalf("cfg = %+v", cfg)
	}
	rt, err := ResolveRuntime(cfg, audienceAgent, "")
	if err != nil {
		t.Fatal(err)
	}
	if rt.OrgID != "org-example" || rt.APIBase != APIBase || rt.Audience != audienceAgent {
		t.Fatalf("runtime = %+v", rt)
	}
}

func TestParseFileCfgRequiresToken(t *testing.T) {
	raw := `{
  "defaultProfile": "work",
  "profiles": [{ "name": "work", "url": "https://codeup.aliyun.com/org-example", "token": "" }]
}`
	_, err := ParseFileCfg(raw, "/tmp/config.json")
	if err == nil || !contains(err.Error(), "token is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileCfgUsesXDGAndFlag(t *testing.T) {
	home := setHome(t)
	xdgFile := filepath.Join(home, ".config", "codeup-cli", "config.json")
	if err := os.MkdirAll(filepath.Dir(xdgFile), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{
  "defaultProfile": "work",
  "profiles": [{ "name": "work", "url": "https://codeup.aliyun.com/org-example", "token": "tok" }]
}`
	if err := os.WriteFile(xdgFile, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFileCfg("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProfile != "work" {
		t.Fatalf("default = %s", cfg.DefaultProfile)
	}
	other := filepath.Join(home, "other.json")
	alt := `{
  "defaultProfile": "alt",
  "profiles": [{ "name": "alt", "url": "https://codeup.aliyun.com/org-alt", "token": "t2" }]
}`
	if err := os.WriteFile(other, []byte(alt), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFileCfg(other)
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultProfile != "alt" {
		t.Fatalf("flag config = %+v", got)
	}
	_, err = LoadFileCfg(filepath.Join(home, "missing.json"))
	if err == nil || !contains(err.Error(), "config file not found:") {
		t.Fatalf("missing = %v", err)
	}
}

func setHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	return home
}
