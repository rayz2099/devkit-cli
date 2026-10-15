package rclone

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExampleConfigLoads(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "rclone-cli", "config.example.json"))
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := parseFileCfg(string(buf), path, "/opt/home")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RcloneBin != "rclone" || cfg.Sources[0].Name != "photograph" {
		t.Fatalf("example: %#v", cfg.Sources)
	}
	if cfg.Sources[0].Path != "/opt/home/workspace/photograph" {
		t.Fatalf("path %s", cfg.Sources[0].Path)
	}
	tgt := cfg.Targets[0]
	if tgt.Root != "photograph" || !tgt.SkipLinks || !tgt.SkipHidden || !tgt.SizeOnly {
		t.Fatalf("target defaults %#v", tgt)
	}
	if tgt.PacerMinSleep != "200ms" || tgt.EncryptFilenames || tgt.HasCrypt {
		t.Fatalf("crypt %#v", tgt)
	}
	if cfg.Tasks[0].Name != "photograph-123" {
		t.Fatal(cfg.Tasks[0].Name)
	}
}

func TestHomeExpand(t *testing.T) {
	body := sampleCfg(nil)
	sources := body["sources"].([]any)
	source := sources[0].(map[string]any)
	source["path"] = "$HOME/workspace/photograph"
	cfg := parseSample(t, body)
	if cfg.Sources[0].Path != "/tmp/rclone-home/workspace/photograph" {
		t.Fatal(cfg.Sources[0].Path)
	}
	source["path"] = "${HOME}/workspace/photograph"
	cfg = parseSample(t, body)
	if cfg.Sources[0].Path != "/tmp/rclone-home/workspace/photograph" {
		t.Fatal(cfg.Sources[0].Path)
	}
	source["path"] = "$HOMEWORK/photograph"
	_, err := parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "sources[0].path must be absolute") {
		t.Fatal(err)
	}
}

func TestConfigRejects(t *testing.T) {
	_, err := parseFileCfg(`{ "sources": [], "targets": [{}], "tasks": [] }`, "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "c.json targets[0].name is required") {
		t.Fatal(err)
	}
	body := sampleCfg(map[string]any{"crypt": map[string]any{"key": "one", "salt": "two"}})
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "must be an array of key pairs") {
		t.Fatal(err)
	}
	pair := samplePair(nil)
	delete(pair, "salt")
	body = sampleCfg(map[string]any{"crypt": []any{pair}})
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "targets[0].crypt[0].salt is required") {
		t.Fatal(err)
	}
	body = sampleCfg(map[string]any{"crypt": []any{map[string]any{
		"name": "v1", "algorithm": "secretbox", "password": "one", "password2": "two", "default": true,
	}}})
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "use key and salt, not password/password2") {
		t.Fatal(err)
	}
	body = sampleCfg(map[string]any{"crypt": []any{}})
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "at least one key pair") {
		t.Fatal(err)
	}
	body = sampleCfg(map[string]any{"crypt": []any{samplePair(map[string]any{"name": "a"}), samplePair(map[string]any{"name": "b", "default": true})}})
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "exactly one default key pair") {
		t.Fatal(err)
	}
	body = sampleCfg(map[string]any{"crypt": []any{samplePair(map[string]any{"algorithm": "aes"})}})
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "algorithm must be secretbox") {
		t.Fatal(err)
	}
	body = sampleCfg(map[string]any{"encryptFilenames": true})
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "encryptFilenames requires crypt") {
		t.Fatal(err)
	}
	body = sampleCfg(map[string]any{"task": map[string]any{"encryptFilenames": true}})
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "encryptFilenames requires crypt on the target") {
		t.Fatal(err)
	}
	body = sampleCfg(nil)
	tasks := body["tasks"].([]any)
	tasks[0].(map[string]any)["url"] = "https://evil.example"
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "url is not allowed") {
		t.Fatal(err)
	}
	body = sampleCfg(nil)
	body["defaultTask"] = "photograph-123"
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "defaultTask is not allowed") {
		t.Fatal(err)
	}
	body = sampleCfg(map[string]any{"task": map[string]any{"source": "missing"}})
	_, err = parseFileCfg(mustJSON(t, body), "c.json", "/tmp/rclone-home")
	if err == nil || !strings.Contains(err.Error(), "tasks[0].source not found: missing") {
		t.Fatal(err)
	}
}

func TestOmittedCryptDefault(t *testing.T) {
	first := samplePair(map[string]any{"name": "v1"})
	delete(first, "default")
	second := samplePair(map[string]any{"name": "legacy", "key": "old", "salt": "s"})
	delete(second, "default")
	delete(second, "algorithm")
	cfg := parseSample(t, sampleCfg(map[string]any{"crypt": []any{first, second}}))
	if !cfg.Targets[0].Crypt[0].Default || cfg.Targets[0].Crypt[1].Default {
		t.Fatalf("%#v", cfg.Targets[0].Crypt)
	}
	if cfg.Targets[0].Crypt[1].Algorithm != "secretbox" {
		t.Fatal(cfg.Targets[0].Crypt[1].Algorithm)
	}
}

func TestJSONCAndDefaults(t *testing.T) {
	text := `{
    // keep
    "sources": [{ "name": "photograph", "path": "/tmp/photo", }],
    "targets": [{
      "name": "123",
      "url": "https://webdav.123pan.cn/webdav",
      "vendor": "other",
      "user": "",
      "pass": "",
      "root": "photograph",
      "skipLinks": true,
      "skipHidden": true,
      "sizeOnly": true,
      "transfers": 1,
      "checkers": 4,
      "retries": 10,
      "retriesSleep": "15s",
      "lowLevelRetries": 20,
      "timeout": "1h",
      "contimeout": "60s",
      "tpslimit": 2,
      "pacerMinSleep": "200ms",
      "encryptFilenames": false,
    }],
    "tasks": [{ "name": "photograph-123", "source": "photograph", "target": "123", }],
  }`
	cfg, err := parseFileCfg(text, "c.json", "/tmp/rclone-home")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Targets[0].SkipLinks || cfg.Targets[0].PacerMinSleep != "200ms" {
		t.Fatalf("%#v", cfg.Targets[0])
	}
	body := sampleCfg(nil)
	target := body["targets"].([]any)[0].(map[string]any)
	delete(target, "skipLinks")
	delete(target, "skipHidden")
	delete(target, "sizeOnly")
	delete(target, "pacerMinSleep")
	delete(target, "encryptFilenames")
	delete(target, "vendor")
	cfg = parseSample(t, body)
	tgt := cfg.Targets[0]
	if !tgt.SkipLinks || !tgt.SkipHidden || !tgt.SizeOnly || tgt.PacerMinSleep != "200ms" || tgt.EncryptFilenames || tgt.Vendor != "other" {
		t.Fatalf("%#v", tgt)
	}
}

func TestSecretMode(t *testing.T) {
	home := withHome(t)
	path := writeCfg(t, home, sampleCfg(nil), 0o644)
	if _, err := Load(path, home); err == nil || !strings.Contains(err.Error(), "config file mode must be 600") {
		t.Fatal(err)
	}
	writeCfg(t, home, sampleCfg(nil), 0o600)
	cfg, err := Load(path, home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tasks[0].Name != "photograph-123" {
		t.Fatal(cfg.Tasks[0].Name)
	}
	missing := filepath.Join(home, ".config", "rclone-cli", "nope.json")
	_, err = Load(missing, home)
	if err == nil || !strings.Contains(err.Error(), "config missing") {
		t.Fatal(err)
	}
}
