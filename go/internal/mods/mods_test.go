package mods

import "testing"

func TestAllOmitsAlistAndKeepsTl(t *testing.T) {
	var sawTl bool
	for _, mod := range All() {
		if mod.Name == "alist-cli" {
			t.Fatal("alist-cli must not be listed")
		}
		if mod.Name == "tl" {
			sawTl = true
			if mod.Kind != KindJust {
				t.Fatal("tl must stay on its own justfile")
			}
		}
	}
	if !sawTl {
		t.Fatal("tl missing")
	}
}

func TestSelectUnknown(t *testing.T) {
	_, err := Select([]string{"alist-cli"})
	if err == nil {
		t.Fatal("expected unknown module to fail")
	}
}
