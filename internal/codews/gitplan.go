package codews

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PlanArgs 是 init/add 的 worktree 编排参数。BranchExists 留给测试替换，避免真的问 git。
type PlanArgs struct {
	Branch       string
	Base         string
	Remote       string
	WsDir        string
	Repos        []Repo
	BranchExists func(Repo, string) bool
}

// SyncArgs 是把合并基线快进到远端的编排参数。
// Checkout 找到已检出的 worktree。git 失败要返回错误，不能当成「没有检出」。
type SyncArgs struct {
	Base     string
	Remote   string
	Repos    []Repo
	Checkout func(Repo, string) (string, bool, error)
}

// ForkArgs 从源 workspace 的 HEAD 派生新 worktree。
type ForkArgs struct {
	Src          string
	Dst          string
	Branch       string
	Repos        []Repo
	BranchExists func(Repo, string) bool
}

func repoBase(repo Repo, def string) string {
	if repo.Branch != "" {
		return repo.Branch
	}
	return def
}

func repoWsBranch(repo Repo, wsBranch string) string {
	if repo.Checkout != "" {
		return repo.Checkout
	}
	return wsBranch
}

func repoFetch(repo Repo, def string) string {
	if repo.Checkout != "" {
		return repo.Checkout
	}
	return repoBase(repo, def)
}

func repoSync(repo Repo, def string) string {
	if repo.Checkout != "" {
		return repo.Checkout
	}
	if repo.Merge != "" {
		return repo.Merge
	}
	return repoBase(repo, def)
}

func addWorktreeArgs(repo Repo, exists func(Repo, string) bool, dst, branch, remoteRef string) []string {
	if repo.Checkout != "" {
		return []string{"git", "worktree", "add", "--detach", dst, remoteRef}
	}
	if exists(repo, branch) {
		return []string{"git", "worktree", "add", dst, branch}
	}
	return []string{"git", "worktree", "add", "-b", branch, dst, remoteRef}
}

// BuildGitPlan 把 git 步骤列出来再执行，测试才能精确核对命令而不是看副作用。
func BuildGitPlan(args PlanArgs) []Cmd {
	exists := args.BranchExists
	if exists == nil {
		exists = localBranchExists
	}
	var cmds []Cmd
	for _, repo := range args.Repos {
		fetchBr := repoFetch(repo, args.Base)
		wtBr := repoWsBranch(repo, args.Branch)
		dst := filepath.Join(args.WsDir, repo.Name)
		cmds = append(cmds, Cmd{
			Repo:   repo.Name,
			Branch: "workspace",
			Step:   "worktree-prune",
			Cwd:    repo.Path,
			Args:   []string{"git", "worktree", "prune"},
		})
		cmds = append(cmds, Cmd{
			Repo:   repo.Name,
			Branch: fetchBr,
			Step:   "fetch",
			Cwd:    repo.Path,
			Args:   []string{"git", "fetch", args.Remote, fetchBr},
		})
		wtArgs := addWorktreeArgs(repo, exists, dst, wtBr, args.Remote+"/"+fetchBr)
		cmds = append(cmds, Cmd{
			Repo:   repo.Name,
			Branch: wtBr,
			Step:   "worktree",
			Cwd:    repo.Path,
			Args:   wtArgs,
		})
	}
	return cmds
}

// BuildSyncPlan 只更新本地基线引用。当前任务分支的 merge 留给人在各仓库里做。
func BuildSyncPlan(args SyncArgs) ([]Cmd, error) {
	checkout := args.Checkout
	if checkout == nil {
		checkout = lookupChecked
	}
	cmds := make([]Cmd, 0, len(args.Repos))
	for _, repo := range args.Repos {
		base := repoSync(repo, args.Base)
		path, ok, err := checkout(repo, base)
		if err != nil {
			return nil, err
		}
		if ok {
			cmds = append(cmds, Cmd{
				Repo:   repo.Name,
				Branch: base,
				Step:   "sync",
				Cwd:    path,
				Args:   []string{"git", "pull", "--ff-only", args.Remote, base},
			})
			continue
		}
		ref := base + ":" + base
		cmds = append(cmds, Cmd{
			Repo:   repo.Name,
			Branch: base,
			Step:   "sync",
			Cwd:    repo.Path,
			Args:   []string{"git", "fetch", args.Remote, ref},
		})
	}
	return cmds, nil
}

// BuildForkPlan 从源 worktree 的 HEAD 拉出新分支，已提交的进度才能带过去。
func BuildForkPlan(args ForkArgs) ([]Cmd, error) {
	exists := args.BranchExists
	if exists == nil {
		exists = localBranchExists
	}
	var cmds []Cmd
	for _, repo := range args.Repos {
		src := filepath.Join(args.Src, repo.Name)
		dstBr := repoWsBranch(repo, args.Branch)
		if repo.Checkout == "" && exists(repo, dstBr) {
			return nil, fmt.Errorf("branch already exists: %s %s", repo.Name, dstBr)
		}
		cmds = append(cmds, Cmd{
			Repo:   repo.Name,
			Branch: "workspace",
			Step:   "worktree-prune",
			Cwd:    repo.Path,
			Args:   []string{"git", "worktree", "prune"},
		})
		dst := filepath.Join(args.Dst, repo.Name)
		var wt []string
		if repo.Checkout == "" {
			wt = []string{"git", "worktree", "add", "-b", dstBr, dst, "HEAD"}
		} else {
			wt = []string{"git", "worktree", "add", "--detach", dst, "HEAD"}
		}
		cmds = append(cmds, Cmd{
			Repo:   repo.Name,
			Branch: dstBr,
			Step:   "worktree-fork",
			Cwd:    src,
			Args:   wt,
		})
	}
	return cmds, nil
}

// BuildRemovePlan 强制卸掉 worktree。已经不在 git 注册表里的路径忽略。
func BuildRemovePlan(wsDir string, repos []Repo) []Cmd {
	cmds := make([]Cmd, 0, len(repos))
	for _, repo := range repos {
		cmds = append(cmds, Cmd{
			Repo:    repo.Name,
			Branch:  "workspace",
			Step:    "worktree-remove",
			Cwd:     repo.Path,
			Args:    []string{"git", "worktree", "remove", "--force", filepath.Join(wsDir, repo.Name)},
			Ignored: []string{"is not a working tree"},
		})
	}
	return cmds
}

// ParseWorktreePath 从 porcelain 里找出某条本地分支当前被哪个 worktree 占用。
func ParseWorktreePath(text, branch string) string {
	path := ""
	for _, raw := range strings.Split(text, "\n") {
		raw = strings.TrimRight(raw, "\r")
		line := strings.TrimSpace(raw)
		if line == "" {
			path = ""
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			path = strings.TrimPrefix(line, "worktree ")
			continue
		}
		if line == "branch refs/heads/"+branch && path != "" {
			return path
		}
	}
	return ""
}

// RemoteNames 从 `git branch -r` 抽出短名。带箭头的符号引用不是可选分支。
func RemoteNames(text, remote string) []string {
	prefix := remote + "/"
	var names []string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if !strings.HasPrefix(line, prefix) || strings.Contains(line, " -> ") {
			continue
		}
		names = append(names, line[len(prefix):])
	}
	return names
}

// MatchedHead 要求远端 refs/heads 和用户写的名字大小写完全一致。
func MatchedHead(text, ref string) string {
	head := "refs/heads/" + ref
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		tab := strings.LastIndex(line, "\t")
		if tab < 0 {
			continue
		}
		if line[tab+1:] == head {
			return ref
		}
	}
	return ""
}

// ExistingDirMsg 是目标目录已经在时的拒绝文案。
func ExistingDirMsg(label, path string) string {
	return label + " already exists: " + path + "\ndelete it and continue? [y/N]"
}

// ConfirmDeletePrompt 问用户是否清掉上次失败留下的 workspace。
func ConfirmDeletePrompt(path string) string {
	return "workspace already exists: " + path + "\ndelete it and continue? [y/N] "
}

// ShouldDelete 只接受单字符 y，避免回车或 yes 误删。
func ShouldDelete(answer string) bool {
	return answer == "y" || answer == "Y"
}

// ConfirmRemovePrompt 在删 worktree 前列出后果。CLI 不能替用户判断分支有没有合完。
func ConfirmRemovePrompt(repo, path, base string) string {
	lines := []string{
		"remove repo from workspace: " + repo,
		"worktree path: " + path,
		"effects:",
		"  - runs git worktree remove --force for this repo",
		"  - removes it from project.yml and .code-workspace folders",
		"  - does not merge, push, or delete the git branch",
		"confirm you have merged the branch into " + base + ". continue? [y/N] ",
	}
	return strings.Join(lines, "\n")
}

// ShouldRemove 跟删除 workspace 一样，只认明确的 y。
func ShouldRemove(answer string) bool {
	return answer == "y" || answer == "Y"
}

// HasUnsafe 有未提交内容或还有未合入提交时不能删 worktree。
func HasUnsafe(status, commits string) bool {
	n := parseCount(commits)
	return strings.TrimSpace(status) != "" || n > 0
}

// HasDirty 只看工作区是不是干净。已提交但没 push 的内容对 fork 是合法起点。
func HasDirty(status string) bool {
	return strings.TrimSpace(status) != ""
}

// UnsafeMsg 把仓库、路径和原因绑在一起，destroy 汇总时才能对上是哪一仓。
func UnsafeMsg(repo, path, reason string) string {
	return "workspace cleanup aborted: repo=" + repo + " path=" + path + "\nreason: " + reason
}

// PatchMerged 没有独有 merge commit，且每个普通提交都有等价 patch 时可以清。
func PatchMerged(cherry, merges string) bool {
	if parseCount(merges) != 0 {
		return false
	}
	for _, raw := range strings.Split(cherry, "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "- ") {
			return false
		}
	}
	return true
}

func parseCount(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return -1
	}
	n := 0
	for _, ch := range text {
		if ch < '0' || ch > '9' {
			return -1
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// StepLabel 是执行前打到 stdout 的一行，方便对着仓库看卡在哪一步。
func StepLabel(cmd Cmd) string {
	return "[" + cmd.Repo + "] " + cmd.Step + " " + cmd.Branch
}

func summaryLine(text string) string {
	var lines []string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line != "" {
			lines = append(lines, line)
		}
	}
	for _, line := range lines {
		low := strings.ToLower(line)
		if strings.HasPrefix(low, "error:") || strings.HasPrefix(low, "fatal:") {
			return line
		}
	}
	if len(lines) == 0 {
		return "git exited with non-zero status"
	}
	return lines[len(lines)-1]
}

// GitFailure 把仓库、步骤和 git 的最后一行原因收成稳定文案。
func GitFailure(cmd Cmd, stderr string) string {
	return "git failed: repo=" + cmd.Repo + " branch=" + cmd.Branch + " step=" + cmd.Step + "\nreason: " + summaryLine(stderr)
}

// IgnoreFailure 用于 worktree 已经不在的删除。别的错误仍然要冒出来。
func IgnoreFailure(cmd Cmd, text string) bool {
	for _, item := range cmd.Ignored {
		if strings.Contains(text, item) {
			return true
		}
	}
	return false
}
