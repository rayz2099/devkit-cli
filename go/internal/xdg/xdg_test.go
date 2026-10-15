package xdg

import (
	"path/filepath"
	"testing"
)

func TestResolveKeepsCacheOutOfConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

	dirs, err := Resolve("code-ws")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !Inside(filepath.Join(home, ".config"), dirs.Config) {
		t.Fatalf("config = %s, want inside ~/.config", dirs.Config)
	}
	if Inside(filepath.Join(home, ".config"), dirs.Cache) {
		t.Fatalf("cache fell inside config: %s", dirs.Cache)
	}
	if Inside(filepath.Join(home, ".config"), dirs.State) {
		t.Fatalf("state fell inside config: %s", dirs.State)
	}
}

func TestResolveRejectsCacheInsideConfig(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(cfg, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

	_, err := Resolve("code-ws")
	if err == nil {
		t.Fatal("expected cache inside config to fail")
	}
}
