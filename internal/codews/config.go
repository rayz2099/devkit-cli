package codews

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
)

const configExample = "code-ws/config.example.json"

// RawCfg 是 config.json 的业务引用，还没展开 project.yml。
type RawCfg struct {
	WsRoot  string
	Base    string
	Remote  string
	InitTpl string
	Order   []string
	Profs   map[string]rawProf
}

type rawProf struct {
	Name   string
	Agents string
	Repos  []string
}

type projEntry struct {
	Name  string
	Group string
	Br    string
	Desc  string
}

// ResolveOpts 是把引用展开成路径时的根。模板只从配置目录取，不跟 git checkout 走。
type ResolveOpts struct {
	Home string
	Tpl  string
}

// DefaultConfig 是没传 -c 时的配置文件。只认 XDG_CONFIG_HOME 和 HOME，不猜其他环境变量。
func DefaultConfig() (string, error) {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		return filepath.Join(xdg, "code-ws", "config.json"), nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("HOME must be set")
	}
	return filepath.Join(home, ".config", "code-ws", "config.json"), nil
}

// ConfigPath 让 -c 覆盖默认位置。空字符串表示用 DefaultConfig。
func ConfigPath(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	return DefaultConfig()
}

// ParseConfigBytes 保留 profiles 的键顺序，list 输出才跟 JSON 文件一致。
func ParseConfigBytes(buf []byte) (RawCfg, error) {
	var raw any
	if err := cfgfile.ParseJSON(buf, &raw); err != nil {
		return RawCfg{}, err
	}
	obj, ok := asObj(raw)
	if !ok {
		return RawCfg{}, errors.New("config must be an object")
	}
	ws, err := reqStr(obj, "workspaceRoot")
	if err != nil {
		return RawCfg{}, err
	}
	base, err := reqStr(obj, "baseBranch")
	if err != nil {
		return RawCfg{}, err
	}
	remote, err := reqStr(obj, "remote")
	if err != nil {
		return RawCfg{}, err
	}
	tpl := ""
	if v, ok := obj["initAgentsTemplate"]; ok && v != nil {
		str, ok := v.(string)
		if !ok {
			return RawCfg{}, errors.New("initAgentsTemplate must be a string")
		}
		tpl = strings.TrimSpace(str)
	}
	if _, ok := asObj(obj["profiles"]); !ok {
		return RawCfg{}, errors.New("profiles must be an object")
	}
	order, blobs, err := orderedObjects(buf, "profiles")
	if err != nil {
		return RawCfg{}, err
	}
	profs := map[string]rawProf{}
	for _, key := range order {
		prof, err := parseProf(blobs[key])
		if err != nil {
			return RawCfg{}, err
		}
		profs[key] = prof
	}
	return RawCfg{
		WsRoot:  ws,
		Base:    base,
		Remote:  remote,
		InitTpl: tpl,
		Order:   order,
		Profs:   profs,
	}, nil
}

func parseProf(buf []byte) (rawProf, error) {
	var raw any
	if err := json.Unmarshal(buf, &raw); err != nil {
		return rawProf{}, errors.New("profile must be an object")
	}
	obj, ok := asObj(raw)
	if !ok {
		return rawProf{}, errors.New("profile must be an object")
	}
	name, err := reqStr(obj, "name")
	if err != nil {
		return rawProf{}, err
	}
	agents := ""
	if v, ok := obj["agentsTemplate"]; ok && v != nil {
		str, ok := v.(string)
		if !ok {
			return rawProf{}, errors.New("agentsTemplate must be a string")
		}
		agents = strings.TrimSpace(str)
	}
	reposRaw, ok := obj["repos"].([]any)
	if !ok {
		if obj["repos"] != nil {
			return rawProf{}, errors.New("profile.repos must be an array")
		}
		reposRaw = []any{}
	}
	repos := make([]string, 0, len(reposRaw))
	for _, item := range reposRaw {
		name, ok := item.(string)
		if !ok || strings.TrimSpace(name) == "" {
			return rawProf{}, errors.New("repo must be a non-empty string")
		}
		repos = append(repos, name)
	}
	return rawProf{Name: name, Agents: agents, Repos: repos}, nil
}

// orderedObjects 只抽出顶层某个对象字段的键顺序。encoding/json 的 map 会打乱它。
func orderedObjects(buf []byte, field string) ([]string, map[string][]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(buf))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, fmt.Errorf("invalid config: %w", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, errors.New("config must be an object")
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, nil, fmt.Errorf("invalid config: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, nil, errors.New("config must be an object")
		}
		if key != field {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return nil, nil, fmt.Errorf("invalid config: %w", err)
			}
			continue
		}
		return decodeObj(dec)
	}
	return nil, nil, errors.New("profiles must be an object")
}

func decodeObj(dec *json.Decoder) ([]string, map[string][]byte, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, fmt.Errorf("invalid config: %w", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, errors.New("profiles must be an object")
	}
	order := []string{}
	blobs := map[string][]byte{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, nil, fmt.Errorf("invalid config: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, nil, errors.New("profiles must be an object")
		}
		var body json.RawMessage
		if err := dec.Decode(&body); err != nil {
			return nil, nil, fmt.Errorf("invalid config: %w", err)
		}
		if _, seen := blobs[key]; !seen {
			order = append(order, key)
		}
		blobs[key] = body
	}
	end, err := dec.Token()
	if err != nil {
		return nil, nil, fmt.Errorf("invalid config: %w", err)
	}
	if d, ok := end.(json.Delim); !ok || d != '}' {
		return nil, nil, errors.New("profiles must be an object")
	}
	return order, blobs, nil
}

func asObj(v any) (map[string]any, bool) {
	obj, ok := v.(map[string]any)
	return obj, ok
}

func reqStr(obj map[string]any, key string) (string, error) {
	val, ok := obj[key].(string)
	if !ok || strings.TrimSpace(val) == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return val, nil
}

// ExpandHome 只替换显式 $HOME，不把配置变成 shell。
func ExpandHome(val, home string) string {
	if val == "$HOME" {
		return home
	}
	if strings.HasPrefix(val, "$HOME/") {
		return filepath.Join(home, val[len("$HOME/"):])
	}
	return val
}

var (
	reHome   = regexp.MustCompile(`^home:\s*"?([^"]+)"?$`)
	reRepo   = regexp.MustCompile(`^-\s+repo-name:\s*"?([^"]+)"?$`)
	reGroup  = regexp.MustCompile(`^group:\s*"?([^"]+)"?$`)
	reDesc   = regexp.MustCompile(`^description:\s*(.+)$`)
	reBranch = regexp.MustCompile(`^branch:\s*"?([^"]+)"?$`)
)

func yamlStr(val string) string {
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		return val[1 : len(val)-1]
	}
	return val
}

// ParseCatalog 只认 project.yml 里 home 和 projects 这一小段，避免为固定 schema 上完整 YAML 库。
func ParseCatalog(txt string) (string, []projEntry, error) {
	home := ""
	var projects []projEntry
	var cur *projEntry
	flush := func() {
		if cur != nil && cur.Name != "" && cur.Group != "" {
			projects = append(projects, *cur)
		}
	}
	for _, raw := range strings.Split(txt, "\n") {
		raw = strings.TrimRight(raw, "\r")
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := reHome.FindStringSubmatch(line); m != nil {
			home = yamlStr(m[1])
			continue
		}
		if m := reRepo.FindStringSubmatch(line); m != nil {
			flush()
			cur = &projEntry{Name: yamlStr(m[1])}
			continue
		}
		if cur == nil {
			continue
		}
		if m := reGroup.FindStringSubmatch(line); m != nil {
			cur.Group = yamlStr(m[1])
		}
		if m := reDesc.FindStringSubmatch(line); m != nil {
			cur.Desc = yamlStr(m[1])
		}
		if m := reBranch.FindStringSubmatch(line); m != nil {
			cur.Br = yamlStr(m[1])
		}
	}
	flush()
	if home == "" {
		return "", nil, errors.New("project.yml home must be a non-empty string")
	}
	return home, projects, nil
}

func repoPath(home string, ent projEntry) string {
	if ent.Group == "." {
		return filepath.Join(home, ent.Name)
	}
	return filepath.Join(home, ent.Group, ent.Name)
}

func repoOf(name, home string, ent projEntry, defBr string) Repo {
	br := ent.Br
	if br == "" {
		br = defBr
	}
	repo := Repo{
		Name:   name,
		Path:   repoPath(home, ent),
		Group:  ent.Group,
		Branch: br,
	}
	if ent.Desc != "" {
		repo.Desc = ent.Desc
	}
	return repo
}

// ResolveRepo 把名字收成唯一物理仓库。重名会让 worktree 落到不确定的目录。
func ResolveRepo(name, home string, projects []projEntry, defBr string) (Repo, error) {
	var matched []projEntry
	for _, ent := range projects {
		if ent.Name == name {
			matched = append(matched, ent)
		}
	}
	if len(matched) == 0 {
		return Repo{}, fmt.Errorf("repo not found in project.yml: %s", name)
	}
	if len(matched) > 1 {
		groups := make([]string, 0, len(matched))
		for _, ent := range matched {
			groups = append(groups, ent.Group)
		}
		msg := fmt.Sprintf("repo name is ambiguous in project.yml: %s (%s)", name, strings.Join(groups, ", "))
		return Repo{}, errors.New(msg)
	}
	return repoOf(name, home, matched[0], defBr), nil
}

// ResolveConfig 把轻量引用展开成执行配置。路径只从 project.yml 和 HOME 派生。
func ResolveConfig(cfg RawCfg, projectTxt string, opts ResolveOpts) (Cfg, error) {
	homeRaw, projects, err := ParseCatalog(projectTxt)
	if err != nil {
		return Cfg{}, err
	}
	projHome := ExpandHome(homeRaw, opts.Home)
	outProjects := make([]Repo, 0, len(projects))
	for _, ent := range projects {
		outProjects = append(outProjects, repoOf(ent.Name, projHome, ent, cfg.Base))
	}
	profs := map[string]Profile{}
	for _, key := range cfg.Order {
		raw := cfg.Profs[key]
		repos := make([]Repo, 0, len(raw.Repos))
		for _, name := range raw.Repos {
			repo, err := ResolveRepo(name, projHome, projects, cfg.Base)
			if err != nil {
				return Cfg{}, err
			}
			repos = append(repos, repo)
		}
		var agentsPath string
		if raw.Agents != "" {
			agentsPath = filepath.Join(opts.Tpl, raw.Agents)
		}
		profs[key] = Profile{
			Name:   raw.Name,
			Agents: agentsPath,
			Repos:  repos,
		}
	}
	var initTpl string
	if cfg.InitTpl != "" {
		initTpl = filepath.Join(opts.Tpl, cfg.InitTpl)
	}
	return Cfg{
		WsRoot:   ExpandHome(cfg.WsRoot, opts.Home),
		Base:     cfg.Base,
		Remote:   cfg.Remote,
		InitTpl:  initTpl,
		Projects: outProjects,
		Profiles: profs,
		Order:    append([]string{}, cfg.Order...),
	}, nil
}

func reqHome() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("HOME must be set")
	}
	return home, nil
}

// LoadConfig 读取配置和旁边的 project.yml。模板目录钉在配置文件所在目录，不写模块标记文件。
func LoadConfig(path string) (Cfg, error) {
	buf, err := cfgfile.Read(path, configExample)
	if err != nil {
		return Cfg{}, err
	}
	raw, err := ParseConfigBytes(buf)
	if err != nil {
		return Cfg{}, err
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Cfg{}, fmt.Errorf("read config: %w", err)
	}
	confDir := filepath.Dir(real)
	projFile := filepath.Join(confDir, "project.yml")
	proj, err := os.ReadFile(projFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Cfg{}, fmt.Errorf("project config not found: %s", projFile)
		}
		return Cfg{}, fmt.Errorf("read project config: %w", err)
	}
	home, err := reqHome()
	if err != nil {
		return Cfg{}, err
	}
	opts := ResolveOpts{
		Home: home,
		Tpl:  filepath.Join(confDir, "templates", "agents"),
	}
	return ResolveConfig(raw, string(proj), opts)
}

// LoadRaw 只解析 config.json 的必填字段。doctor 不读 project.yml，也不访问仓库。
func LoadRaw(path string) (RawCfg, error) {
	buf, err := cfgfile.Read(path, configExample)
	if err != nil {
		return RawCfg{}, err
	}
	return ParseConfigBytes(buf)
}
