package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRootHelp 测试顶层帮助信息输出。
func TestRootHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := execute([]string{"--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Read-only diagnostic CLI for Apache Doris") {
		t.Errorf("expected description in help, got: %s", out)
	}
	if !strings.Contains(out, "cluster") || !strings.Contains(out, "doctor") || !strings.Contains(out, "profiles") || !strings.Contains(out, "completion") {
		t.Errorf("missing expected subcommands in help output")
	}
}

// TestDoctorCommand 测试 CLI doctor 命令执行。
func TestDoctorCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := execute([]string{"doctor"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error running doctor: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "pass config") {
		t.Errorf("expected doctor pass output, got: %s", out)
	}
}

// TestProfilesCommand 测试 profiles 列出命令。
func TestProfilesCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := execute([]string{"profiles"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error running profiles: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "default_profile") || !strings.Contains(out, "profiles") {
		t.Errorf("expected profiles report, got: %s", out)
	}
}

// TestCompletionFish 测试 Fish 补全脚本输出。
func TestCompletionFish(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := execute([]string{"completion", "fish"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error running completion fish: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "doris-cli") || !strings.Contains(out, "complete -c doris-cli") {
		t.Errorf("expected fish completion script, got: %s", out)
	}
}

// TestSubcommandFlags 测试子命令的标志解析是否完备。
func TestSubcommandFlags(t *testing.T) {
	root := newRoot()
	feLogCmd, _, err := root.Find([]string{"fe-log"})
	if err != nil {
		t.Fatalf("cannot find fe-log command: %v", err)
	}

	if feLogCmd.Flag("level") == nil || feLogCmd.Flag("limit") == nil || feLogCmd.Flag("grep") == nil {
		t.Errorf("fe-log missing expected flags")
	}

	queryCmd, _, err := root.Find([]string{"query"})
	if err != nil {
		t.Fatalf("cannot find query command: %v", err)
	}
	if queryCmd.Flag("slow") == nil || queryCmd.Flag("min-ms") == nil {
		t.Errorf("query missing expected flags")
	}
}
