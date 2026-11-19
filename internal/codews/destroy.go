package codews

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runDestroy(out io.Writer, args Parsed) error {
	cfg, wsDir, ws, err := loadWs(args.Config)
	if err != nil {
		return err
	}
	return DestroyWorkspace(out, args, cfg, wsDir, ws)
}

// DestroyWorkspace 按仓串行收尾；保留文档和清单，以便重跑未完成的步骤。
func DestroyWorkspace(out io.Writer, args Parsed, cfg Cfg, wsDir string, ws WsProject) error {
	if args.DeleteRemote && !args.Force {
		return errors.New("--delete-remote requires --force")
	}
	root, err := filepath.EvalSymlinks(wsDir)
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	if args.Force {
		ref := "refs/heads/" + ws.Branch
		cmd := []string{"git", "check-ref-format", ref}
		if res := runGit(root, cmd, true); res.Code != 0 {
			return fmt.Errorf("invalid workspace branch: %q", ws.Branch)
		}
	}

	var failures []error
	cleared, already := 0, 0
	for _, name := range ws.Repos {
		repo, err := repoFromWs(cfg, ws, name)
		changed := false
		if err == nil {
			changed, err = destroyRepo(out, args, cfg, root, ws.Branch, repo)
		}
		switch {
		case err != nil:
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
		case changed:
			cleared++
		default:
			already++
		}
	}
	fmt.Fprintf(out, "workspace cleanup: cleared=%d already=%d failed=%d total=%d\n", cleared, already, len(failures), len(ws.Repos))
	fmt.Fprintf(out, "workspace directory kept: %s\n", wsDir)
	if len(failures) > 0 {
		return fmt.Errorf("workspace cleanup incomplete:\n%w", errors.Join(failures...))
	}
	return nil
}

// worktreeRemoveCmd 只让 Git 卸载已注册的 worktree，不直接删目录。
func worktreeRemoveCmd(repo Repo, root string) Cmd {
	return Cmd{
		Repo:   repo.Name,
		Branch: "workspace",
		Step:   "worktree-remove",
		Cwd:    repo.Path,
		Args:   []string{"git", "worktree", "remove", "--force", filepath.Join(root, repo.Name)},
	}
}

// destroyWorktrees 只把当前 workspace 的注册路径算作待卸载；其他 worktree 仍拥有其分支。
func destroyWorktrees(text, dst, ref string) (registered bool, occupied string) {
	path := ""
	for field := range strings.SplitSeq(text, "\x00") {
		if p, ok := strings.CutPrefix(field, "worktree "); ok {
			path = filepath.Clean(p)
			registered = registered || path == dst
		} else if field == "branch "+ref && path != dst {
			occupied = path
		}
	}
	return registered, occupied
}

// destroyRepo 先卸载再删分支；只要卸载失败，就不触碰该仓的分支。
func destroyRepo(out io.Writer, args Parsed, cfg Cfg, root, branch string, repo Repo) (bool, error) {
	if repo.Name == "" || repo.Name == "." || repo.Name == ".." || filepath.Base(repo.Name) != repo.Name {
		return false, fmt.Errorf("invalid workspace repo name: %q", repo.Name)
	}
	dst := filepath.Join(root, repo.Name)
	ref := "refs/heads/" + branch
	if args.Force && repo.Checkout == "" {
		// 源仓目录配错时，Git 可能向上找到另一个仓，不能在那个仓删分支。
		prefix, err := gitOut(repo.Path, []string{"git", "rev-parse", "--show-prefix"})
		if err != nil {
			return false, err
		}
		if strings.TrimSuffix(prefix, "\n") != "" {
			return false, fmt.Errorf("configured source is not a repository root: %s", repo.Path)
		}
	}
	list, err := gitOut(repo.Path, []string{"git", "worktree", "list", "--porcelain", "-z"})
	if err != nil {
		return false, err
	}
	registered, occupied := destroyWorktrees(list, dst, ref)
	info, err := os.Lstat(dst)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if info != nil && (!registered || !info.IsDir()) {
		return false, errors.New(UnsafeMsg(repo.Name, dst, "path is not a registered worktree directory"))
	}
	if registered {
		if !args.Force {
			repos := []Repo{repo}
			if err := AssertDisposable(root, repos, cfg.Remote, cfg.Base); err != nil {
				return false, err
			}
		}
		cmd := worktreeRemoveCmd(repo, root)
		if err := RunCmd(out, cmd, args.Verbose); err != nil {
			return false, err
		}
	}
	// checkout 覆盖创建的是 detached worktree，不拥有覆盖分支，也不拥有同名任务分支。
	if !args.Force || repo.Checkout != "" {
		return registered, nil
	}
	if branch == cfg.Base || branch == repo.Branch || branch == repo.Merge {
		return registered, fmt.Errorf("refusing to delete base branch: %s", branch)
	}
	if occupied != "" {
		return registered, fmt.Errorf("task branch %s is in use by another worktree: %s", branch, occupied)
	}
	local, err := gitOut(repo.Path, []string{"git", "for-each-ref", "--format=%(refname)", "--", ref})
	if err != nil {
		return registered, err
	}
	changed := registered
	for name := range strings.SplitSeq(local, "\n") {
		if name == ref {
			cmd := Cmd{Repo: repo.Name, Branch: branch, Step: "branch-delete", Cwd: repo.Path,
				Args: []string{"git", "branch", "-D", "--", branch}}
			if err := RunCmd(out, cmd, args.Verbose); err != nil {
				return changed, err
			}
			changed = true
			break
		}
	}
	if !args.DeleteRemote {
		return changed, nil
	}
	deleted, err := deleteRemoteBranch(out, repo, cfg.Remote, branch, args.Verbose)
	return changed || deleted, err
}

// deleteRemoteBranch 查实际 push 目标，不用可能过期的本地引用；只删查到的版本，保护并发推送。
func deleteRemoteBranch(out io.Writer, repo Repo, remote, branch string, verbose bool) (bool, error) {
	urls, err := gitOut(repo.Path, []string{"git", "remote", "get-url", "--push", "--all", "--", remote})
	if err != nil {
		return false, err
	}
	url := strings.TrimSuffix(urls, "\n")
	if url == "" || strings.Contains(url, "\n") {
		return false, fmt.Errorf("remote %s must have exactly one push URL", remote)
	}
	ref := "refs/heads/" + branch
	text, err := gitOut(repo.Path, []string{"git", "ls-remote", "--heads", "--", url, ref})
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(text, "\n") {
		oid, name, ok := strings.Cut(line, "\t")
		if !ok || name != ref {
			continue
		}
		lease := "--force-with-lease=" + ref + ":" + oid
		cmd := Cmd{Repo: repo.Name, Branch: branch, Step: "remote-branch-delete", Cwd: repo.Path,
			Args: []string{"git", "push", "--no-follow-tags", lease, "--", remote, ":" + ref}}
		if err := RunCmd(out, cmd, verbose); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
