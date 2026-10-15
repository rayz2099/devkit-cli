package codeup

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// RunGit 走同一条 git 子进程。push 和当前仓库解析不能各写一套。
func RunGit(cwd string, args []string) (string, error) {
	dir := cwd
	if dir == "" {
		got, err := os.Getwd()
		if err != nil {
			return "", fail(1, err.Error())
		}
		dir = got
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = "git " + strings.Join(args, " ") + " failed"
		}
		return "", fail(1, detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// ResolveRepoFromCwd 用 origin 推断 group/project。主机不是云效就要求显式 --repo。
func ResolveRepoFromCwd(orgID, cwd string) (string, error) {
	raw, err := RunGit(cwd, []string{"remote", "get-url", "origin"})
	if err != nil {
		return "", err
	}
	if raw == "" {
		return "", fail(1, "no git origin remote in cwd; pass --repo")
	}
	remote, err := ParseGitRemote(raw)
	if err != nil {
		return "", err
	}
	if remote.Host != CodeupHost {
		return "", failf(1, "origin host is not %s (%s); pass --repo", CodeupHost, remote.Host)
	}
	return RepoFromRemotePath(remote.Path, orgID)
}

// CurrentBranch 读 HEAD。分离头指针时不能猜分支名。
func CurrentBranch(cwd string) (string, error) {
	branch, err := RunGit(cwd, []string{"rev-parse", "--abbrev-ref", "HEAD"})
	if err != nil {
		return "", err
	}
	if branch == "" || branch == "HEAD" {
		return "", fail(1, "detached HEAD; pass an explicit branch to push")
	}
	return branch, nil
}

// PushBranch 先核对 remote 主机再推。不是云效就不调用 push。
func PushBranch(remote, branch, cwd string) error {
	raw, err := RunGit(cwd, []string{"remote", "get-url", remote})
	if err != nil {
		return err
	}
	parsed, err := ParseGitRemote(raw)
	if err != nil {
		return err
	}
	if parsed.Host != CodeupHost {
		return failf(1, "remote %s is not %s (%s)", remote, CodeupHost, parsed.Host)
	}
	_, err = RunGit(cwd, []string{"push", remote, branch})
	return err
}

type repoPick struct {
	OrgID   string
	Profile string
	Flag    string
	Pos     string
	Cwd     string
}

// ResolveRepo 按 flag、位置参数、cwd 的顺序定仓库。短名只查索引。
func ResolveRepo(q repoPick) (string, error) {
	if strings.TrimSpace(q.Flag) != "" {
		return expandRepo(q.OrgID, q.Profile, strings.TrimSpace(q.Flag))
	}
	if strings.TrimSpace(q.Pos) != "" {
		return expandRepo(q.OrgID, q.Profile, strings.TrimSpace(q.Pos))
	}
	return ResolveRepoFromCwd(q.OrgID, q.Cwd)
}

func expandRepo(orgID, profile, raw string) (string, error) {
	if digitRE.MatchString(raw) {
		return raw, nil
	}
	parts := splitPath(raw)
	if len(parts) > 0 && parts[0] == orgID {
		parts = parts[1:]
	}
	if len(parts) >= 2 {
		return strings.Join(parts, "/"), nil
	}
	idx, err := LoadIndex(profile, orgID)
	if err != nil {
		return "", err
	}
	hit, err := PickCachedRepo(raw, idx.Repos)
	if err != nil {
		return "", err
	}
	return hit.PathNs, nil
}
