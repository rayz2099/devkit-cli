package rclone

import (
	"os"
	"path/filepath"
	"strings"
)

// 内置排除写死在命令里。每个 target 再抄一份会漏掉 .grok，也会把工作目录传上去。
var builtinExcludes = []string{".git/**", ".grok/**", ".DS_Store"}

// 根上的点前缀和子目录里的点前缀要两条规则，目录命中后 rclone 不再往里走。
var hiddenExcludes = []string{".*", "**/.*"}

// Plan 是已经合并好的一次传输。拼 argv 时不再读任务覆盖。
type Plan struct {
	Action    string
	Task      Task
	Target    Target
	Local     string
	Rel       string
	Dest      string
	Label     string
	Excludes  []string
	Transfers float64
	Tpslimit  float64
	Timeout   string
	NameEnc   string
	DryRun    bool
	Progress  bool
	Verbose   bool
}

type planIn struct {
	cfg      *FileCfg
	action   string
	task     string
	given    string
	hasGiven bool
	dryRun   bool
	audience string
	dest     string
	hasDest  bool
	cwd      string
	verbose  bool
}

func findTask(cfg *FileCfg, name string) (Task, error) {
	for _, task := range cfg.Tasks {
		if task.Name == name {
			return task, nil
		}
	}
	names := strings.Join(taskNames(cfg), ", ")
	suffix := ""
	if names != "" {
		suffix = ". available: " + names
	}
	return Task{}, fail(1, "task not found: %s%s", name, suffix)
}

// StripBin 去掉用户带上的旧 .bin。远端逻辑名是没有后缀的那一个。
func StripBin(rel string) string {
	if len(rel) > 4 && strings.HasSuffix(strings.ToLower(rel), ".bin") {
		return rel[:len(rel)-4]
	}
	return rel
}

func makePlan(in planIn) (*Plan, error) {
	task, err := findTask(in.cfg, in.task)
	if err != nil {
		return nil, err
	}
	source, ok := findSource(in.cfg, task.Source)
	if !ok {
		return nil, fail(1, "tasks.%s.source not found: %s", task.Name, task.Source)
	}
	target, ok := targetNamed(in.cfg.Targets, task.Target)
	if !ok {
		return nil, fail(1, "tasks.%s.target not found: %s", task.Name, task.Target)
	}
	srcName := filepath.Base(source.Path)
	if target.Root != srcName {
		return nil, fail(1, "targets.%s.root must match source folder %s", target.Name, srcName)
	}
	given := ""
	if in.hasGiven {
		given = in.given
	}
	if (in.action == "get" || in.action == "download") && given == "" {
		return nil, fail(2, "%s requires a path", in.action)
	}
	local, rel, err := resolveRel(source.Path, given)
	if err != nil {
		return nil, err
	}
	if in.action == "get" || in.action == "download" {
		local = StripBin(local)
		rel = StripBin(rel)
	}
	dest := joinRoot(target.Root, rel)
	encNames := target.EncryptFilenames
	if task.EncryptFilenames != nil {
		encNames = *task.EncryptFilenames
	}
	nameEnc := "off"
	if encNames {
		nameEnc = "standard"
	}
	writeLocal, err := resolveWriteLocal(in, local, rel)
	if err != nil {
		return nil, err
	}
	if err := assertPullTree(in.action, task.Name, rel, writeLocal); err != nil {
		return nil, err
	}
	skipHidden := target.SkipHidden
	if task.SkipHidden != nil {
		skipHidden = *task.SkipHidden
	}
	transfers := target.Transfers
	if task.Transfers != nil {
		transfers = *task.Transfers
	}
	tps := target.Tpslimit
	if task.Tpslimit != nil {
		tps = *task.Tpslimit
	}
	timeout := target.Timeout
	if task.Timeout != nil {
		timeout = *task.Timeout
	}
	progress := in.audience == "human" && in.action != "check" && (in.verbose || (in.action != "get" && in.action != "download"))
	return &Plan{
		Action: in.action, Task: task, Target: target, Local: writeLocal, Rel: rel,
		Dest: dest, Label: target.Name + ":" + dest, Excludes: mergeExcludes(task.Excludes, skipHidden),
		Transfers: transfers, Tpslimit: tps, Timeout: timeout, NameEnc: nameEnc,
		DryRun: in.dryRun, Progress: progress, Verbose: in.verbose,
	}, nil
}

func findSource(cfg *FileCfg, name string) (Source, bool) {
	for _, item := range cfg.Sources {
		if item.Name == name {
			return item, true
		}
	}
	return Source{}, false
}

func resolveRel(root, given string) (string, string, error) {
	base := filepath.Clean(root)
	if given == "" {
		return base, "", nil
	}
	local := given
	if !filepath.IsAbs(given) {
		local = filepath.Join(base, given)
	}
	local = filepath.Clean(local)
	rel, err := filepath.Rel(base, local)
	if err != nil {
		return "", "", fail(1, "path outside source: %s", given)
	}
	if rel == "." {
		return base, "", nil
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", "", fail(1, "path outside source: %s", given)
	}
	return local, filepath.ToSlash(rel), nil
}

func joinRoot(root, rel string) string {
	return joinDest(root, rel)
}

func resolveWriteLocal(in planIn, locatedLocal, rel string) (string, error) {
	if in.action != "download" {
		if !in.hasDest || in.dest == "" {
			return locatedLocal, nil
		}
		return absIn(in.cwd, in.dest), nil
	}
	name := filepath.Base(rel)
	if name == "" || name == "." || name == ".." {
		return "", fail(2, "download requires a file path")
	}
	if !in.hasDest || in.dest == "" {
		return absIn(in.cwd, name), nil
	}
	dest := absIn(in.cwd, in.dest)
	info, err := os.Stat(dest)
	if err == nil && info.IsDir() {
		return filepath.Join(dest, name), nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fail(1, "dest: %s", err.Error())
	}
	return dest, nil
}

func absIn(cwd, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(cwd, path))
}

func assertPullTree(action, task, rel, local string) error {
	if action != "pull" {
		return nil
	}
	info, err := os.Stat(local)
	isFile := false
	if err == nil {
		isFile = info.Mode().IsRegular()
	} else if os.IsNotExist(err) {
		isFile = extName(local) != ""
	} else {
		return fail(1, "stat: %s", err.Error())
	}
	if !isFile {
		return nil
	}
	return fail(2, "pull expects a directory. download one file: rclone-cli download %s %s", task, rel)
}

func extName(path string) string {
	base := filepath.Base(path)
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return ""
	}
	return base[dot:]
}

func mergeExcludes(extra []string, skipHidden bool) []string {
	out := append([]string{}, builtinExcludes...)
	if skipHidden {
		for _, item := range hiddenExcludes {
			if !containsStr(out, item) {
				out = append(out, item)
			}
		}
	}
	for _, item := range extra {
		if !containsStr(out, item) {
			out = append(out, item)
		}
	}
	return out
}
