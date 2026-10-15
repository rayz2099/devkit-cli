package codews

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// WsProject 是 workspace 里已经定下来的分支和仓库覆盖。add/fork/sync 必须复用它。
type WsProject struct {
	Branch   string
	Repos    []string
	Checkout map[string]string
	Merges   map[string]string
}

type vsFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type vsFile struct {
	Folders  []vsFolder     `json:"folders"`
	Settings map[string]any `json:"settings"`
}

// WorkspaceName 只取分支末段。feature/spec101 这种 Git 名字不适合再套一层目录。
func WorkspaceName(branch string) (string, error) {
	parts := []string{}
	for _, part := range strings.Split(branch, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "", errors.New("branch must be a non-empty string")
	}
	return parts[len(parts)-1] + "-workspace", nil
}

// WorkspaceFileName 跟目录名同一末段，扩展名给 VS Code。
func WorkspaceFileName(branch string) (string, error) {
	name, err := WorkspaceName(branch)
	if err != nil {
		return "", err
	}
	base := strings.TrimSuffix(name, "-workspace")
	return base + ".code-workspace", nil
}

// CreateWorkspaceJSON 用相对路径，整个 workspace 目录搬走后 VS Code 仍能打开。
func CreateWorkspaceJSON(names []string) vsFile {
	folders := make([]vsFolder, 0, len(names))
	for _, name := range names {
		folders = append(folders, vsFolder{Name: name, Path: name})
	}
	return vsFile{Folders: folders, Settings: map[string]any{}}
}

func writeJSON(path string, v any, excl bool) error {
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if excl {
		flag = os.O_CREATE | os.O_WRONLY | os.O_EXCL
	}
	f, err := os.OpenFile(path, flag, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(buf)
	return err
}

// WriteWorkspaceFile 新建 .code-workspace。已存在就失败，避免覆盖用户改过的 folders。
func WriteWorkspaceFile(wsDir, branch string, names []string) (string, error) {
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return "", err
	}
	file, err := WorkspaceFileName(branch)
	if err != nil {
		return "", err
	}
	path := filepath.Join(wsDir, file)
	doc := CreateWorkspaceJSON(names)
	if err := writeJSON(path, doc, true); err != nil {
		return "", err
	}
	return file, nil
}

// CreateProjectYAML 把分支、描述和 merge 写全，agent 不用从缺省主分支推断合入目标。
func CreateProjectYAML(branch string, repos []Repo, top []string) string {
	lines := []string{"branch: " + branch}
	lines = append(lines, top...)
	lines = append(lines, "projects:")
	for _, repo := range repos {
		lines = append(lines, "  - repo-name: "+repo.Name)
		lines = append(lines, "    group: "+repo.Group)
		if repo.Desc != "" {
			lines = append(lines, "    description: "+repo.Desc)
		}
		if repo.Checkout != "" {
			lines = append(lines, "    branch: "+repo.Checkout)
		}
		merge := repo.Merge
		if merge == "" {
			merge = repo.Branch
		}
		lines = append(lines, "    merge: "+merge)
	}
	return strings.Join(lines, "\n") + "\n"
}

func customTopLines(txt string) []string {
	var lines []string
	for _, raw := range strings.Split(txt, "\n") {
		raw = strings.TrimRight(raw, "\r")
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if line == "projects:" {
			break
		}
		if strings.HasPrefix(line, "branch:") {
			continue
		}
		if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
			continue
		}
		lines = append(lines, raw)
	}
	return lines
}

// WriteProjectFile 每次重写描述文件，因为它是当前仓库集合的派生结果。顶层业务变量会留下来。
func WriteProjectFile(wsDir, branch string, repos []Repo) (string, error) {
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(wsDir, "project.yml")
	top := []string{"it-env-no:"}
	if buf, err := os.ReadFile(path); err == nil {
		top = customTopLines(string(buf))
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	txt := CreateProjectYAML(branch, repos, top)
	if err := os.WriteFile(path, []byte(txt), 0o644); err != nil {
		return "", err
	}
	return "project.yml", nil
}

var (
	reTopBr    = regexp.MustCompile(`^branch:\s*(.+)$`)
	reRepoBr   = regexp.MustCompile(`^branch:\s*(.+)$`)
	reMerge    = regexp.MustCompile(`^merge:\s*(.+)$`)
	reLegacy   = regexp.MustCompile(`^checkout-branch:\s*(.+)$`)
	reRepoName = regexp.MustCompile(`^-\s+repo-name:\s*(.+)$`)
)

// ParseWsProject 读 workspace 固定分支和每个仓库的 checkout/merge 覆盖。
func ParseWsProject(txt string) (WsProject, error) {
	branch := ""
	var repos []string
	checkout := map[string]string{}
	merges := map[string]string{}
	cur := ""
	for _, raw := range strings.Split(txt, "\n") {
		raw = strings.TrimRight(raw, "\r")
		line := strings.TrimSpace(raw)
		if m := reTopBr.FindStringSubmatch(raw); m != nil {
			branch = m[1]
			continue
		}
		if m := reRepoBr.FindStringSubmatch(line); m != nil && cur != "" && !strings.HasPrefix(raw, "branch:") {
			// 缩进的 branch 才是仓库 checkout。顶层已在上面吃掉。
		}
		if m := reRepoBr.FindStringSubmatch(line); m != nil && cur != "" {
			if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") || !strings.HasPrefix(raw, "branch:") {
				if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
					checkout[cur] = m[1]
					continue
				}
			}
		}
		if m := reMerge.FindStringSubmatch(line); m != nil && cur != "" {
			if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
				merges[cur] = m[1]
				continue
			}
		}
		if m := reLegacy.FindStringSubmatch(line); m != nil && cur != "" {
			checkout[cur] = m[1]
			continue
		}
		if m := reRepoName.FindStringSubmatch(line); m != nil {
			cur = m[1]
			repos = append(repos, cur)
		}
	}
	if branch == "" {
		return WsProject{}, errors.New("workspace project.yml branch must be a non-empty string")
	}
	return WsProject{Branch: branch, Repos: repos, Checkout: checkout, Merges: merges}, nil
}

// FindWorkspace 从子目录往上找 project.yml，让命令可以在仓库里面跑。
func FindWorkspace(start string) (string, error) {
	cur := start
	for {
		if _, err := os.Stat(filepath.Join(cur, "project.yml")); err == nil {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("workspace project.yml not found from: %s", start)
		}
		cur = parent
	}
}

func workspaceFilePath(wsDir string) (string, error) {
	ents, err := os.ReadDir(wsDir)
	if err != nil {
		return "", err
	}
	var matched []string
	for _, ent := range ents {
		if strings.HasSuffix(ent.Name(), ".code-workspace") {
			matched = append(matched, ent.Name())
		}
	}
	if len(matched) != 1 {
		return "", fmt.Errorf("workspace file must be unique in: %s", wsDir)
	}
	return filepath.Join(wsDir, matched[0]), nil
}

// AddRepoJSON 把新 worktree 补进 VS Code folders，否则编辑器里看不到。
func AddRepoJSON(wsDir, name string) (string, error) {
	file, err := workspaceFilePath(wsDir)
	if err != nil {
		return "", err
	}
	buf, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	var doc vsFile
	if err := json.Unmarshal(buf, &doc); err != nil {
		return "", err
	}
	if doc.Settings == nil {
		doc.Settings = map[string]any{}
	}
	for _, folder := range doc.Folders {
		if folder.Name == name {
			return "", fmt.Errorf("repo already exists in workspace: %s", name)
		}
	}
	doc.Folders = append(doc.Folders, vsFolder{Name: name, Path: name})
	if err := writeJSON(file, doc, false); err != nil {
		return "", err
	}
	return filepath.Base(file), nil
}

// RemoveRepoJSON 从 folders 删掉仓库。先改内存里的结构，测试不必先落盘。
func RemoveRepoJSON(doc vsFile, name string) (vsFile, error) {
	next := make([]vsFolder, 0, len(doc.Folders))
	found := false
	for _, folder := range doc.Folders {
		if folder.Name == name {
			found = true
			continue
		}
		next = append(next, folder)
	}
	if !found {
		return vsFile{}, fmt.Errorf("repo not found in workspace: %s", name)
	}
	doc.Folders = next
	if doc.Settings == nil {
		doc.Settings = map[string]any{}
	}
	return doc, nil
}

// RemoveRepoFile 同步删掉 VS Code folder，避免编辑器还打开已经移除的路径。
func RemoveRepoFile(wsDir, name string) (string, error) {
	file, err := workspaceFilePath(wsDir)
	if err != nil {
		return "", err
	}
	buf, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	var doc vsFile
	if err := json.Unmarshal(buf, &doc); err != nil {
		return "", err
	}
	next, err := RemoveRepoJSON(doc, name)
	if err != nil {
		return "", err
	}
	if err := writeJSON(file, next, false); err != nil {
		return "", err
	}
	return filepath.Base(file), nil
}
