package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.internal.linran.top/linran/devkit-cli/internal/olly"
)

func TestHelpVersionDoctor(t *testing.T) {
	home := isolate(t)
	out, err := run(nil, "--help")
	if err != nil || out != olly.RootHelp {
		t.Fatalf("help err=%v\n%s", err, out)
	}
	out, err = run(nil)
	if err != nil || out != olly.RootHelp {
		t.Fatalf("empty err=%v", err)
	}
	out, err = run(nil, "help")
	if err != nil || out != olly.RootHelp {
		t.Fatalf("help cmd %v %q", err, out)
	}
	out, err = run(nil, "uptrace", "-h")
	if err != nil || out != olly.UptraceHelp {
		t.Fatalf("uptrace help %v\n%s", err, out)
	}
	out, err = run(nil, "prom", "--help")
	if err != nil || out != olly.PrometheusHelp {
		t.Fatalf("prom help %v", err)
	}
	out, err = run(nil, "logs", "help")
	if err != nil || out != olly.GraylogHelp {
		t.Fatalf("logs help %v\n%s", err, out)
	}
	out, err = run(nil, "grafana", "--help")
	if err != nil || out != olly.GrafanaHelp {
		t.Fatalf("grafana help %v", err)
	}
	out, err = run(nil, "version")
	if err != nil || out != "olly-cli 0.1.0\n" {
		t.Fatalf("version %v %q", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(statErr) {
		t.Fatal("version touched config", statErr)
	}
	_, err = run(nil, "doctor")
	if err == nil {
		t.Fatal("missing config should fail")
	}
	comp, err := run(nil, "__complete", "prometheus", "")
	if err != nil || !strings.Contains(comp, "query") || !strings.Contains(comp, "ready") {
		t.Fatalf("complete %v %q", err, comp)
	}
}

func TestPromZeroConfig(t *testing.T) {
	isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"job":"prometheus"},"value":[1,"1"]}]}}`))
	}))
	defer srv.Close()
	out, err := run(nil, "--output", "agent", "prom", "query", "instant", "--base-url", srv.URL, "up")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"command": "query instant"`) || !strings.Contains(out, `"job": "prometheus"`) {
		t.Fatal(out)
	}
	if _, err := run(nil, "-c", filepath.Join(t.TempDir(), "missing.json"), "prom", "query", "instant", "up"); err == nil || !strings.Contains(err.Error(), "config file not found") {
		t.Fatal(err)
	}
}

func TestGrayAndGrafanaCommands(t *testing.T) {
	home := isolate(t)
	var grayBody string
	gray := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		grayBody = r.URL.RequestURI()
		if r.Header.Get("X-Requested-By") != "olly-cli" || !strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
			t.Errorf("headers %v", r.Header)
		}
		w.Write([]byte(`{"total_results":0,"messages":[],"fields":[]}`))
	}))
	defer gray.Close()
	cfg := filepath.Join(home, "olly.json")
	os.WriteFile(cfg, []byte(`{"graylog":{"base_url":"`+gray.URL+`","username":"admin","password":"admin"},"grafana":{"base_url":"http://127.0.0.1:9","username":"admin","password":"x"}}`), 0o644)
	out, err := run(nil, "-c", cfg, "graylog", "app:billing", "--relative", "10", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No logs.") || !strings.Contains(grayBody, "query=app%3Abilling") {
		t.Fatalf("%s\n%s", out, grayBody)
	}
	graf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/api/dashboards/uid/") {
			if r.Header.Get("X-Grafana-Org-Id") != "1" {
				t.Errorf("org %s", r.Header.Get("X-Grafana-Org-Id"))
			}
			w.Write([]byte(`{"meta":{"url":"/d/uid/biz"},"dashboard":{"title":"业务接口","uid":"uid","time":{"from":"now-6h","to":"now"},"panels":[{"id":1,"type":"timeseries","title":"p","datasource":{"type":"prometheus","uid":"prom"},"targets":[{"refId":"A","expr":"up","range":true,"datasource":{"type":"prometheus","uid":"prom"}}]}]}}`))
			return
		}
		w.Write([]byte(`{"results":{"A":{"status":200,"frames":[{"schema":{"name":"up","fields":[{"name":"Time","type":"time"},{"name":"Value","type":"number","labels":{"job":"a"},"config":{"displayNameFromDS":"up"}}]},"data":{"values":[[1],[2]]}}]}}}`))
	}))
	defer graf.Close()
	os.WriteFile(cfg, []byte(`{"grafana":{"base_url":"`+graf.URL+`","username":"admin","password":"secret"}}`), 0o644)
	out, err = run(nil, "-f", cfg, "--output", "agent", "grafana", "https://grafana.example.com/d/uid/biz?orgId=1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"title": "业务接口"`) || !strings.Contains(out, "https://grafana.example.com/d/uid/biz?orgId=1&from=now-6h&to=now") {
		t.Fatal(out)
	}
}

func TestUptraceCommand(t *testing.T) {
	home := isolate(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "token=jwt" {
			t.Errorf("auth %v", r.Header)
		}
		w.Write([]byte(`{"root":{"id":"1","name":"GET /","kind":"server","duration":10,"time":"2026-04-29T06:00:00.000Z","children":[]}}`))
	}))
	defer up.Close()
	cfg := filepath.Join(home, "olly.json")
	os.WriteFile(cfg, []byte(`{"uptrace":{"base_url":"`+up.URL+`","project_id":2,"jwt_token":"jwt"}}`), 0o644)
	out, err := run(nil, "-c", cfg, "uptrace", "trace", "e4f1e0bcd6a1ac296661a3e6ea5507c9")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Trace: e4f1e0bcd6a1ac296661a3e6ea5507c9") || !strings.Contains(out, `name="GET /"`) {
		t.Fatal(out)
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

func run(t *testing.T, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := execute(args, &stdout, &stderr)
	if err != nil {
		return stdout.String(), err
	}
	return stdout.String(), nil
}
