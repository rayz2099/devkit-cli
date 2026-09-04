package cli

import (
	"testing"
)

func TestSplitConfigFlag(t *testing.T) {
	t.Parallel()

	path, rest, err := SplitConfigFlag([]string{"-c", "/tmp/tl.json", "en2zh", "hello"})
	if err != nil {
		t.Fatalf("SplitConfigFlag() error = %v", err)
	}
	if path != "/tmp/tl.json" {
		t.Fatalf("path = %q, want /tmp/tl.json", path)
	}
	if got := stringsJoin(rest); got != "en2zh hello" {
		t.Fatalf("rest = %q, want en2zh hello", got)
	}

	path, rest, err = SplitConfigFlag([]string{"zh2en", "--config", "/tmp/b.json", "你好"})
	if err != nil {
		t.Fatalf("SplitConfigFlag() error = %v", err)
	}
	if path != "/tmp/b.json" {
		t.Fatalf("path = %q, want /tmp/b.json", path)
	}
	if got := stringsJoin(rest); got != "zh2en 你好" {
		t.Fatalf("rest = %q, want zh2en 你好", got)
	}

	_, _, err = SplitConfigFlag([]string{"-c"})
	if err == nil {
		t.Fatal("SplitConfigFlag() missing value, want error")
	}
}

func stringsJoin(values []string) string {
	out := ""
	for index, value := range values {
		if index > 0 {
			out += " "
		}
		out += value
	}
	return out
}
