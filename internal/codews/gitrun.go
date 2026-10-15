package codews

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type gitResult struct {
	Code   int
	Stdout string
	Stderr string
}

func runGit(cwd string, args []string, discard bool) gitResult {
	if len(args) == 0 {
		return gitResult{Code: 1, Stderr: "command args must not be empty"}
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cwd
	var out, errb bytes.Buffer
	if discard {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	} else {
		cmd.Stdout = &out
		cmd.Stderr = &errb
	}
	err := cmd.Run()
	res := gitResult{Stdout: out.String(), Stderr: errb.String()}
	if err == nil {
		return res
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		res.Code = exit.ExitCode()
		return res
	}
	res.Code = 1
	if res.Stderr == "" {
		res.Stderr = err.Error()
	}
	return res
}

func gitOut(cwd string, args []string) (string, error) {
	res := runGit(cwd, args, false)
	if res.Code != 0 {
		err := res.Stderr
		if err == "" {
			err = res.Stdout
		}
		return "", fmt.Errorf("git check failed: %s", strings.TrimSpace(err))
	}
	return res.Stdout, nil
}

func localBranchExists(repo Repo, branch string) bool {
	res := runGit(repo.Path, []string{
		"git", "show-ref", "--verify", "--quiet", "refs/heads/" + branch,
	}, true)
	return res.Code == 0
}

func lookupChecked(repo Repo, branch string) (string, bool, error) {
	res := runGit(repo.Path, []string{"git", "worktree", "list", "--porcelain"}, false)
	if res.Code != 0 {
		err := res.Stderr
		if err == "" {
			err = res.Stdout
		}
		return "", false, fmt.Errorf("git check failed: %s", strings.TrimSpace(err))
	}
	path := ParseWorktreePath(res.Stdout, branch)
	if path == "" {
		return "", false, nil
	}
	return path, true, nil
}

// LookupRemote 用 ls-remote 确认远端分支名，写进 project.yml 的必须是真实 refs/heads。
func LookupRemote(cwd, remote, ref string) (string, error) {
	text, err := gitOut(cwd, []string{"git", "ls-remote", "--heads", remote, "refs/heads/" + ref})
	if err != nil {
		return "", err
	}
	name := MatchedHead(text, ref)
	if name == "" {
		return "", fmt.Errorf("remote branch not found: %s/%s", remote, ref)
	}
	return name, nil
}

// ListRemote 列出远端短名，给 branches 命令和补全用。
func ListRemote(cwd, remote string) ([]string, error) {
	text, err := gitOut(cwd, []string{"git", "branch", "-r"})
	if err != nil {
		return nil, err
	}
	names := RemoteNames(text, remote)
	if names == nil {
		names = []string{}
	}
	return names, nil
}

func isWorktree(path string) bool {
	res := runGit("", []string{"git", "-C", path, "rev-parse", "--is-inside-work-tree"}, false)
	return res.Code == 0 && strings.TrimSpace(res.Stdout) == "true"
}

func mergedByPatch(cwd, head, target string) (bool, error) {
	rng := target + ".." + head
	merges, err := gitOut(cwd, []string{"git", "rev-list", "--count", "--merges", rng})
	if err != nil {
		return false, err
	}
	if parseCount(merges) != 0 {
		return false, nil
	}
	cherry, err := gitOut(cwd, []string{"git", "cherry", target, head})
	if err != nil {
		return false, err
	}
	return PatchMerged(cherry, merges), nil
}

func mergedByTree(cwd, head, target string) (bool, error) {
	res := runGit(cwd, []string{"git", "merge-tree", "--write-tree", target, head}, false)
	if res.Code != 0 && res.Code != 1 {
		err := res.Stderr
		if err == "" {
			err = res.Stdout
		}
		return false, fmt.Errorf("git check failed: %s", strings.TrimSpace(err))
	}
	if res.Code == 0 {
		tree, err := gitOut(cwd, []string{"git", "rev-parse", target + "^{tree}"})
		if err != nil {
			return false, err
		}
		if strings.TrimSpace(res.Stdout) == strings.TrimSpace(tree) {
			return true, nil
		}
	}
	return mergedByPatch(cwd, head, target)
}

// AssertDisposable 删除前确认没有未提交改动，且 HEAD 相对远端合入分支没有独有变更。
func AssertDisposable(wsDir string, repos []Repo, remote, base string) error {
	for _, repo := range repos {
		br := repo.Merge
		if br == "" {
			br = repoBase(repo, base)
		}
		dst := filepath.Join(wsDir, repo.Name)
		if _, err := os.Stat(dst); err != nil || !isWorktree(dst) {
			continue
		}
		status, err := gitOut(dst, []string{"git", "status", "--porcelain"})
		if err != nil {
			return err
		}
		if strings.TrimSpace(status) != "" {
			return errors.New(UnsafeMsg(repo.Name, dst, "uncommitted changes"))
		}
		if _, err := gitOut(dst, []string{"git", "fetch", "--quiet", remote, br}); err != nil {
			return err
		}
		remoteRef := remote + "/" + br
		ok, err := mergedByTree(dst, "HEAD", remoteRef)
		if err != nil {
			return err
		}
		if !ok {
			commits, err := gitOut(dst, []string{"git", "rev-list", "--count", remoteRef + "..HEAD"})
			if err != nil {
				return err
			}
			reason := "unmerged commits: " + strings.TrimSpace(commits)
			return errors.New(UnsafeMsg(repo.Name, dst, reason))
		}
	}
	return nil
}

// AssertClean fork 前只拦未提交内容。已经提交的 commit 是新分支的合法起点。
func AssertClean(wsDir string, repos []Repo) error {
	for _, repo := range repos {
		dst := filepath.Join(wsDir, repo.Name)
		if _, err := os.Stat(dst); err != nil {
			return fmt.Errorf("workspace repo path not found: %s", dst)
		}
		status, err := gitOut(dst, []string{"git", "status", "--porcelain"})
		if err != nil {
			return err
		}
		if HasDirty(status) {
			return errors.New(UnsafeMsg(repo.Name, dst, "uncommitted changes"))
		}
	}
	return nil
}

// AssertTargets 在创建目录前确认源仓库在、目标还不在，避免留下半成品。
func AssertTargets(wsDir string, repos []Repo) error {
	if _, err := os.Stat(wsDir); err == nil {
		return errors.New(ExistingDirMsg("workspace dir", wsDir))
	}
	for _, repo := range repos {
		if _, err := os.Stat(repo.Path); err != nil {
			return fmt.Errorf("repo not found: %s", repo.Path)
		}
		dst := filepath.Join(wsDir, repo.Name)
		if _, err := os.Stat(dst); err == nil {
			return errors.New(ExistingDirMsg("worktree dir", dst))
		}
	}
	return nil
}

func detailLog(stdout, stderr string) string {
	parts := []string{}
	if strings.TrimSpace(stdout) != "" {
		parts = append(parts, strings.TrimSpace(stdout))
	}
	if strings.TrimSpace(stderr) != "" {
		parts = append(parts, strings.TrimSpace(stderr))
	}
	return strings.Join(parts, "\n")
}

// RunCmd 执行一条 git 步骤。失败时默认只给摘要，-v 才附上完整输出。
func RunCmd(out io.Writer, cmd Cmd, verbose bool) error {
	if len(cmd.Args) == 0 {
		return errors.New("command args must not be empty")
	}
	fmt.Fprintln(out, StepLabel(cmd))
	res := runGit(cmd.Cwd, cmd.Args, false)
	if res.Code == 0 {
		return nil
	}
	errText := res.Stderr
	if errText == "" {
		errText = res.Stdout
	}
	if IgnoreFailure(cmd, errText) {
		return nil
	}
	msg := GitFailure(cmd, errText)
	detail := detailLog(res.Stdout, res.Stderr)
	if verbose && detail != "" {
		return errors.New(msg + "\n\n" + detail)
	}
	return errors.New(msg + "\nrun with -v for full git output")
}

// RunPlan 按顺序执行。前一条失败就停，避免后一条在坏状态上继续。
func RunPlan(out io.Writer, cmds []Cmd, verbose bool) error {
	for _, cmd := range cmds {
		if err := RunCmd(out, cmd, verbose); err != nil {
			return err
		}
	}
	return nil
}

// RunPlanKeepGoing 同步多仓时留住整体进度。单仓失败不挡住其他仓库。
func RunPlanKeepGoing(out, errw io.Writer, cmds []Cmd, verbose bool) int {
	failures := 0
	for _, cmd := range cmds {
		err := RunCmd(out, cmd, verbose)
		if err == nil {
			continue
		}
		failures++
		fmt.Fprintln(errw, err.Error())
	}
	return failures
}

func removeDir(path string) error {
	return os.RemoveAll(path)
}

// PrepareTargets 允许确认后清掉上次失败的 workspace，否则 init 会永远卡在已存在的目录上。
func PrepareTargets(out io.Writer, wsDir string, repos []Repo, remote, base string, verbose bool, read func(string) (string, error)) error {
	if _, err := os.Stat(wsDir); err == nil {
		answer, err := read(ConfirmDeletePrompt(wsDir))
		if err != nil {
			return err
		}
		if !ShouldDelete(answer) {
			return fmt.Errorf("workspace init aborted: %s", wsDir)
		}
		if err := AssertDisposable(wsDir, repos, remote, base); err != nil {
			return err
		}
		if err := RunPlan(out, BuildRemovePlan(wsDir, repos), verbose); err != nil {
			return err
		}
		if err := removeDir(wsDir); err != nil {
			return err
		}
	}
	return AssertTargets(wsDir, repos)
}
