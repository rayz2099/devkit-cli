package cli

import "testing"

func TestIsHelpRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want bool
	}{
		{name: "root short flag", args: []string{"-h"}, want: true},
		{name: "root long flag", args: []string{"--help"}, want: true},
		{name: "root help command", args: []string{"help"}, want: true},
		{name: "md short flag", args: []string{"md", "-h"}, want: true},
		{name: "md help command", args: []string{"md", "help"}, want: true},
		{name: "real command", args: []string{"md", "en2zh"}, want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsHelpRequest(tc.args); got != tc.want {
				t.Fatalf("IsHelpRequest(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestHelpTextContainsCompletionCommand(t *testing.T) {
	t.Parallel()

	text := HelpText([]string{"-h"})
	if text == "" {
		t.Fatal("HelpText() returned empty text")
	}
	if !containsAll(text, "tl completion fish", "completion") {
		t.Fatalf("HelpText() = %q, want completion usage", text)
	}
}

func TestHelpTextContainsRootTranslateCommands(t *testing.T) {
	t.Parallel()

	text := HelpText([]string{"-h"})
	if !containsAll(text, "tl en2zh [text]", "tl zh2en [text]", "stdin") {
		t.Fatalf("HelpText() = %q, want root translate usage", text)
	}
}

func TestHelpTextContainsVersionCommand(t *testing.T) {
	t.Parallel()

	text := HelpText([]string{"-h"})
	if !containsAll(text, "tl --version", "Print current version") {
		t.Fatalf("HelpText() = %q, want version usage", text)
	}
}

func containsAll(text string, wants ...string) bool {
	for _, want := range wants {
		if !contains(text, want) {
			return false
		}
	}
	return true
}

func contains(text string, target string) bool {
	return len(text) >= len(target) && (text == target || (len(target) > 0 && (indexOf(text, target) >= 0)))
}

func indexOf(text string, target string) int {
	for index := 0; index+len(target) <= len(text); index++ {
		if text[index:index+len(target)] == target {
			return index
		}
	}
	return -1
}
