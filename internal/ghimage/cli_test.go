package ghimage

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	return home
}

func gitRepo(t *testing.T, images string) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "-b", "main")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if images != "" {
		path := filepath.Join(dir, "images.yaml")
		if err := os.WriteFile(path, []byte(images), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	return dir
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return runWith(t, defaultRT(), args...)
}

func runWith(t *testing.T, rt runtime, args ...string) (int, string, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runCLI(args, &stdout, &stderr, rt)
	return code, stdout.String(), stderr.String()
}

func TestHelp(t *testing.T) {
	isolate(t)
	want := usageText + "\n"
	for _, args := range [][]string{nil, {"-h"}, {"--help"}, {"help"}, {"help", "add"}} {
		code, stdout, stderr := run(t, args...)
		if code != 0 || stdout != want || stderr != "" {
			t.Fatalf("args %v code %d\n%s\n%s", args, code, stdout, stderr)
		}
	}
}

func TestUnknown(t *testing.T) {
	isolate(t)
	code, stdout, stderr := run(t, "nope")
	if code != 2 || stdout != "" {
		t.Fatalf("code %d stdout %q", code, stdout)
	}
	want := "gh-image-cli: unknown command: nope\n" + usageText + "\n"
	if stderr != want {
		t.Fatalf("stderr:\n%s", stderr)
	}
}

func TestVersionSkipsConfig(t *testing.T) {
	home := isolate(t)
	code, stdout, stderr := run(t, "version", "-c", filepath.Join(home, "missing.json"))
	if code != 0 || stdout != "gh-image-cli 0.1.0\n" || stderr != "" {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(err) {
		t.Fatalf("config dir created: %v", err)
	}
}

func TestDoctorMissing(t *testing.T) {
	home := isolate(t)
	code, stdout, stderr := run(t, "doctor")
	if code != 1 || stderr != "" {
		t.Fatalf("code %d stderr %q stdout %q", code, stderr, stdout)
	}
	if !strings.HasPrefix(stdout, "fail config: ") || !strings.Contains(stdout, "config missing") {
		t.Fatalf("stdout %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(err) {
		t.Fatalf("config dir created: %v", err)
	}
}

func TestDoctorValidAndInvalid(t *testing.T) {
	home := isolate(t)
	dir := filepath.Join(home, ".config", "gh-image-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"timeoutSeconds":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "doctor")
	if code != 1 || stderr != "" || !strings.Contains(stdout, "timeoutSeconds must be a positive integer") {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	if err := os.WriteFile(path, []byte(`{"namespace":"custom"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run(t, "doctor")
	if code != 0 || stdout != "pass config\n" || stderr != "" {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	if err := os.WriteFile(path, []byte(`{"registry":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = run(t, "doctor")
	if code != 1 || !strings.Contains(stdout, "registry is required") {
		t.Fatalf("code %d stdout %q", code, stdout)
	}
}

func TestConfigPath(t *testing.T) {
	home := isolate(t)
	dir := gitRepo(t, "images:\n  cpa:\n    type: mirror\n    source: postgres\n    versions:\n      latest:\n        status: done\n")
	_ = dir
	xdg := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	cfgDir := filepath.Join(xdg, "gh-image-cli")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"namespace":"from-xdg"}`)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_IMAGE_NAMESPACE", "from-env")
	code, stdout, stderr := run(t, "list")
	if code != 0 || stderr != "" {
		t.Fatalf("code %d %s", code, stderr)
	}
	if !strings.Contains(stdout, "registry.cn-shanghai.aliyuncs.com/from-xdg/cpa") {
		t.Fatalf("stdout %s", stdout)
	}
	code, _, stderr = run(t, "list", "-c", filepath.Join(home, "nope.json"))
	if code != 2 || !strings.Contains(stderr, "config file not found:") {
		t.Fatalf("code %d %s", code, stderr)
	}
	explicit := filepath.Join(home, "alt.json")
	if err := os.WriteFile(explicit, []byte(`{"namespace":"alt","registry":"r.example"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run(t, "list", "cpa", "--config", explicit)
	if code != 0 || stderr != "" {
		t.Fatalf("code %d %s", code, stderr)
	}
	want := "ALIAS\tTYPE\tSOURCE/SCRIPT\tTARGET\ncpa\tmirror\tpostgres\tr.example/alt/cpa\n"
	if stdout != want {
		t.Fatalf("stdout %q", stdout)
	}
}

func TestCacheInsideConfig(t *testing.T) {
	home := isolate(t)
	cfg := filepath.Join(home, ".config")
	t.Setenv("XDG_CACHE_HOME", filepath.Join(cfg, "cache"))
	gitRepo(t, "images: {}\n")
	code, _, stderr := run(t, "list")
	if code != 2 || !strings.Contains(stderr, "refusing path inside config dir") {
		t.Fatalf("code %d %s", code, stderr)
	}
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Fatalf("config dir created: %v", err)
	}
}

func TestAddAndList(t *testing.T) {
	isolate(t)
	dir := gitRepo(t, "images: {}\n")
	code, stdout, stderr := run(t, "add", "ghcr.io/sagernet/sing-box:latest", "sing-box")
	if code != 0 || stderr != "" {
		t.Fatalf("code %d %s", code, stderr)
	}
	if stdout != "added: sing-box:latest <- ghcr.io/sagernet/sing-box:latest\n" {
		t.Fatalf("stdout %q", stdout)
	}
	code, _, stderr = run(t, "add", "postgres:15", "pg", "16")
	if code != 2 || !strings.Contains(stderr, "source tag 15 conflicts with version 16") {
		t.Fatalf("code %d %s", code, stderr)
	}
	script := "dockerfile/sail/build.sh"
	if err := os.MkdirAll(filepath.Join(dir, "dockerfile", "sail"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, script), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run(t, "add-dockerfile", script, "sail", "0.6.4")
	if code != 0 || stderr != "" {
		t.Fatalf("code %d %s", code, stderr)
	}
	if stdout != "added: sail:0.6.4 <- "+script+"\n" {
		t.Fatalf("stdout %q", stdout)
	}
	code, _, stderr = run(t, "add-dockerfile", "missing.sh", "nope")
	if code != 2 || !strings.Contains(stderr, "Dockerfile build script not found: missing.sh") {
		t.Fatalf("code %d %s", code, stderr)
	}
	code, stdout, stderr = run(t, "list")
	if code != 0 || stderr != "" {
		t.Fatalf("code %d %s", code, stderr)
	}
	if !strings.Contains(stdout, "sing-box\tmirror\tghcr.io/sagernet/sing-box\t") {
		t.Fatalf("stdout %s", stdout)
	}
	if !strings.Contains(stdout, "sail\tdockerfile\t"+script+"\t") {
		t.Fatalf("stdout %s", stdout)
	}
	code, _, stderr = run(t, "list", "missing")
	if code != 2 || !strings.Contains(stderr, "image alias not found: missing") {
		t.Fatalf("code %d %s", code, stderr)
	}
}

func TestComplete(t *testing.T) {
	isolate(t)
	gitRepo(t, "images:\n  cpa:\n    type: mirror\n    source: postgres\n    versions:\n      v2:\n        status: init\n      latest:\n        status: done\n")
	code, stdout, stderr := run(t, "__complete", "aliases")
	if code != 0 || stderr != "" || stdout != "cpa\n" {
		t.Fatalf("code %d %q %q", code, stdout, stderr)
	}
	code, stdout, stderr = run(t, "__complete", "versions", "cpa")
	if code != 0 || stdout != "v2\nlatest\n" || stderr != "" {
		t.Fatalf("code %d %q %q", code, stdout, stderr)
	}
	code, stdout, stderr = run(t, "__complete", "versions", "missing")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("code %d %q %q", code, stdout, stderr)
	}
	outside := t.TempDir()
	t.Chdir(outside)
	code, stdout, stderr = run(t, "__complete", "aliases")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("outside code %d %q %q", code, stdout, stderr)
	}
}

func TestFishCompletion(t *testing.T) {
	isolate(t)
	code, stdout, stderr := run(t, "completion", "fish")
	if code != 0 || stderr != "" {
		t.Fatalf("code %d %s", code, stderr)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "archive", "gh-image-cli", "src", "completion.ts"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, "String.raw`")
	end := strings.LastIndex(text, "`")
	if start < 0 || end <= start {
		t.Fatal("completion.ts template missing")
	}
	script := text[start+len("String.raw`") : end]
	// console.log 会再补一个换行。
	if stdout != script+"\n" {
		t.Fatalf("fish script drifted:\n%s", stdout)
	}
	code, _, stderr = run(t, "completion", "bash")
	if code != 2 || !strings.Contains(stderr, "only fish completion is supported") {
		t.Fatalf("code %d %s", code, stderr)
	}
}

func TestFlagRequiresValue(t *testing.T) {
	isolate(t)
	code, _, stderr := run(t, "list", "-c")
	if code != 2 || !strings.Contains(stderr, "-c requires a value") {
		t.Fatalf("code %d %s", code, stderr)
	}
	code, _, stderr = run(t, "--config", "--help")
	if code != 2 || !strings.Contains(stderr, "--config requires a value") {
		t.Fatalf("code %d %s", code, stderr)
	}
}

type ghFake struct {
	calls    [][]string
	list     func(id string) string
	viewOut  string
	viewCode int
	dispCode int
	dispErr  string
}

func (g *ghFake) run(bin string, args []string, cwd string) (cmdOut, error) {
	if bin == "git" {
		return defaultRun(bin, args, cwd)
	}
	if bin != "gh" {
		return cmdOut{}, fmt.Errorf("unexpected bin %s", bin)
	}
	g.calls = append(g.calls, append([]string{}, args...))
	switch {
	case len(args) >= 2 && args[0] == "repo" && args[1] == "view":
		return cmdOut{stdout: "{\"nameWithOwner\":\"acme/img\",\"defaultBranchRef\":{\"name\":\"main\"}}\n"}, nil
	case len(args) >= 1 && args[0] == "workflow":
		if g.dispCode != 0 {
			return cmdOut{code: g.dispCode, stderr: g.dispErr}, nil
		}
		return cmdOut{}, nil
	case len(args) >= 2 && args[0] == "run" && args[1] == "list":
		body := "[]"
		if g.list != nil {
			body = g.list(flagOf(g.calls, "request_id"))
		}
		return cmdOut{stdout: body}, nil
	case len(args) >= 2 && args[0] == "run" && args[1] == "view":
		return cmdOut{code: g.viewCode, stdout: g.viewOut}, nil
	default:
		return cmdOut{code: 1, stderr: "unexpected gh"}, nil
	}
}

func flagOf(calls [][]string, name string) string {
	prefix := name + "="
	for _, args := range calls {
		for i := 0; i < len(args); i++ {
			if args[i] == "-f" && i+1 < len(args) && strings.HasPrefix(args[i+1], prefix) {
				return strings.TrimPrefix(args[i+1], prefix)
			}
		}
	}
	return ""
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestBuildSuccess(t *testing.T) {
	isolate(t)
	dir := gitRepo(t, "# inventory\nimages:\n  cpa:\n    type: mirror\n    source: eceasy/cli-proxy-api\n    versions:\n      latest:\n        status: done\n")
	fake := &ghFake{
		list: func(id string) string {
			return `[{"databaseId":9,"status":"completed","conclusion":"success","url":"https://example.test/run/9","displayTitle":"sync ` + id + `"}]`
		},
	}
	rt := defaultRT()
	rt.run = fake.run
	code, stdout, stderr := runWith(t, rt, "build", "cpa", "v9")
	if code != 0 || stderr != "" {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "alias: cpa\nsource: eceasy/cli-proxy-api\nversion: v9\nworkflow: sync-image.yml\n") {
		t.Fatalf("stdout %s", stdout)
	}
	if !strings.Contains(stdout, "action: completed\nrun: https://example.test/run/9\n") {
		t.Fatalf("stdout %s", stdout)
	}
	if !strings.Contains(stdout, "status: done\nrun: https://example.test/run/9\ntarget: registry.cn-shanghai.aliyuncs.com/linran-pub/cpa:v9\n") {
		t.Fatalf("stdout %s", stdout)
	}
	var dispatch []string
	for _, args := range fake.calls {
		if len(args) > 0 && args[0] == "workflow" {
			dispatch = args
		}
	}
	id := flagOf(fake.calls, "request_id")
	wantArgs := []string{
		"workflow", "run", "sync-image.yml",
		"--repo", "acme/img",
		"--ref", "main",
		"-f", "source=eceasy/cli-proxy-api",
		"-f", "alias=cpa",
		"-f", "version=v9",
		"-f", "request_id=" + id,
		"-f", "registry=registry.cn-shanghai.aliyuncs.com",
		"-f", "namespace=linran-pub",
	}
	if strings.Join(dispatch, "\n") != strings.Join(wantArgs, "\n") {
		t.Fatalf("dispatch:\n%s", strings.Join(dispatch, "\n"))
	}
	body, err := os.ReadFile(filepath.Join(dir, "images.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.HasPrefix(text, "# inventory") {
		t.Fatalf("comment lost %s", text)
	}
	if strings.Index(text, "v9") > strings.Index(text, "latest") {
		t.Fatalf("order %s", text)
	}
	if !strings.Contains(text, "status: done") {
		t.Fatalf("status %s", text)
	}
}

func TestBuildFailed(t *testing.T) {
	isolate(t)
	dir := gitRepo(t, "images:\n  sail:\n    type: dockerfile\n    script: dockerfile/sail/build.sh\n    versions:\n      0.6.4:\n        status: init\n")
	fake := &ghFake{
		list: func(id string) string {
			return `[{"databaseId":3,"status":"completed","conclusion":"failure","url":"https://example.test/run/3","displayTitle":"build ` + id + `"}]`
		},
		viewOut:  "step failed\n",
		viewCode: 1,
	}
	rt := defaultRT()
	rt.run = fake.run
	code, stdout, stderr := runWith(t, rt, "build", "sail")
	if code != 1 {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	if stderr != "step failed\n" {
		t.Fatalf("stderr %q", stderr)
	}
	if !strings.Contains(stdout, "workflow: build-dockerfile.yml\n") || !strings.Contains(stdout, "status: failed\n") {
		t.Fatalf("stdout %s", stdout)
	}
	body, err := os.ReadFile(filepath.Join(dir, "images.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "status: failed") {
		t.Fatalf("yaml %s", body)
	}
	var dispatch []string
	for _, args := range fake.calls {
		if len(args) > 0 && args[0] == "workflow" {
			dispatch = args
		}
	}
	if !hasArg(dispatch, "script=dockerfile/sail/build.sh") || !hasArg(dispatch, "build-dockerfile.yml") {
		t.Fatalf("dispatch %v", dispatch)
	}
}

func TestBuildDispatchFail(t *testing.T) {
	isolate(t)
	dir := gitRepo(t, "images:\n  cpa:\n    type: mirror\n    source: postgres\n    versions:\n      latest:\n        status: done\n")
	fake := &ghFake{dispCode: 1, dispErr: "dispatch refused\n"}
	rt := defaultRT()
	rt.run = fake.run
	code, _, stderr := runWith(t, rt, "build", "cpa", "latest")
	if code != 2 || !strings.Contains(stderr, "gh workflow run sync-image.yml") || !strings.Contains(stderr, "dispatch refused") {
		t.Fatalf("code %d %s", code, stderr)
	}
	body, err := os.ReadFile(filepath.Join(dir, "images.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "status: failed") {
		t.Fatalf("yaml %s", body)
	}
}

func TestBuildTimeout(t *testing.T) {
	isolate(t)
	dir := gitRepo(t, "images:\n  cpa:\n    type: mirror\n    source: postgres\n    versions:\n      latest:\n        status: done\n")
	home := os.Getenv("HOME")
	cfgDir := filepath.Join(home, ".config", "gh-image-cli")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"timeoutSeconds":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &ghFake{}
	cur := time.Unix(0, 0)
	rt := defaultRT()
	rt.run = fake.run
	rt.now = func() time.Time { return cur }
	rt.sleep = func(d time.Duration) { cur = cur.Add(d) }
	code, stdout, stderr := runWith(t, rt, "build", "cpa")
	if code != 124 || stderr != "" {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "status: timeout\n") || !strings.Contains(stdout, "https://github.com/acme/img/actions/workflows/sync-image.yml\n") {
		t.Fatalf("stdout %s", stdout)
	}
	body, err := os.ReadFile(filepath.Join(dir, "images.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "status: timeout") {
		t.Fatalf("yaml %s", body)
	}
}
