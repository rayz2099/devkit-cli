package dsn

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionDoesNotReadConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	out, errOut, code := runCLI(t, "version")
	if code != 0 || out != "dsn-cli 0.1.0\n" || errOut != "" {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
	if _, statErr := os.Stat(filepath.Join(home, "cfg")); !os.IsNotExist(statErr) {
		t.Fatal("version created config", statErr)
	}
}

func TestDoctorConfigOnly(t *testing.T) {
	home := t.TempDir()
	cfgRoot := filepath.Join(home, "cfg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfgRoot)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	out, _, code := runCLI(t, "doctor")
	if code == 0 || !strings.Contains(out, "fail config:") || !strings.Contains(out, "config missing") {
		t.Fatalf("missing code=%d out=%q", code, out)
	}
	if _, statErr := os.Stat(filepath.Join(cfgRoot, "dsn-cli")); !os.IsNotExist(statErr) {
		t.Fatal("doctor created config dir")
	}

	path := filepath.Join(cfgRoot, "dsn-cli", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("{\n  \"profiles\": [\n    {\n      \"name\": \"buy\",\n      \"kind\": \"mysql\",\n      \"url\": \"mysql://readonly@127.0.0.1:1/buy\",\n    },\n    {\n      \"name\": \"bad\",\n      \"kind\": \"oracle\",\n      \"url\": \"oracle://127.0.0.1/db\",\n    }\n  ]\n}\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t, "doctor")
	if code == 0 || !strings.Contains(out, "pass config\n") || !strings.Contains(out, "pass buy\n") || !strings.Contains(out, "fail bad:") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
	if strings.Contains(out, "oracle://") || strings.Contains(errOut, "doctor failed") {
		t.Fatalf("leaked out=%q err=%q", out, errOut)
	}

	out, _, code = runCLI(t, "doctor", "--output", "xml")
	if code == 0 || !strings.Contains(out, "invalid --output") && !strings.Contains(runErr(t, "doctor", "--output", "xml"), "invalid --output") {
		t.Fatal(out, code)
	}
}

func TestQueryGateAndDS(t *testing.T) {
	home := t.TempDir()
	cfgRoot := filepath.Join(home, "cfg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfgRoot)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	path := filepath.Join(cfgRoot, "dsn-cli", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"profiles":[{"name":"buy","kind":"mysql","url":"mysql://secret@127.0.0.1:1/buy","description":"交易库只读"}]}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t, "-p", "buy", "query", "INSERT INTO t VALUES (1)")
	if code != 2 || !strings.Contains(errOut, "INSERT") || out != "" {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
	out, errOut, code = runCLI(t, "ds")
	if code != 0 || strings.Contains(out, "secret") || !strings.Contains(out, "buy") || !strings.Contains(out, "交易库只读") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
	out, _, code = runCLI(t, "-h", "ds")
	if code != 0 || strings.Contains(out, "secret") || !strings.Contains(out, "mysql") {
		t.Fatalf("help ds code=%d out=%q", code, out)
	}
	_, errOut, code = runCLI(t, "-p", "buy")
	if code == 0 || !strings.Contains(errOut, "non-TTY requires query") {
		t.Fatalf("console code=%d err=%q", code, errOut)
	}
	_, errOut, code = runCLI(t, "agent", "-p", "buy")
	if code == 0 || !strings.Contains(errOut, "agent cannot enter Console") {
		t.Fatalf("agent console code=%d err=%q", code, errOut)
	}
	_, errOut, code = runCLI(t, "nope")
	if code == 0 || !strings.Contains(errOut, "unknown command: nope") {
		t.Fatalf("unknown code=%d err=%q", code, errOut)
	}
	out, _, code = runCLI(t, "completion", "fish")
	if code != 0 || !strings.Contains(out, "__complete") {
		t.Fatalf("fish code=%d out=%q", code, out)
	}
	out, _, code = runCLI(t, "__complete", "")
	if code != 0 || !strings.Contains(out, "doctor\n") {
		t.Fatalf("complete code=%d out=%q", code, out)
	}
}

func TestESQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cluster_name":"local"}`))
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	path := filepath.Join(home, "cfg", "dsn-cli", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"profiles":[{"name":"es","kind":"elasticsearch","url":"` + srv.URL + `"}]}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t, "-p", "es", "query", "GET /")
	if code != 0 || !strings.Contains(out, "local") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestTopicCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	if err := writeTopicCache("kf-biz", []string{"album_audit_log", "blog_core_1"}); err != nil {
		t.Fatal(err)
	}
	got, err := readTopicCache("kf-biz")
	if err != nil || strings.Join(got, ",") != "album_audit_log,blog_core_1" {
		t.Fatal(got, err)
	}
	got, err = readTopicCache("other")
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	if _, statErr := os.Stat(filepath.Join(home, "cfg")); !os.IsNotExist(statErr) {
		t.Fatal("cache write touched config", statErr)
	}

	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cfg", "nested"))
	if _, err := topicCachePath("kf-biz"); err == nil {
		t.Fatal("cache inside config should fail")
	}
}

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Execute(args, IO{Out: &out, Err: &errOut, TTY: false})
	return out.String(), errOut.String(), code
}

func runErr(t *testing.T, args ...string) string {
	t.Helper()
	_, errOut, _ := runCLI(t, args...)
	return errOut
}
