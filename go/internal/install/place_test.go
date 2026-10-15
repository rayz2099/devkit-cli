package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlaceCopiesAndLeavesConfigUntouched(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root is refused")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	cfg := filepath.Join(home, ".config", "code-ws")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(cfg, "config.json")
	if err := os.WriteFile(marker, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "har-cli")
	if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	destDir := filepath.Join(home, ".local", "bin")
	got, err := Place(src, destDir)
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("installed file: %v", err)
	}
	buf, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "{}\n" {
		t.Fatalf("config changed: %q", buf)
	}
}

func TestPlaceRefusesConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := filepath.Join(home, ".config")
	t.Setenv("XDG_CONFIG_HOME", cfg)
	src := filepath.Join(t.TempDir(), "har-cli")
	if err := os.WriteFile(src, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Place(src, cfg); err == nil {
		t.Fatal("expected refuse")
	}
}
