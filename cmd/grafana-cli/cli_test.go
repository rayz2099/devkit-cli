package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionDoctorProfiles(t *testing.T) {
	home := isolate(t)
	out, errText, err := run(nil, "version")
	if err != nil || out != "grafana-cli 0.1.0\n" || errText != "" {
		t.Fatalf("version %v %q %q", err, out, errText)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(statErr) {
		t.Fatal("version touched config", statErr)
	}
	out, _, err = run(nil, "doctor")
	if err == nil || !strings.Contains(out, "fail config:") {
		t.Fatalf("doctor %v %s", err, out)
	}

	cfg := writeCfg(t, home)
	out, _, err = run(nil, "-c", cfg, "doctor")
	if err != nil || !strings.Contains(out, "pass config") || !strings.Contains(out, "pass profile ops") {
		t.Fatalf("doctor ok %v %s", err, out)
	}
	out, _, err = run(nil, "-c", cfg, "profiles")
	if err != nil || strings.Contains(out, "secret") || !strings.Contains(out, "ops\thttp://127.0.0.1:9") {
		t.Fatalf("profiles %v %s", err, out)
	}
	out, _, err = run(nil, "-c", cfg, "agent", "profiles")
	if err != nil || strings.Contains(out, "secret") || !strings.Contains(out, `"active_profile": "ops"`) {
		t.Fatalf("agent profiles %v %s", err, out)
	}
}

func TestEdit(t *testing.T) {
	home := isolate(t)
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.Header.Get("X-Grafana-Org-Id") != "1" {
			t.Errorf("org %s", r.Header.Get("X-Grafana-Org-Id"))
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
			t.Errorf("auth %s", r.Header.Get("Authorization"))
		}
		if r.Method == http.MethodGet {
			w.Write([]byte(liveDash))
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["overwrite"] != true || body["folderUid"] != "fold-1" || body["message"] != "grafana-cli edit" {
			t.Fatalf("save %#v", body)
		}
		dash, _ := body["dashboard"].(map[string]any)
		if dash["uid"] != "kt-safety-workers" || dash["version"] != float64(4) {
			t.Fatalf("dash %#v", dash)
		}
		w.Write([]byte(`{"uid":"kt-safety-workers","url":"/d/kt-safety-workers/probe","status":"success","version":5}`))
	}))
	defer srv.Close()
	cfg := writeCfg(t, home)
	replace := strings.Replace(mustRead(t, cfg), "http://127.0.0.1:9", srv.URL, 1)
	os.WriteFile(cfg, []byte(replace), 0o644)
	src := "https://grafana.dtactivity.cn/d/kt-safety-workers/kt-safety-workers?orgId=1&refresh=1m"

	out, _, err := run(nil, "-c", cfg, "edit", src)
	if err != nil || !strings.Contains(out, "title: kt-safety / workers") || !strings.Contains(out, "version: 4") || !strings.Contains(out, "panels: 2") || !strings.Contains(out, "refresh=1m") {
		t.Fatalf("human read %v %s", err, out)
	}
	out, _, err = run(nil, "-c", cfg, "agent", "edit", src)
	if err != nil || !strings.Contains(out, `"uid": "kt-safety-workers"`) || strings.Contains(out, `"ok"`) {
		t.Fatalf("agent read %v %s", err, out)
	}

	file := filepath.Join(home, "dash.json")
	os.WriteFile(file, []byte(out), 0o644)
	_, errText, err := run(nil, "-c", cfg, "agent", "edit", src, "--file", file)
	if err == nil || !strings.Contains(errText, `"type": "exists"`) || strings.Contains(strings.Join(calls, ","), "POST") {
		t.Fatalf("exists %v %s calls=%v", err, errText, calls)
	}

	stale := strings.Replace(out, `"version": 4`, `"version": 3`, 1)
	os.WriteFile(file, []byte(stale), 0o644)
	_, errText, err = run(nil, "-c", cfg, "edit", src, "--file", file, "--overwrite")
	if err == nil || !strings.Contains(err.Error(), "does not match server version 4") || strings.Contains(strings.Join(calls, ","), "POST") {
		t.Fatalf("version %v %s", err, errText)
	}

	os.WriteFile(file, []byte(out), 0o644)
	text, _, err := run(nil, "-c", cfg, "agent", "edit", src, "--file", file, "--overwrite")
	if err != nil || !strings.Contains(text, `"version": 5`) || !strings.Contains(text, `"status": "updated"`) || !strings.Contains(text, "/probe?orgId=1") {
		t.Fatalf("write %v %s", err, text)
	}
	if !strings.Contains(strings.Join(calls, ","), "POST /api/dashboards/db") {
		t.Fatal(calls)
	}
}

func TestEditRejectsMismatchedUID(t *testing.T) {
	home := isolate(t)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	cfg := writeCfg(t, home)
	os.WriteFile(cfg, []byte(strings.Replace(mustRead(t, cfg), "http://127.0.0.1:9", srv.URL, 1)), 0o644)
	body := bytes.NewReader([]byte(`{"uid":"other","title":"x","version":1}`))
	_, errText, err := run(body, "-c", cfg, "edit", "kt-safety-workers")
	if err == nil || hits != 0 || !strings.Contains(errText, "does not match url uid") {
		t.Fatalf("mismatch %v %s hits=%d", err, errText, hits)
	}
}

func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	return home
}

func writeCfg(t *testing.T, home string) string {
	t.Helper()
	path := filepath.Join(home, "config.json")
	text := `{
  "default_profile": "ops",
  "profiles": {
    "ops": {
      "base_url": "http://127.0.0.1:9",
      "web_base_url": "https://grafana.dtactivity.cn",
      "username": "admin",
      "password": "secret"
    }
  }
}`
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(buf)
}

func run(stdin *bytes.Reader, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	var in interface{ Read([]byte) (int, error) }
	if stdin != nil {
		in = stdin
	}
	err := execute(args, in, &stdout, &stderr)
	if err != nil && stderr.Len() == 0 {
		stderr.WriteString(err.Error())
	}
	return stdout.String(), stderr.String(), err
}

const liveDash = `{
  "meta": {"url": "/d/kt-safety-workers/kt-safety-workers", "folderUid": "fold-1"},
  "dashboard": {
    "uid": "kt-safety-workers",
    "title": "kt-safety / workers",
    "version": 4,
    "panels": [
      {"id": 1, "type": "stat"},
      {"id": 2, "type": "row", "panels": [{"id": 3, "type": "graph"}]}
    ]
  }
}`
