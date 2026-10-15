package codews

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func readAnswer(in io.Reader, out io.Writer, msg string) (string, error) {
	fmt.Fprint(out, msg)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func profileOf(cfg Cfg, name string) (Profile, error) {
	prof, ok := cfg.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("profile not found: %s", name)
	}
	return prof, nil
}

func repoOfName(cfg Cfg, name string) (Repo, error) {
	for _, repo := range cfg.Projects {
		if repo.Name == name {
			return repo, nil
		}
	}
	return Repo{}, fmt.Errorf("repo not found in project.yml: %s", name)
}

func withCheckout(repo Repo, branch string, ok bool) Repo {
	if !ok {
		return repo
	}
	repo.Checkout = branch
	return repo
}

func withMerge(repo Repo, merge string) Repo {
	if merge == "" {
		return repo
	}
	repo.Merge = merge
	return repo
}

func reposFromWs(cfg Cfg, ws WsProject) ([]Repo, error) {
	out := make([]Repo, 0, len(ws.Repos))
	for _, name := range ws.Repos {
		repo, err := repoOfName(cfg, name)
		if err != nil {
			return nil, err
		}
		if br, ok := ws.Checkout[name]; ok {
			repo = withCheckout(repo, br, true)
		}
		if merge, ok := ws.Merges[name]; ok {
			repo = withMerge(repo, merge)
		}
		out = append(out, repo)
	}
	return out, nil
}

func reposMerged(repos []Repo, from []string, remote string) ([]Repo, error) {
	specs, err := ParseFrom(from)
	if err != nil {
		return nil, err
	}
	return ApplyMerge(repos, specs, func(repo Repo, ref string) (string, error) {
		return LookupRemote(repo.Path, remote, ref)
	})
}

func checkAgents(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("agents template not found: %s", path)
	}
	return nil
}

func checkRepos(repos []Repo) error {
	for _, repo := range repos {
		if _, err := os.Stat(repo.Path); err != nil {
			return fmt.Errorf("repo not found: %s", repo.Path)
		}
	}
	return nil
}

// InitTarget 显式 project 替换 profile 的仓库集合，模板仍从 profile 取。
func InitTarget(cfg Cfg, args Parsed) (string, []Repo, error) {
	if args.HasProf {
		prof, err := profileOf(cfg, args.Profile)
		if err != nil {
			return "", nil, err
		}
		repos := prof.Repos
		if args.HasProj {
			repo, err := repoOfName(cfg, args.Project)
			if err != nil {
				return "", nil, err
			}
			repos = []Repo{repo}
		}
		return prof.Agents, repos, nil
	}
	repo, err := repoOfName(cfg, args.Project)
	if err != nil {
		return "", nil, err
	}
	return cfg.InitTpl, []Repo{repo}, nil
}

func runInit(in io.Reader, out io.Writer, args Parsed) error {
	cfg, err := cfgOf(args.Config)
	if err != nil {
		return err
	}
	tpl, repos, err := InitTarget(cfg, args)
	if err != nil {
		return err
	}
	if err := checkAgents(tpl); err != nil {
		return err
	}
	if err := checkRepos(repos); err != nil {
		return err
	}
	merged, err := reposMerged(repos, args.From, cfg.Remote)
	if err != nil {
		return err
	}
	wsName, err := WorkspaceName(args.Branch)
	if err != nil {
		return err
	}
	wsDir := filepath.Join(cfg.WsRoot, wsName)
	read := func(msg string) (string, error) {
		return readAnswer(in, out, msg)
	}
	if err := PrepareTargets(out, wsDir, merged, cfg.Remote, cfg.Base, args.Verbose, read); err != nil {
		return err
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return err
	}
	plan := BuildGitPlan(PlanArgs{
		Branch: args.Branch,
		Base:   cfg.Base,
		Remote: cfg.Remote,
		WsDir:  wsDir,
		Repos:  merged,
	})
	if err := RunPlan(out, plan, args.Verbose); err != nil {
		return err
	}
	if err := ApplyAgents(tpl, wsDir); err != nil {
		return err
	}
	names := make([]string, 0, len(merged))
	for _, repo := range merged {
		names = append(names, repo.Name)
	}
	file, err := WriteWorkspaceFile(wsDir, args.Branch, names)
	if err != nil {
		return err
	}
	if _, err := WriteProjectFile(wsDir, args.Branch, merged); err != nil {
		return err
	}
	fmt.Fprintf(out, "workspace created: %s\n", filepath.Join(wsDir, file))
	fmt.Fprintf(out, "workspace directory: %s\n", wsDir)
	return nil
}

func runList(out io.Writer, args Parsed) error {
	cfg, err := cfgOf(args.Config)
	if err != nil {
		return err
	}
	for _, name := range cfg.Order {
		prof := cfg.Profiles[name]
		repos := make([]string, 0, len(prof.Repos))
		for _, repo := range prof.Repos {
			repos = append(repos, repo.Name)
		}
		fmt.Fprintf(out, "%s: %s\n", name, strings.Join(repos, ", "))
	}
	return nil
}

func runProjects(out io.Writer, args Parsed) error {
	cfg, err := cfgOf(args.Config)
	if err != nil {
		return err
	}
	if args.Workspace {
		wsDir, err := FindWorkspace(cwd())
		if err != nil {
			return err
		}
		buf, err := os.ReadFile(filepath.Join(wsDir, "project.yml"))
		if err != nil {
			return err
		}
		ws, err := ParseWsProject(string(buf))
		if err != nil {
			return err
		}
		for _, name := range ws.Repos {
			fmt.Fprintln(out, name)
		}
		return nil
	}
	if args.HasProf {
		prof, err := profileOf(cfg, args.Profile)
		if err != nil {
			return err
		}
		for _, repo := range prof.Repos {
			fmt.Fprintln(out, repo.Name)
		}
		return nil
	}
	for _, repo := range cfg.Projects {
		fmt.Fprintln(out, repo.Name)
	}
	return nil
}

func runBranches(out io.Writer, args Parsed) error {
	cfg, err := cfgOf(args.Config)
	if err != nil {
		return err
	}
	repo, err := repoOfName(cfg, args.Repo)
	if err != nil {
		return err
	}
	names, err := ListRemote(repo.Path, cfg.Remote)
	if err != nil {
		return err
	}
	for _, name := range names {
		fmt.Fprintln(out, name)
	}
	return nil
}

func loadWs(cfgPath string) (Cfg, string, WsProject, error) {
	cfg, err := cfgOf(cfgPath)
	if err != nil {
		return Cfg{}, "", WsProject{}, err
	}
	wsDir, err := FindWorkspace(cwd())
	if err != nil {
		return Cfg{}, "", WsProject{}, err
	}
	buf, err := os.ReadFile(filepath.Join(wsDir, "project.yml"))
	if err != nil {
		return Cfg{}, "", WsProject{}, err
	}
	ws, err := ParseWsProject(string(buf))
	if err != nil {
		return Cfg{}, "", WsProject{}, err
	}
	return cfg, wsDir, ws, nil
}

func runAdd(out io.Writer, args Parsed) error {
	cfg, wsDir, ws, err := loadWs(args.Config)
	if err != nil {
		return err
	}
	repo, err := repoOfName(cfg, args.Repo)
	if err != nil {
		return err
	}
	repo = withCheckout(repo, args.Branch, args.HasBranch)
	for _, name := range ws.Repos {
		if name == repo.Name {
			return fmt.Errorf("repo already exists in workspace project.yml: %s", repo.Name)
		}
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return err
	}
	plan := BuildGitPlan(PlanArgs{
		Branch: ws.Branch,
		Base:   cfg.Base,
		Remote: cfg.Remote,
		WsDir:  wsDir,
		Repos:  []Repo{repo},
	})
	if err := RunPlan(out, plan, args.Verbose); err != nil {
		return err
	}
	if _, err := AddRepoJSON(wsDir, repo.Name); err != nil {
		return err
	}
	current, err := reposFromWs(cfg, ws)
	if err != nil {
		return err
	}
	current = append(current, repo)
	if _, err := WriteProjectFile(wsDir, ws.Branch, current); err != nil {
		return err
	}
	fmt.Fprintf(out, "project added: %s\n", filepath.Join(wsDir, repo.Name))
	return nil
}

func runRemove(in io.Reader, out io.Writer, args Parsed) error {
	cfg, wsDir, ws, err := loadWs(args.Config)
	if err != nil {
		return err
	}
	repo, err := repoOfName(cfg, args.Repo)
	if err != nil {
		return err
	}
	found := false
	for _, name := range ws.Repos {
		if name == repo.Name {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("repo not found in workspace project.yml: %s", repo.Name)
	}
	work := filepath.Join(wsDir, repo.Name)
	cur := withMerge(repo, ws.Merges[repo.Name])
	merge := cur.Merge
	if merge == "" {
		merge = cur.Branch
	}
	answer, err := readAnswer(in, out, ConfirmRemovePrompt(repo.Name, work, merge))
	if err != nil {
		return err
	}
	if !ShouldRemove(answer) {
		return fmt.Errorf("project remove aborted: %s", repo.Name)
	}
	plan := BuildRemovePlan(wsDir, []Repo{repo})
	if err := RunPlan(out, plan, args.Verbose); err != nil {
		return err
	}
	if _, err := RemoveRepoFile(wsDir, repo.Name); err != nil {
		return err
	}
	var nextNames []string
	for _, name := range ws.Repos {
		if name != repo.Name {
			nextNames = append(nextNames, name)
		}
	}
	ws.Repos = nextNames
	next, err := reposFromWs(cfg, ws)
	if err != nil {
		return err
	}
	if _, err := WriteProjectFile(wsDir, ws.Branch, next); err != nil {
		return err
	}
	fmt.Fprintf(out, "project removed: %s\n", work)
	return nil
}

func runSync(out, errw io.Writer, args Parsed) error {
	cfg, wsDir, ws, err := loadWs(args.Config)
	if err != nil {
		return err
	}
	repos, err := reposFromWs(cfg, ws)
	if err != nil {
		return err
	}
	if len(args.From) > 0 {
		repos, err = reposMerged(repos, args.From, cfg.Remote)
		if err != nil {
			return err
		}
		if _, err := WriteProjectFile(wsDir, ws.Branch, repos); err != nil {
			return err
		}
	}
	plan, err := BuildSyncPlan(SyncArgs{Base: cfg.Base, Remote: cfg.Remote, Repos: repos})
	if err != nil {
		return err
	}
	failures := RunPlanKeepGoing(out, errw, plan, args.Verbose)
	if failures > 0 {
		return fmt.Errorf("base branch sync failed: %d repo(s)", failures)
	}
	fmt.Fprintf(out, "base branches synced: %s\n", wsDir)
	return nil
}

func runDestroy(out io.Writer, args Parsed) error {
	cfg, wsDir, ws, err := loadWs(args.Config)
	if err != nil {
		return err
	}
	return DestroyWorkspace(out, args, cfg, wsDir, ws)
}

// DestroyWorkspace 只卸 git worktree。文档留在目录里，后面淬炼还要读。
func DestroyWorkspace(out io.Writer, args Parsed, cfg Cfg, wsDir string, ws WsProject) error {
	repos, err := reposFromWs(cfg, ws)
	if err != nil {
		return err
	}
	var failures []string
	cleared := 0
	for _, repo := range repos {
		err := AssertDisposable(wsDir, []Repo{repo}, cfg.Remote, cfg.Base)
		if err == nil {
			err = RunPlan(out, BuildRemovePlan(wsDir, []Repo{repo}), args.Verbose)
		}
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		cleared++
	}
	if len(failures) > 0 {
		fmt.Fprintf(out, "workspace worktrees cleared: %d/%d\n", cleared, len(repos))
		fmt.Fprintf(out, "workspace directory kept: %s\n", wsDir)
		head := fmt.Sprintf("workspace cleanup incomplete: cleared=%d failed=%d", cleared, len(failures))
		parts := append([]string{head}, failures...)
		return errors.New(strings.Join(parts, "\n\n"))
	}
	fmt.Fprintf(out, "workspace worktrees cleared: %s\n", wsDir)
	fmt.Fprintf(out, "workspace directory kept: %s\n", wsDir)
	return nil
}

func runFork(in io.Reader, out io.Writer, args Parsed) error {
	cfg, wsDir, ws, err := loadWs(args.Config)
	if err != nil {
		return err
	}
	return ForkWorkspace(in, out, args, cfg, wsDir, ws)
}

// ForkWorkspace 从当前 workspace 派生后续分支。源仓库必须干净，否则新 worktree 会丢掉未提交内容。
func ForkWorkspace(in io.Reader, out io.Writer, args Parsed, cfg Cfg, src string, ws WsProject) error {
	repos, err := reposFromWs(cfg, ws)
	if err != nil {
		return err
	}
	if err := AssertClean(src, repos); err != nil {
		return err
	}
	name, err := WorkspaceName(args.Branch)
	if err != nil {
		return err
	}
	dst := filepath.Join(cfg.WsRoot, name)
	if _, err := os.Stat(dst); err == nil {
		answer, err := readAnswer(in, out, ConfirmDeletePrompt(dst))
		if err != nil {
			return err
		}
		if !ShouldDelete(answer) {
			return fmt.Errorf("workspace fork aborted: %s", dst)
		}
		if err := AssertDisposable(dst, repos, cfg.Remote, cfg.Base); err != nil {
			return err
		}
		if err := RunPlan(out, BuildRemovePlan(dst, repos), args.Verbose); err != nil {
			return err
		}
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	if err := AssertTargets(dst, repos); err != nil {
		return err
	}
	plan, err := BuildForkPlan(ForkArgs{
		Src:    src,
		Dst:    dst,
		Branch: args.Branch,
		Repos:  repos,
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := RunPlan(out, plan, args.Verbose); err != nil {
		return err
	}
	if _, err := CopyForkEntries(src, dst); err != nil {
		return err
	}
	if err := ApplyAgents(cfg.InitTpl, dst); err != nil {
		return err
	}
	names := make([]string, 0, len(repos))
	for _, repo := range repos {
		names = append(names, repo.Name)
	}
	file, err := WriteWorkspaceFile(dst, args.Branch, names)
	if err != nil {
		return err
	}
	if _, err := WriteProjectFile(dst, args.Branch, repos); err != nil {
		return err
	}
	fmt.Fprintf(out, "workspace forked: %s\n", filepath.Join(dst, file))
	return nil
}

func runCheck(out io.Writer, args Parsed) error {
	cfg, err := cfgOf(args.Config)
	if err != nil {
		return err
	}
	for _, name := range cfg.Order {
		prof := cfg.Profiles[name]
		if err := checkAgents(prof.Agents); err != nil {
			return err
		}
		if err := checkRepos(prof.Repos); err != nil {
			return err
		}
	}
	fmt.Fprintln(out, "config ok")
	return nil
}

func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
