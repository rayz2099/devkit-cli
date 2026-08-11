package cli

import (
	"strings"
	"testing"
)

func TestIsCompletionRequest(t *testing.T) {
	t.Parallel()

	if !IsCompletionRequest([]string{"completion", "fish"}) {
		t.Fatal("expected completion fish request to be true")
	}
	if IsCompletionRequest([]string{"completion"}) {
		t.Fatal("expected completion without shell to be false")
	}
	if IsCompletionRequest([]string{"completion", "bash"}) {
		t.Fatal("expected unsupported shell to be false")
	}
}

func TestCompletionScriptContainsCoreCommands(t *testing.T) {
	t.Parallel()

	script, err := CompletionScript([]string{"completion", "fish"})
	if err != nil {
		t.Fatalf("CompletionScript() error = %v", err)
	}
	if !strings.Contains(script, "complete -c tl") {
		t.Fatalf("script = %q, want fish completion prefix", script)
	}
	if !strings.Contains(script, "-a md") || !strings.Contains(script, "-a completion") {
		t.Fatalf("script = %q, want md and completion commands", script)
	}
}

func TestCompletionScriptContainsRootTranslateCommands(t *testing.T) {
	t.Parallel()

	script, err := CompletionScript([]string{"completion", "fish"})
	if err != nil {
		t.Fatalf("CompletionScript() error = %v", err)
	}
	if !strings.Contains(script, "-a en2zh") || !strings.Contains(script, "-a zh2en") {
		t.Fatalf("script = %q, want root translate commands", script)
	}
}

func TestCompletionScriptContainsVersionOption(t *testing.T) {
	t.Parallel()

	script, err := CompletionScript([]string{"completion", "fish"})
	if err != nil {
		t.Fatalf("CompletionScript() error = %v", err)
	}
	if !strings.Contains(script, "-l version") {
		t.Fatalf("script = %q, want version option", script)
	}
}
