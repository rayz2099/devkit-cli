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

// TestIssueRegistered 保证根 build/install 能发现新命令，且名单保持有序。
func TestIssueRegistered(t *testing.T) {
	selected, err := Select([]string{"issue-cli"})
	if err != nil || len(selected) != 1 || selected[0].Kind != KindGo {
		t.Fatalf("issue-cli registration: %v %v", selected, err)
	}
	all := All()
	for i := 1; i < len(all); i++ {
		if all[i-1].Name >= all[i].Name {
			t.Fatal("module list must be sorted")
		}
	}
}

func TestSelectUnknown(t *testing.T) {
	_, err := Select([]string{"alist-cli"})
	if err == nil {
		t.Fatal("expected unknown module to fail")
	}
}
