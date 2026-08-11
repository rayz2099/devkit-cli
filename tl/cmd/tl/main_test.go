package main

import (
	"bytes"
	"context"
	"testing"

	"git.internal.linran.top/linran/tl/internal/cli"
)

func TestRunWithHelpSkipsConfigValidation(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run(
		context.Background(),
		[]string{"-h"},
		&stdout,
		&stderr,
		func() (*cli.App, error) {
			t.Fatal("app factory should not be called for help")
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("stdout should contain help text")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunWithMdHelpPrintsCommandUsage(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := run(
		context.Background(),
		[]string{"md", "-h"},
		&stdout,
		&bytes.Buffer{},
		func() (*cli.App, error) {
			t.Fatal("app factory should not be called for md help")
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := stdout.String(); got == "" || !containsAll(got, "tl md en2zh [file]", "stdin") {
		t.Fatalf("stdout = %q, want md help text", got)
	}
}

func TestRunWithRootHelpPrintsRootTranslateUsage(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := run(
		context.Background(),
		[]string{"help"},
		&stdout,
		&bytes.Buffer{},
		func() (*cli.App, error) {
			t.Fatal("app factory should not be called for root help")
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := stdout.String(); got == "" || !containsAll(got, "tl en2zh [text]", "tl zh2en [text]") {
		t.Fatalf("stdout = %q, want root translate help text", got)
	}
}

func TestRunWithCompletionFishSkipsFactoryAndPrintsScript(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := run(
		context.Background(),
		[]string{"completion", "fish"},
		&stdout,
		&bytes.Buffer{},
		func() (*cli.App, error) {
			t.Fatal("app factory should not be called for completion fish")
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := stdout.String(); got == "" || !containsAll(got, "complete -c tl", "__fish_use_subcommand") {
		t.Fatalf("stdout = %q, want fish completion script", got)
	}
}

func TestRunWithVersionSkipsFactoryAndPrintsVersion(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := run(
		context.Background(),
		[]string{"--version"},
		&stdout,
		&bytes.Buffer{},
		func() (*cli.App, error) {
			t.Fatal("app factory should not be called for version")
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := stdout.String(); got != "v0.2.1\n" {
		t.Fatalf("stdout = %q, want %q", got, "v0.2.1\n")
	}
}

func containsAll(text string, wants ...string) bool {
	for _, want := range wants {
		if !bytes.Contains([]byte(text), []byte(want)) {
			return false
		}
	}
	return true
}
