package rclone

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sampleSrc = "/tmp/photograph"
const plainPass = "p@ss,w:rd='x'\"y"

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	return home
}

func samplePair(over map[string]any) map[string]any {
	pair := map[string]any{
		"name":      "v1",
		"algorithm": "secretbox",
		"key":       "one",
		"salt":      "two",
		"default":   true,
	}
	for key, value := range over {
		if value == nil {
			delete(pair, key)
			continue
		}
		pair[key] = value
	}
	return pair
}

func sampleCfg(over map[string]any) map[string]any {
	target := map[string]any{
		"name":             "123",
		"url":              "https://webdav.123pan.cn/webdav",
		"vendor":           "other",
		"user":             "user",
		"pass":             plainPass,
		"root":             "photograph",
		"skipLinks":        true,
		"skipHidden":       true,
		"sizeOnly":         true,
		"transfers":        1,
		"checkers":         4,
		"retries":          10,
		"retriesSleep":     "15s",
		"lowLevelRetries":  20,
		"timeout":          "1h",
		"contimeout":       "60s",
		"tpslimit":         2,
		"pacerMinSleep":    "200ms",
		"encryptFilenames": false,
	}
	if value, ok := over["user"]; ok {
		target["user"] = value
	}
	if value, ok := over["pass"]; ok {
		target["pass"] = value
	}
	for _, key := range []string{"skipLinks", "skipHidden", "sizeOnly", "encryptFilenames", "crypt", "vendor", "pacerMinSleep"} {
		if value, ok := over[key]; ok {
			if value == nil {
				delete(target, key)
				continue
			}
			target[key] = value
		}
	}
	task := map[string]any{
		"name":   "photograph-123",
		"source": "photograph",
		"target": "123",
	}
	if extra, ok := over["task"].(map[string]any); ok {
		for key, value := range extra {
			task[key] = value
		}
	}
	return map[string]any{
		"rcloneBin": "rclone",
		"sources":   []any{map[string]any{"name": "photograph", "path": sampleSrc}},
		"targets":   []any{target},
		"tasks":     []any{task},
	}
}

func parseSample(t *testing.T, body any) *FileCfg {
	t.Helper()
	text := mustJSON(t, body)
	cfg, err := parseFileCfg(text, "c.json", "/tmp/rclone-home")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func mustJSON(t *testing.T, body any) string {
	t.Helper()
	switch text := body.(type) {
	case string:
		return text
	default:
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		return string(buf)
	}
}

func writeCfg(t *testing.T, home string, body any, mode os.FileMode) string {
	t.Helper()
	dir := filepath.Join(home, ".config", "rclone-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(mustJSON(t, body)), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func testDeps(home string, spawn SpawnFn) Deps {
	n := 0
	return Deps{
		Home:  home,
		Spawn: spawn,
		Now: func() time.Time {
			return time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
		},
		ID: func() (string, error) {
			n++
			return "run-" + numStr(float64(n)), nil
		},
		Cwd:    "/tmp",
		Stdout: &nopBuf{},
		Stderr: &nopBuf{},
	}
}

type nopBuf struct{ buf []byte }

func (n *nopBuf) Write(p []byte) (int, error) {
	n.buf = append(n.buf, p...)
	return len(p), nil
}

func (n *nopBuf) String() string { return string(n.buf) }

type runIO struct {
	code   int
	stdout string
	stderr string
}

func runCLI(t *testing.T, home string, spawn SpawnFn, args ...string) runIO {
	t.Helper()
	deps := testDeps(home, spawn)
	out := &nopBuf{}
	errBuf := &nopBuf{}
	deps.Stdout = out
	deps.Stderr = errBuf
	code := Execute(args, deps)
	return runIO{code: code, stdout: out.String(), stderr: errBuf.String()}
}

func stubSpawn(code int, stderr string) (SpawnFn, *[][]string) {
	copies := [][]string{}
	spawn := func(_ string, args []string, _ ErrSink) (SpawnRet, error) {
		if len(args) > 2 && args[2] == "obscure" {
			plain := args[len(args)-1]
			obscured := base64.RawURLEncoding.EncodeToString([]byte("obscured:" + plain))
			return SpawnRet{Stdout: obscured + "\n"}, nil
		}
		cp := append([]string{}, args...)
		copies = append(copies, cp)
		return SpawnRet{Code: code, Stderr: stderr}, nil
	}
	return spawn, &copies
}

func errText(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	return err.Error()
}
