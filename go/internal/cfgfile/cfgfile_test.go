package cfgfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadMissingMentionsExample(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	_, err := Read(path, "dsn-cli/config.example.json")
	if !errors.Is(err, ErrMissing) {
		t.Fatalf("err = %v, want ErrMissing", err)
	}
	if got := err.Error(); !contains(got, "dsn-cli/config.example.json") {
		t.Fatalf("err = %q, want example path", got)
	}
}

func TestParseJSONCStripsCommentAndTrailingComma(t *testing.T) {
	raw := []byte("{\n  // note\n  \"profiles\": [\n    {\"name\": \"a\",},\n  ],\n}\n")
	var dest struct {
		Profiles []struct {
			Name string `json:"name"`
		} `json:"profiles"`
	}
	if err := ParseJSONC(raw, &dest); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(dest.Profiles) != 1 || dest.Profiles[0].Name != "a" {
		t.Fatalf("dest = %#v", dest)
	}
}

func TestReadDoesNotCreateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	_, _ = Read(path, "example.json")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read created config: %v", err)
	}
}

func contains(s, part string) bool {
	return len(s) >= len(part) && (s == part || len(part) == 0 ||
		(func() bool {
			for i := 0; i+len(part) <= len(s); i++ {
				if s[i:i+len(part)] == part {
					return true
				}
			}
			return false
		})())
}
