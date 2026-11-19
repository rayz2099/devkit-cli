package codews

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const taskBranch = "feature/cleanup"

// newDestroyFixture 使用本地裸仓当远端，验证实际引用与 worktree 的变化，不访问业务仓。
func newDestroyFixture(t *testing.T) (string, string, Cfg, WsProject) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	src := filepath.Join(root, "source")
	remote := filepath.Join(root, "remote.git")
	wsDir := filepath.Join(root, "workspace")
	if err := os.Mkdir(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	destroyTestGit(t, root, "init", "--initial-branch=main", src)
	destroyTestGit(t, root, "init", "--bare", "--initial-branch=main", remote)
	destroyTestGit(t, src, "config", "user.name", "Worktree Test")
	destroyTestGit(t, src, "config", "user.email", "test@example.invalid")
	destroyTestGit(t, src, "commit", "--allow-empty", "-m", "base")
	destroyTestGit(t, src, "remote", "add", "origin", remote)
	destroyTestGit(t, src, "push", "origin", "main")
	cfg := Cfg{Base: "main", Remote: "origin", Projects: []Repo{{Name: "app", Path: src, Branch: "main"}}}
	ws := WsProject{Branch: taskBranch, Repos: []string{"app"}}
	return wsDir, remote, cfg, ws
}

func destroyTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	buf, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, buf)
	}
	return strings.TrimSpace(string(buf))
}

func requireDestroyRef(t *testing.T, dir, branch string, exists bool) {
	t.Helper()
	ref := "refs/heads/" + branch
	got := destroyTestGit(t, dir, "for-each-ref", "--format=%(refname)", ref)
	if (got == ref) != exists {
		t.Fatalf("%s branch %s: got %q, want exists=%t", dir, branch, got, exists)
	}
}

func TestDestroySafetyForceAndRemote(t *testing.T) {
	root, remote, cfg, ws := newDestroyFixture(t)
	src := cfg.Projects[0].Path
	dst := filepath.Join(root, "app")
	destroyTestGit(t, src, "worktree", "add", "-b", taskBranch, dst, "main")
	file := filepath.Join(dst, "change.txt")
	if err := os.WriteFile(file, []byte("committed"), 0o644); err != nil {
		t.Fatal(err)
	}
	destroyTestGit(t, dst, "add", "change.txt")
	destroyTestGit(t, dst, "commit", "-m", "task")
	destroyTestGit(t, src, "push", "origin", taskBranch)
	destroyTestGit(t, src, "push", "origin", "main:refs/heads/feature/other")
	if err := os.WriteFile(file, []byte("uncommitted"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := DestroyWorkspace(&out, Parsed{}, cfg, root, ws); err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatal(err, out.String())
	}
	destroyTestGit(t, dst, "restore", "change.txt")
	if err := DestroyWorkspace(&out, Parsed{}, cfg, root, ws); err == nil || !strings.Contains(err.Error(), "unmerged commits: 1") {
		t.Fatal(err, out.String())
	}
	if err := os.WriteFile(file, []byte("discard me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := DestroyWorkspace(&out, Parsed{Force: true}, cfg, root, ws); err != nil {
		t.Fatal(err, out.String())
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("worktree should be gone, got %v", err)
	}
	requireDestroyRef(t, src, taskBranch, false)
	requireDestroyRef(t, remote, taskBranch, true)
	out.Reset()
	args := Parsed{Force: true, DeleteRemote: true}
	if err := DestroyWorkspace(&out, args, cfg, root, ws); err != nil {
		t.Fatal(err, out.String())
	}
	if strings.Contains(out.String(), "worktree-remove") || !strings.Contains(out.String(), "remote-branch-delete") {
		t.Fatal(out.String())
	}
	requireDestroyRef(t, remote, taskBranch, false)
	requireDestroyRef(t, remote, "feature/other", true)
	requireDestroyRef(t, remote, "main", true)
	out.Reset()
	if err := DestroyWorkspace(&out, args, cfg, root, ws); err != nil {
		t.Fatal(err, out.String())
	}
	if strings.Contains(out.String(), "[app]") || !strings.Contains(out.String(), "cleared=0 already=1 failed=0") {
		t.Fatal(out.String())
	}
}

func TestDestroyPreservesSharedAndUnrelatedBranches(t *testing.T) {
	t.Run("different checked-out branch", func(t *testing.T) {
		root, _, cfg, ws := newDestroyFixture(t)
		src := cfg.Projects[0].Path
		dst := filepath.Join(root, "app")
		destroyTestGit(t, src, "branch", taskBranch)
		destroyTestGit(t, src, "worktree", "add", "-b", "feature/other", dst)
		var out bytes.Buffer
		if err := DestroyWorkspace(&out, Parsed{Force: true}, cfg, root, ws); err != nil {
			t.Fatal(err, out.String())
		}
		requireDestroyRef(t, src, taskBranch, false)
		requireDestroyRef(t, src, "feature/other", true)
	})

	t.Run("other worktree and next repo", func(t *testing.T) {
		root, remote, cfg, ws := newDestroyFixture(t)
		src := cfg.Projects[0].Path
		other := filepath.Join(t.TempDir(), "other")
		destroyTestGit(t, src, "worktree", "add", "-b", taskBranch, other)
		destroyTestGit(t, src, "push", "origin", taskBranch)
		destroyTestGit(t, src, "worktree", "add", "--detach", filepath.Join(root, "app"))
		second := filepath.Join(t.TempDir(), "next")
		destroyTestGit(t, src, "clone", remote, second)
		destroyTestGit(t, second, "worktree", "add", "-b", taskBranch, filepath.Join(root, "next"))
		cfg.Projects = append(cfg.Projects, Repo{Name: "next", Path: second})
		ws.Repos = append(ws.Repos, "next")
		var out bytes.Buffer
		err := DestroyWorkspace(&out, Parsed{Force: true}, cfg, root, ws)
		if err == nil || !strings.Contains(err.Error(), "in use by another worktree") {
			t.Fatal(err, out.String())
		}
		if !strings.Contains(out.String(), "cleared=1 already=0 failed=1 total=2") {
			t.Fatal(out.String())
		}
		requireDestroyRef(t, src, taskBranch, true)
		requireDestroyRef(t, second, taskBranch, false)
	})

	t.Run("detached override", func(t *testing.T) {
		root, remote, cfg, ws := newDestroyFixture(t)
		src := cfg.Projects[0].Path
		ws.Checkout = map[string]string{"app": "feature/borrowed"}
		destroyTestGit(t, src, "branch", taskBranch)
		destroyTestGit(t, src, "branch", "feature/borrowed")
		destroyTestGit(t, src, "push", "origin", taskBranch, "feature/borrowed")
		destroyTestGit(t, src, "worktree", "add", "--detach", filepath.Join(root, "app"), "feature/borrowed")
		var out bytes.Buffer
		err := DestroyWorkspace(&out, Parsed{Force: true, DeleteRemote: true}, cfg, root, ws)
		if err != nil || !strings.Contains(out.String(), "cleared=1") {
			t.Fatal(err, out.String())
		}
		requireDestroyRef(t, src, taskBranch, true)
		requireDestroyRef(t, remote, taskBranch, true)
		requireDestroyRef(t, src, "feature/borrowed", true)
		requireDestroyRef(t, remote, "feature/borrowed", true)
	})
}

func TestDestroyPreservesBaseAndUnregisteredPaths(t *testing.T) {
	root, remote, cfg, ws := newDestroyFixture(t)
	src := cfg.Projects[0].Path
	dst := filepath.Join(root, "app")
	destroyTestGit(t, src, "branch", "integration")
	destroyTestGit(t, src, "push", "origin", "integration")
	ws.Branch = "integration"
	ws.Merges = map[string]string{"app": "integration"}
	destroyTestGit(t, src, "worktree", "add", dst, "integration")
	var out bytes.Buffer
	args := Parsed{Force: true, DeleteRemote: true}
	if err := DestroyWorkspace(&out, args, cfg, root, ws); err == nil || !strings.Contains(err.Error(), "refusing to delete base branch") {
		t.Fatal(err, out.String())
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("worktree should be gone, got %v", err)
	}
	requireDestroyRef(t, src, "integration", true)
	requireDestroyRef(t, remote, "integration", true)

	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	destroyTestGit(t, src, "branch", taskBranch)
	ws.Branch = taskBranch
	out.Reset()
	if err := DestroyWorkspace(&out, args, cfg, root, ws); err == nil || !strings.Contains(err.Error(), "not a registered worktree") {
		t.Fatal(err, out.String())
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatal(err)
	}
	requireDestroyRef(t, src, taskBranch, true)
}

func TestDestroyUsesPushURLAndRetriesRemoteFailure(t *testing.T) {
	root, fetch, cfg, ws := newDestroyFixture(t)
	src := cfg.Projects[0].Path
	destroyTestGit(t, src, "branch", taskBranch)
	destroyTestGit(t, src, "push", "origin", taskBranch)
	push := filepath.Join(t.TempDir(), "push.git")
	destroyTestGit(t, src, "clone", "--bare", fetch, push)
	destroyTestGit(t, src, "remote", "set-url", "--push", "origin", push)
	destroyTestGit(t, push, "config", "receive.denyDeletes", "true")
	var out bytes.Buffer
	args := Parsed{Force: true, DeleteRemote: true}
	if err := DestroyWorkspace(&out, args, cfg, root, ws); err == nil || !strings.Contains(err.Error(), "remote-branch-delete") {
		t.Fatal(err, out.String())
	}
	requireDestroyRef(t, src, taskBranch, false)
	requireDestroyRef(t, push, taskBranch, true)
	destroyTestGit(t, push, "config", "receive.denyDeletes", "false")
	out.Reset()
	if err := DestroyWorkspace(&out, args, cfg, root, ws); err != nil {
		t.Fatal(err, out.String())
	}
	if strings.Contains(out.String(), "[app] branch-delete") {
		t.Fatal(out.String())
	}
	requireDestroyRef(t, push, taskBranch, false)
	requireDestroyRef(t, fetch, taskBranch, true)
}
