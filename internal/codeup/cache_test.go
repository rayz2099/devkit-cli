package codeup

import (
	"os"
	"path/filepath"
	"testing"

	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

func TestPickCachedRepo(t *testing.T) {
	repos := []CachedRepo{
		{ID: "1", Name: "demo-api", PathNs: "backend/demo-api"},
		{ID: "2", Name: "demo-ops", PathNs: "backend/demo-ops"},
		{ID: "3", Name: "demo-ui", PathNs: "fe/demo-ui"},
	}
	hit, err := PickCachedRepo("demo-api", repos)
	if err != nil || hit.PathNs != "backend/demo-api" {
		t.Fatalf("hit = %+v %v", hit, err)
	}
	if ScoreRepo("demo-api", repos[0]) != 100 {
		t.Fatal("score")
	}
	ui, err := PickCachedRepo("demo-ui", repos)
	if err != nil || ui.PathNs != "fe/demo-ui" {
		t.Fatalf("ui = %+v %v", ui, err)
	}
	if _, err := PickCachedRepo("demo-", repos); err == nil || !contains(err.Error(), "ambiguous repository") {
		t.Fatalf("amb = %v", err)
	}
	if _, err := PickCachedRepo("nope", repos); err == nil || !contains(err.Error(), "no repository matched") {
		t.Fatalf("miss = %v", err)
	}
	hints := HintRepos(repos)
	if !containsSlice(hints, "backend/demo-api") || !containsSlice(hints, "demo-api") {
		t.Fatalf("hints = %v", hints)
	}
}

func TestIndexLandsInStateNotConfig(t *testing.T) {
	home := setHome(t)
	rt := Runtime{Profile: Profile{Name: "work"}, OrgID: "org-example", APIBase: APIBase}
	idx, err := SaveIndex(rt, []Repo{{
		ID: "1", Name: "demo", Path: "demo", PathNs: "org-example/group/demo",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if idx.Repos[0].PathNs != "group/demo" {
		t.Fatalf("short = %+v", idx.Repos)
	}
	path, err := IndexPath("work")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, ".config")
	if xdg.Inside(root, path) {
		t.Fatalf("index inside config: %s", path)
	}
	want := filepath.Join(home, ".local", "state", "codeup-cli", "repos-work.json")
	if path != want {
		t.Fatalf("path = %s", path)
	}
	if _, err := os.Stat(filepath.Join(root, "codeup-cli")); !os.IsNotExist(err) {
		t.Fatalf("config dir was created: %v", err)
	}
	loaded, err := LoadIndex("work", "org-example")
	if err != nil || loaded.Repos[0].PathNs != "group/demo" {
		t.Fatalf("load = %+v %v", loaded, err)
	}
	if _, err := LoadIndex("work", "other"); err == nil || !contains(err.Error(), "org mismatch") {
		t.Fatalf("org = %v", err)
	}
}

func TestIndexRejectsStateInsideConfig(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(cfg, "state"))
	_, err := SaveIndex(Runtime{Profile: Profile{Name: "work"}, OrgID: "org"}, nil)
	if err == nil {
		t.Fatal("expected state inside config to fail")
	}
	if _, statErr := os.Stat(cfg); !os.IsNotExist(statErr) {
		t.Fatalf("config created: %v", statErr)
	}
}

func containsSlice(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
