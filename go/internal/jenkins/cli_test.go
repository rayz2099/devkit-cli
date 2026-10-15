package jenkins

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionSkipsConfig(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(cfg, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	out, err := runCLI("version")
	if err != nil || out != "jenkins-cli 0.1.0\n" {
		t.Fatalf("out %q err %v", out, err)
	}
	if _, statErr := os.Stat(filepath.Join(cfg, "jenkins-cli")); !os.IsNotExist(statErr) {
		t.Fatalf("config dir created: %v", statErr)
	}
}

func TestDoctorMissingAndValid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	dir := filepath.Join(home, ".config", "jenkins-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCLI("doctor")
	if err == nil || ExitCode(err) == 0 {
		t.Fatalf("missing should fail: %v", err)
	}
	if !strings.HasPrefix(out, "fail config: ") || !strings.Contains(out, "config missing:") {
		t.Fatalf("doctor out = %q", out)
	}
	after, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if before.Mode() != after.Mode() {
		t.Fatalf("mode %v -> %v", before.Mode(), after.Mode())
	}

	path := filepath.Join(dir, "config.json")
	body := `{
	  "defaultProfile": "prod",
	  // hand written
	  "profiles": [{"name": "prod", "url": "http://127.0.0.1:9/", "username": "admin", "apiToken": "",}]
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	defer srv.Close()
	out, err = runCLI("doctor")
	if err != nil || out != "pass config\n" {
		t.Fatalf("pass out %q err %v", out, err)
	}
	if hits != 0 {
		t.Fatalf("doctor dialed: %d", hits)
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"defaultProfile":"","profiles":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI("-c", bad, "doctor")
	if err == nil || !strings.Contains(out, "fail config:") || !strings.Contains(out, "defaultProfile is required") {
		t.Fatalf("bad out %q err %v", out, err)
	}
}

func TestHelpAndCompletionDoNotTouchConfig(t *testing.T) {
	setIsolatedHome(t)
	out, err := runCLI()
	if err != nil || out != helpText("") {
		t.Fatalf("help %q %v", out, err)
	}
	out, err = runCLI("completion", "fish")
	if err != nil || out != fishScript() {
		t.Fatalf("fish mismatch %v", err)
	}
	out, err = runCLI("__complete", "")
	if err != nil || out != "job\nrun\nlog\nqueue\ncompletion\nagent\nhuman\n" {
		t.Fatalf("root complete %q %v", out, err)
	}
	out, err = runCLI("__complete", "job", "")
	if err != nil || out != "ls\nview\n" {
		t.Fatalf("job complete %q %v", out, err)
	}
	out, err = runCLI("__complete", "agent", "")
	if err != nil || !strings.Contains(out, "trigger\n") {
		t.Fatalf("agent complete %q %v", out, err)
	}
	out, err = runCLI("__complete", "-c", "")
	if err != nil || out != "" {
		t.Fatalf("config complete %q %v", out, err)
	}
}

func TestProfileCompletion(t *testing.T) {
	home := setIsolatedHome(t)
	path := filepath.Join(home, ".config", "jenkins-cli", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"defaultProfile":"prod","profiles":[{"name":"prod","url":"http://x","username":"u","apiToken":"t"},{"name":"lab","url":"http://y","username":"u","password":"p"}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI("__complete", "-p", "")
	if err != nil || out != "prod\nlab\n" {
		t.Fatalf("profiles %q %v", out, err)
	}
}

func TestAgentAndHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.RawQuery, "jobs["):
			_, _ = w.Write([]byte(`{"jobs":[{"name":"a&b","url":"http://j/a","_class":"hudson.model.FreeStyleProject"}]}`))
		case strings.Contains(r.URL.Path, "/queue/"):
			_, _ = w.Write([]byte(`{"items":[]}`))
		case strings.HasSuffix(r.URL.Path, "/consoleText"):
			_, _ = w.Write([]byte("line"))
		default:
			_, _ = w.Write([]byte(`{"name":"a&b","url":"http://j/a","buildable":true,"lastBuild":{"number":3},"lastSuccessfulBuild":null,"lastFailedBuild":null}`))
		}
	}))
	defer srv.Close()
	cfg := writeCfg(t, srv.URL)
	out, err := runCLI("-c", cfg, "job", "ls")
	if err != nil {
		t.Fatal(err)
	}
	want := "NAME  CLASS           \na&b   FreeStyleProject\n"
	if out != want {
		t.Fatalf("human jobs\n%q\nwant\n%q", out, want)
	}
	out, err = runCLI("-c", cfg, "agent", "job", "ls")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name": "a&b"`) || strings.Contains(out, "\\u0026") {
		t.Fatalf("json = %s", out)
	}
	out, err = runCLI("-c", cfg, "job", "view", "a&b")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "lastBuild\t3\n") || !strings.Contains(out, "lastSuccessfulBuild\t-\n") {
		t.Fatalf("view = %q", out)
	}
	out, err = runCLI("-c", cfg, "agent", "log", "a/b", "1")
	if err != nil || out != "line\n" {
		t.Fatalf("log %q %v", out, err)
	}
	out, err = runCLI("-c", cfg, "queue", "ls")
	if err != nil || out != "Queue is empty\n" {
		t.Fatalf("queue %q %v", out, err)
	}
}

func TestTriggerCancelAndRuns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/crumbIssuer/api/json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(r.URL.RawQuery, "builds[") {
			_, _ = w.Write([]byte(`{"builds":[{"number":2,"result":"FAILURE","building":false,"timestamp":1700000000000,"duration":90000}]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/stop") {
			w.WriteHeader(http.StatusFound)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/api/json") && strings.Contains(r.URL.Path, "/4/") {
			_, _ = w.Write([]byte(`{"number":4,"result":"SUCCESS","url":"http://j/4","actions":[{"causes":[{"shortDescription":"Started by user"}]}],"description":null,"artifacts":[]}`))
			return
		}
		w.Header().Set("Location", "http://j/queue/1")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	cfg := writeCfg(t, srv.URL)
	out, err := runCLI("-c", cfg, "run", "ls", "app/feature")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2") || !strings.Contains(out, "FAILURE") || !strings.Contains(out, "1m30s") {
		t.Fatalf("runs = %q", out)
	}
	if !strings.Contains(out, "2023-11-14T22:13:20.000Z") {
		t.Fatalf("time = %q", out)
	}
	out, err = runCLI("-c", cfg, "run", "view", "app/feature", "4")
	if err != nil {
		t.Fatal(err)
	}
	want := "app/feature #4 SUCCESS\nhttp://j/4\nStarted by user\n"
	if out != want {
		t.Fatalf("view %q", out)
	}
	out, err = runCLI("-c", cfg, "agent", "status", "app/feature", "4")
	if err != nil || !strings.Contains(out, `"result": "SUCCESS"`) || strings.Contains(out, "causes") {
		t.Fatalf("status %s %v", out, err)
	}
	out, err = runCLI("-c", cfg, "run", "start", "app/feature")
	if err != nil || out != "triggered app/feature\nhttp://j/queue/1\n" {
		t.Fatalf("start %q %v", out, err)
	}
	out, err = runCLI("-c", cfg, "agent", "run", "cancel", "app/feature", "4")
	if err != nil || !strings.Contains(out, `"action": "stop"`) {
		t.Fatalf("cancel %s %v", out, err)
	}
	_, err = runCLI("-c", cfg, "run", "start", "app/master")
	if err == nil || ExitCode(err) != 1 {
		t.Fatalf("mainline code %v", err)
	}
}

func TestParamsKeepInsertionOrder(t *testing.T) {
	info := RunInfo{
		RunStatus: RunStatus{Job: "app", Build: 1, Result: "SUCCESS", URL: "http://j"},
		Artifacts: []Artifact{},
		Causes:    []string{"Started"},
	}
	info.Params.set("BRANCH", "feat")
	info.Params.set("A", "1")
	text, err := renderJSON(info)
	if err != nil {
		t.Fatal(err)
	}
	branch := strings.Index(text, `"BRANCH"`)
	later := strings.Index(text, `"A"`)
	if branch < 0 || later < branch {
		t.Fatalf("order\n%s", text)
	}
	if !strings.Contains(text, "{\n    \"BRANCH\": \"feat\",\n    \"A\": \"1\"\n  }") {
		t.Fatalf("params\n%s", text)
	}
}

func TestUnknownCommandExit(t *testing.T) {
	setIsolatedHome(t)
	out, err := runCLI("nope")
	if err == nil || err.Error() != "unknown command: nope" || ExitCode(err) != 1 || out != "" {
		t.Fatalf("out %q err %v", out, err)
	}
}

func runCLI(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := Run(args, &stdout, &stderr)
	return stdout.String(), err
}

func setIsolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	return home
}

func writeCfg(t *testing.T, base string) string {
	t.Helper()
	setIsolatedHome(t)
	path := filepath.Join(t.TempDir(), "jenkins.json")
	body := `{
	  "defaultProfile": "prod",
	  "profiles": [{"name":"prod","url":"` + base + `","username":"admin","apiToken":"token"}]
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
