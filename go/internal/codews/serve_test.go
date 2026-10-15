package codews

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeHTTP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{Root: dir, Files: ListIndex(dir, 0), Watch: "disabled", hub: newHub()}
	srv := httptest.NewServer(rt)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/meta")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(body), `"watch":"disabled"`) {
		t.Fatalf("%d %s", res.StatusCode, body)
	}

	res, err = http.Get(srv.URL + "/api/blob?path=../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 403 || string(body) != "path denied" {
		t.Fatalf("%d %s", res.StatusCode, body)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/raw/a.txt", nil)
	req.Header.Set("Range", "bytes=0-4")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 206 || string(body) != "hello" {
		t.Fatalf("%d %s", res.StatusCode, body)
	}
	if res.Header.Get("content-range") != "bytes 0-4/11" {
		t.Fatal(res.Header.Get("content-range"))
	}

	res, err = http.Get(srv.URL + "/api/watch")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 503 || string(body) != "watch is not active" {
		t.Fatalf("%d %s", res.StatusCode, body)
	}

	res, err = http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(body), "code-ws serve") {
		t.Fatalf("%d %s", res.StatusCode, body[:min(80, len(body))])
	}

	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/index", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 405 || string(body) != "method not allowed" {
		t.Fatalf("%d %s", res.StatusCode, body)
	}
}

func TestConfigCheck(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "projects", "backend", "app")
	tpl := filepath.Join(home, ".config", "code-ws", "templates", "agents", "work")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tpl, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tpl, "AGENTS.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(home, ".config", "code-ws", "config.json")
	yml := "home: \"$HOME/projects\"\nprojects:\n  - repo-name: app\n    group: backend\n"
	if err := os.WriteFile(filepath.Join(home, ".config", "code-ws", "project.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	js := `{
	  "workspaceRoot": "$HOME/ws",
	  "baseBranch": "main",
	  "remote": "origin",
	  "initAgentsTemplate": "work",
	  "profiles": {"default": {"name": "d", "agentsTemplate": "work", "repos": ["app"]}}
	}`
	if err := os.WriteFile(conf, []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	var out strings.Builder
	if err := Run(strings.NewReader(""), &out, io.Discard, []string{"config", "check"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "config ok\n" {
		t.Fatalf("%q", out.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "code-ws", ".code-ws-root")); !os.IsNotExist(err) {
		t.Fatal("wrote a module marker")
	}
}
