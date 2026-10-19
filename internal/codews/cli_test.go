package codews

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	got, err := ParseArgs([]string{"init", "feature/spec101", "-t", "work-01"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmd != "init" || got.Branch != "feature/spec101" || got.Profile != "work-01" || !got.HasProf || got.Verbose {
		t.Fatalf("%+v", got)
	}
	got, err = ParseArgs([]string{"init", "feature/spec101", "shared-lib", "-v"})
	if err != nil || got.Project != "shared-lib" || !got.Verbose || got.HasProf {
		t.Fatal(err, got)
	}
	got, err = ParseArgs([]string{"init", "feature/spec101"})
	if err != nil || got.Cmd != "init" || got.Branch != "feature/spec101" || !got.HasProf || got.Profile != "default" || got.HasProj {
		t.Fatalf("%+v", got)
	}
	_, err = ParseArgs([]string{"init", "feature/spec101", "shared-lib", "-t", "work-01"})
	if err == nil || !strings.Contains(err.Error(), "usage: code-ws init <branch> <project>") {
		t.Fatal(err)
	}
	got, err = ParseArgs([]string{"completion", "fish"})
	if err != nil || got.Cmd != "completion-fish" {
		t.Fatal(err, got)
	}
	got, err = ParseArgs([]string{"serve"})
	if err != nil || !got.Lan || !got.Watch || got.HasPath || got.Cf || got.HasPort {
		t.Fatalf("%+v", got)
	}
	got, err = ParseArgs([]string{"serve", "./docs", "--lan", "--port", "8787"})
	if err != nil || got.Path != "./docs" || got.Port != 8787 || !got.Lan {
		t.Fatal(got, err)
	}
	got, err = ParseArgs([]string{"serve", "--local"})
	if err != nil || got.Lan {
		t.Fatal(got, err)
	}
	got, err = ParseArgs([]string{"serve", "--no-watch"})
	if err != nil || got.Watch {
		t.Fatal(got, err)
	}
	got, err = ParseArgs([]string{"serve", "--cloudflare"})
	if err != nil || !got.Cf {
		t.Fatal(got, err)
	}
	got, err = ParseArgs([]string{"add", "project", "my-room", "-h"})
	if err != nil || got.Cmd != "help" {
		t.Fatal(got, err)
	}
	got, err = ParseArgs([]string{"projects", "-c", "/tmp/config.json"})
	if err != nil || got.Config != "/tmp/config.json" {
		t.Fatal(got, err)
	}
	_, err = ParseArgs([]string{"projects", "-p", "x"})
	if err == nil || !strings.Contains(err.Error(), "--project is only supported by init") {
		t.Fatal(err)
	}
	_, err = ParseArgs([]string{"--port", "nope"})
	if err == nil || !strings.Contains(err.Error(), "--port requires an integer") {
		t.Fatal(err)
	}
	_, err = ParseArgs([]string{"nope"})
	if err == nil || err.Error() != "unknown command: nope" {
		t.Fatal(err)
	}
}

func TestVersionDoesNotReadConfig(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "missing"))
	var out bytes.Buffer
	err := Run(strings.NewReader(""), &out, io.Discard, []string{"version"})
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "code-ws 0.1.0\n" {
		t.Fatalf("%q", out.String())
	}
}

func TestDoctorMissing(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	var out, errb bytes.Buffer
	err := Run(strings.NewReader(""), &out, &errb, []string{"doctor"})
	var ex *Exit
	if err == nil || !asExit(err, &ex) || ex.Code != 1 {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "fail config:") {
		t.Fatalf("stdout %q", out.String())
	}
	if errb.Len() != 0 {
		t.Fatalf("stderr %q", errb.String())
	}
	if _, statErr := os.Stat(cfg); !os.IsNotExist(statErr) {
		t.Fatal("config dir was created")
	}
}

func TestDoctorOK(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "code-ws")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{
	  "workspaceRoot": "$HOME/ws",
	  "baseBranch": "main",
	  "remote": "origin",
	  "initAgentsTemplate": "work",
	  "profiles": {"default": {"name": "d", "agentsTemplate": "work", "repos": ["app"]}}
	}`
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	var out bytes.Buffer
	if err := Run(strings.NewReader(""), &out, io.Discard, []string{"doctor"}); err != nil {
		t.Fatal(err, out.String())
	}
	if out.String() != "pass config\n" {
		t.Fatalf("%q", out.String())
	}
}

func TestFishAndHelp(t *testing.T) {
	txt := FishCompletion()
	for _, part := range []string{"__code_ws_projects", "__code_ws_from", "code-ws branches", "--workspace", "-l cf", "fork", "serve"} {
		if !strings.Contains(txt, part) {
			t.Fatalf("missing %s", part)
		}
	}
	var out bytes.Buffer
	if err := Run(strings.NewReader(""), &out, io.Discard, []string{"completion", "fish"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.String(), "\n") || !strings.Contains(out.String(), "-l cloudflare") {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := Run(strings.NewReader(""), &out, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "code-ws init") {
		t.Fatal(out.String())
	}
}

func asExit(err error, dst **Exit) bool {
	ex, ok := err.(*Exit)
	if ok {
		*dst = ex
	}
	return ok
}
