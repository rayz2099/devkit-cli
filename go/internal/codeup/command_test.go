package codeup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionSkipsConfig(t *testing.T) {
	setHome(t)
	stdout, stderr, code := runCLI(t, "version")
	if code != 0 || stdout != "codeup-cli 0.1.0\n" || stderr != "" {
		t.Fatalf("version code=%d out=%q err=%q", code, stdout, stderr)
	}
}

func TestDoctorMissingAndValid(t *testing.T) {
	home := setHome(t)
	stdout, stderr, code := runCLI(t, "doctor")
	if code == 0 || stderr != "" || !contains(stdout, "fail config:") || !contains(stdout, "config missing") {
		t.Fatalf("missing doctor code=%d out=%q err=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(err) {
		t.Fatalf("doctor created config: %v", err)
	}
	cfg := filepath.Join(home, "ok.json")
	body := []byte(`{
  "defaultProfile": "work",
  "profiles": [{ "name": "work", "url": "https://codeup.aliyun.com/org-example", "token": "secret-token" }]
}`)
	if err := os.WriteFile(cfg, body, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runCLI(t, "-c", cfg, "doctor")
	if code != 0 || stdout != "pass config\n" || stderr != "" {
		t.Fatalf("ok doctor code=%d out=%q err=%q", code, stdout, stderr)
	}
	if contains(stdout, "secret-token") {
		t.Fatal("doctor printed token")
	}
	bad := filepath.Join(home, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code = runCLI(t, "doctor", "-c", bad)
	if code == 0 || !contains(stdout, "fail config:") || !contains(stdout, "invalid config file") {
		t.Fatalf("bad doctor code=%d out=%q", code, stdout)
	}
}

func TestHelpAndCompletionDoNotWriteConfig(t *testing.T) {
	home := setHome(t)
	stdout, stderr, code := runCLI(t)
	if code != 0 || !contains(stdout, "codeup-cli") || stderr != "" {
		t.Fatalf("help code=%d out=%q err=%q", code, stdout, stderr)
	}
	stdout, _, code = runCLI(t, "completion", "fish")
	if code != 0 || !contains(stdout, "__complete") || !contains(stdout, "show webhook secretToken") {
		t.Fatalf("fish = %q %d", stdout, code)
	}
	stdout, stderr, code = runCLI(t, "__complete", "")
	if code != 0 || stderr != "" || !contains(stdout, "init\n") || contains(stdout, ":") {
		t.Fatalf("complete code=%d out=%q err=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(err) {
		t.Fatalf("completion wrote config: %v", err)
	}
	_, stderr, code = runCLI(t, "completion", "bash")
	if code == 0 || !contains(stderr, "usage: codeup-cli completion fish") {
		t.Fatalf("bash code=%d err=%q", code, stderr)
	}
}

func TestUsageExitCodes(t *testing.T) {
	setHome(t)
	_, stderr, code := runCLI(t, "nope")
	if code != 1 || !contains(stderr, "unknown command: nope") {
		t.Fatalf("unknown code=%d err=%q", code, stderr)
	}
	_, stderr, code = runCLI(t, "cr", "merge", "4", "--type", "ours")
	if code != 1 || !contains(stderr, "unknown --type ours") {
		t.Fatalf("type code=%d err=%q", code, stderr)
	}
}

func TestPushRejectsForeignRemote(t *testing.T) {
	home := setHome(t)
	logPath := filepath.Join(home, "git.log")
	writeFakeGit(t, logPath)
	t.Setenv("GIT_LOG", logPath)
	t.Setenv("GIT_REMOTE", "git@github.com:org/repo.git")
	t.Setenv("GIT_BRANCH", "feature/x")
	cfg := writeCfg(t, home)
	_, stderr, code := runCLI(t, "-c", cfg, "push", "feature/x")
	if code != 1 || !contains(stderr, "remote origin is not codeup.aliyun.com") {
		t.Fatalf("push code=%d err=%q", code, stderr)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(log), "push ") {
		t.Fatalf("push ran: %s", log)
	}
}

func TestPushUsesCurrentBranch(t *testing.T) {
	home := setHome(t)
	logPath := filepath.Join(home, "git.log")
	writeFakeGit(t, logPath)
	t.Setenv("GIT_LOG", logPath)
	t.Setenv("GIT_REMOTE", "git@codeup.aliyun.com:org-example/group/demo.git")
	t.Setenv("GIT_BRANCH", "feature/x")
	cfg := writeCfg(t, home)
	stdout, stderr, code := runCLI(t, "-c", cfg, "agent", "push")
	if code != 0 || stderr != "" {
		t.Fatalf("push code=%d out=%q err=%q", code, stdout, stderr)
	}
	if !contains(stdout, `"remote": "origin"`) || !contains(stdout, `"branch": "feature/x"`) || !contains(stdout, `"pushed": true`) {
		t.Fatalf("json = %s", stdout)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(log), "push origin feature/x") {
		t.Fatalf("log = %s", log)
	}
}

func TestCompleteProfilesFromConfig(t *testing.T) {
	home := setHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".config", "codeup-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{
  "defaultProfile": "work",
  "profiles": [{ "name": "work", "url": "https://codeup.aliyun.com/org-example", "token": "tok" }]
}`)
	path := filepath.Join(home, ".config", "codeup-cli", "config.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCLI(t, "__complete", "-p", "")
	if code != 0 || stdout != "work\n" || stderr != "" {
		t.Fatalf("profiles code=%d out=%q err=%q", code, stdout, stderr)
	}
}

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func writeCfg(t *testing.T, home string) string {
	t.Helper()
	path := filepath.Join(home, "codeup.json")
	body := []byte(`{
  "defaultProfile": "work",
  "profiles": [{ "name": "work", "url": "https://codeup.aliyun.com/org-example", "token": "tok" }]
}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFakeGit(t *testing.T, logPath string) {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/bash
printf '%s\n' "$*" >> "$GIT_LOG"
if [ -n "$GIT_FAIL" ]; then
  echo "fatal: boom" >&2
  exit 1
fi
case "$1" in
remote)
  printf '%s\n' "$GIT_REMOTE"
  ;;
rev-parse)
  printf '%s\n' "$GIT_BRANCH"
  ;;
push)
  exit 0
  ;;
*)
  echo "bad $*" >&2
  exit 1
  ;;
esac
`
	path := filepath.Join(dir, "git")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	_ = logPath
}
