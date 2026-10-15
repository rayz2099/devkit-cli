package har

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteMatchesFixtureFilters(t *testing.T) {
	fix := filepath.Join("..", "..", "..", "har-cli", "test", "fixtures", "sample.har")

	code, stdout, stderr := run(t, fix, "-f", "http.status == 200 and body.status == 1")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	want := strings.Join([]string{
		"[Request]",
		"POST /api/order/create?trace=1 HTTP/1.1",
		"Host: example.com",
		"Content-Type: application/json",
		"",
		`{"trace":1}`,
		"",
		"[Response]",
		"HTTP/1.1 200 OK",
		"Server: Tengine",
		"Content-Type: application/json;charset=UTF-8",
		"",
		`{"status":1}`,
		"",
	}, "\n")
	if stdout != want {
		t.Fatalf("stdout=%q", stdout)
	}

	code, stdout, stderr = run(t, fix, "-f", "body.status == 1 and http.status == 403", "-s", "path")
	if code != 0 || stderr != "" || stdout != "/api/admin/report\n" {
		t.Fatalf("path code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	code, stdout, stderr = run(t, fix, "-s", "path")
	if code != 0 || stdout != "/api/user/info\n/api/order/create\n/api/admin/report\n" {
		t.Fatalf("paths stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = run(t, fix, "-s", "prefix", "-p", "2")
	if code != 0 || stdout != "/api/user/*\n/api/order/*\n/api/admin/*\n" {
		t.Fatalf("prefix stdout=%q code=%d", stdout, code)
	}

	code, stdout, stderr = run(t, fix, "-f", "http.status == 200 and body.status == 1", "-s", "")
	if code != 0 || !strings.Contains(stdout, "POST /api/order/create?trace=1 HTTP/1.1") {
		t.Fatalf("empty select stdout=%q code=%d", stdout, code)
	}

	code, stdout, stderr = run(t, fix, "-f", "body.status == 1 and http.status == 403", "-s", "path,http.status")
	if code != 0 || stderr != "" {
		t.Fatalf("fields code=%d stderr=%q", code, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &got); err != nil {
		t.Fatalf("fields json: %v stdout=%q", err, stdout)
	}
	if got["path"] != "/api/admin/report" {
		t.Fatalf("path=%v", got["path"])
	}
	http, _ := got["http"].(map[string]any)
	if http["status"] != float64(403) {
		t.Fatalf("status=%v", http["status"])
	}
	if stdout != "{\"path\":\"/api/admin/report\",\"http\":{\"status\":403}}\n" {
		t.Fatalf("fields text=%q", stdout)
	}
}

func TestNoFilterKeepsNonMatchingBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "only-ok.har")
	body := `{"log":{"entries":[{"request":{"method":"GET","url":"https://example.com/api/health","httpVersion":"HTTP/1.1","headers":[]},"response":{"status":200,"statusText":"OK","httpVersion":"HTTP/1.1","headers":[],"content":{"mimeType":"application/json","text":"{\"status\":0}"}}}]}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, path, "-s", "path")
	if code != 0 || stderr != "" || stdout != "/api/health\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestNoDedupeAndJSON(t *testing.T) {
	fix := filepath.Join("..", "..", "..", "har-cli", "test", "fixtures", "sample.har")
	code, stdout, stderr := run(t, fix, "-s", "path", "--no-dedupe")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if stdout != "/api/user/info\n/api/order/create\n/api/admin/report\n/api/user/info\n" {
		t.Fatalf("stdout=%q", stdout)
	}

	code, stdout, stderr = run(t, fix, "-o", "json", "-s", "path")
	want := "[\n  {\n    \"path\": \"/api/user/info\"\n  },\n  {\n    \"path\": \"/api/order/create\"\n  },\n  {\n    \"path\": \"/api/admin/report\"\n  }\n]\n"
	if code != 0 || stdout != want {
		t.Fatalf("json code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestHelpCompletionsVersionDoctor(t *testing.T) {
	code, stdout, stderr := run(t)
	if code != 0 || stderr != "" || stdout != Help() {
		t.Fatalf("help code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = run(t, "help")
	if code != 0 || !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, "--filter") || strings.Contains(stdout, "--where") || !strings.Contains(stdout, "-s") {
		t.Fatalf("help text=%q", stdout)
	}
	code, stdout, stderr = run(t, "file.har", "--help")
	if code != 0 || stdout != Help() {
		t.Fatalf("flag help code=%d stdout=%q", code, stdout)
	}

	code, stdout, stderr = run(t, "completions", "fish")
	if code != 0 || stderr != "" || stdout != Fish()+"\n" {
		t.Fatalf("fish code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "complete -c har-cli") || !strings.Contains(stdout, "-l filter") || !strings.Contains(stdout, "-s f") {
		t.Fatalf("fish script missing flags: %q", stdout)
	}
	code, _, stderr = run(t, "completions", "bash")
	if code != 1 || stderr != "Only fish completion is supported.\n" {
		t.Fatalf("bash code=%d stderr=%q", code, stderr)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	code, stdout, stderr = run(t, "version")
	if code != 0 || stdout != "har-cli 0.1.0\n" || stderr != "" {
		t.Fatalf("version code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = run(t, "doctor")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("doctor code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, "config")); !os.IsNotExist(err) {
		t.Fatalf("doctor touched config: %v", err)
	}
}

func TestScanErrors(t *testing.T) {
	cases := []struct {
		args []string
		msg  string
	}{
		{[]string{"-c", "x"}, "Unknown option: -c\n"},
		{[]string{"--filter"}, "Missing value for --filter\n"},
		{[]string{"-o", "yaml", "a.har"}, "Unsupported format: yaml\n"},
		{[]string{"-p", "0", "a.har"}, "Invalid value for -p: 0\n"},
		{nil, ""},
	}
	code, stdout, stderr := run(t)
	if code != 0 || stdout == "" {
		t.Fatalf("empty should be help, code=%d stdout=%q", code, stdout)
	}
	for _, tc := range cases {
		if tc.args == nil {
			continue
		}
		code, stdout, stderr = run(t, tc.args...)
		if code != 1 || stdout != "" || stderr != tc.msg {
			t.Fatalf("args=%v code=%d stdout=%q stderr=%q", tc.args, code, stdout, stderr)
		}
	}
	code, stdout, stderr = run(t, "-f", "http.status == 1")
	if code != 1 || stderr != "At least one HAR file is required.\n" {
		t.Fatalf("no file stderr=%q code=%d", stderr, code)
	}
}

func TestBase64BodyAndTwoFiles(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.har")
	second := filepath.Join(dir, "b.har")
	// "eyJzdGF0dXMiOjF9" is {"status":1}
	writeHAR(t, first, "GET", "https://example.com/api/a", 200, "eyJzdGF0dXMiOjF9", "base64")
	writeHAR(t, second, "GET", "https://example.com/api/b", 403, `{"status":1}`, "")
	code, stdout, stderr := run(t, first, second, "-f", "body.status == 1", "-s", "path")
	if code != 0 || stderr != "" || stdout != "/api/a\n/api/b\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestFishCompletionListsHar(t *testing.T) {
	fish, err := exec.LookPath("fish")
	if err != nil {
		t.Skip("fish not installed")
	}
	dir := filepath.Join("..", "..", "..", "har-cli", "test", "fixtures")
	script := filepath.Join(t.TempDir(), "har-cli.fish")
	if err := os.WriteFile(script, []byte(Fish()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(fish, "--no-config", "-c", "source "+quoteFish(script)+"; complete --do-complete \"har-cli sam\"")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("fish: %v", err)
	}
	if !strings.Contains(string(out), "sample.har") {
		t.Fatalf("completion=%q", out)
	}
}

func TestMissingFileAndDirtyBase64(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.har")
	code, stdout, stderr := run(t, missing)
	want := "ENOENT: no such file or directory, open '" + missing + "'\n"
	if code != 1 || stdout != "" || stderr != want {
		t.Fatalf("missing code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "dirty.har")
	writeHAR(t, path, "GET", "https://example.com/api/dirty", 200, "eyJz*dGF0dXMiOjF9", "base64")
	code, stdout, stderr = run(t, path, "-f", "body.status == 1", "-s", "path")
	if code != 0 || stderr != "" || stdout != "/api/dirty\n" {
		t.Fatalf("dirty code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func quoteFish(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

func writeHAR(t *testing.T, path, method, rawURL string, status int, text, enc string) {
	t.Helper()
	content := map[string]any{"mimeType": "application/json", "text": text}
	if enc != "" {
		content["encoding"] = enc
	}
	doc := map[string]any{
		"log": map[string]any{
			"entries": []any{
				map[string]any{
					"request": map[string]any{
						"method":      method,
						"url":         rawURL,
						"httpVersion": "HTTP/1.1",
						"headers":     []any{},
					},
					"response": map[string]any{
						"status":      status,
						"statusText":  "OK",
						"httpVersion": "HTTP/1.1",
						"headers":     []any{},
						"content":     content,
					},
				},
			},
		},
	}
	buf, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Execute(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}
