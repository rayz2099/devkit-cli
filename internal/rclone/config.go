package rclone

import (
	"fmt"
	"math"
	"os"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
)

const exampleRel = "rclone-cli/config.example.json"

var topKeys = []string{"rcloneBin", "sources", "targets", "tasks"}
var sourceKeys = []string{"name", "path"}
var targetKeys = []string{
	"name", "url", "vendor", "user", "pass", "root",
	"skipLinks", "skipHidden", "sizeOnly", "transfers", "checkers",
	"retries", "retriesSleep", "lowLevelRetries", "timeout", "contimeout",
	"tpslimit", "pacerMinSleep", "encryptFilenames", "crypt",
}
var taskKeys = []string{
	"name", "source", "target", "excludes", "transfers", "tpslimit",
	"timeout", "encryptFilenames", "skipHidden",
}
var cryptKeys = []string{"name", "key", "salt", "algorithm", "default"}

// Load 读取并校验配置。缺文件必须失败，空配置会让后面的命令看起来成功。
func Load(path, home string) (*FileCfg, error) {
	buf, err := cfgfile.Read(path, exampleRel)
	if err != nil {
		return nil, fail(1, "%s", err.Error())
	}
	cfg, err := parseFileCfg(string(buf), path, home)
	if err != nil {
		return nil, err
	}
	if err := assertSecretMode(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func parseFileCfg(content, path, home string) (*FileCfg, error) {
	// cfgfile 判断尾逗号时不跳过注释。示例配置是逗号后面直接跟 //，先去掉注释再解析。
	stripped := dropComments(content)
	var parsed any
	if err := cfgfile.ParseJSONC([]byte(stripped), &parsed); err != nil {
		return nil, fail(1, "invalid config file %s", path)
	}
	raw, err := needObj(parsed, path)
	if err != nil {
		return nil, err
	}
	if err := rejectUnknown(raw, topKeys, path); err != nil {
		return nil, err
	}
	sources, err := readSources(raw["sources"], path, home)
	if err != nil {
		return nil, err
	}
	targets, err := readTargets(raw["targets"], path)
	if err != nil {
		return nil, err
	}
	tasks, err := readTasks(raw["tasks"], path, sources, targets)
	if err != nil {
		return nil, err
	}
	bin, err := readBin(raw, path)
	if err != nil {
		return nil, err
	}
	return &FileCfg{RcloneBin: bin, Sources: sources, Targets: targets, Tasks: tasks}, nil
}

func taskNames(cfg *FileCfg) []string {
	names := make([]string, 0, len(cfg.Tasks))
	for _, task := range cfg.Tasks {
		names = append(names, task.Name)
	}
	return names
}

// assertSecretMode 只读现有权限。口令在文件里时必须是 600，这里不能 chmod，避免改掉用户的配置。
func assertSecretMode(path string, cfg *FileCfg) error {
	secret := false
	for _, target := range cfg.Targets {
		if hasSecret(target) {
			secret = true
			break
		}
	}
	if !secret {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fail(1, "read config: %s", err.Error())
	}
	mode := info.Mode().Perm()
	if mode != 0o600 {
		return fail(1, "config file mode must be 600: %s", path)
	}
	return nil
}

func hasSecret(target Target) bool {
	if target.User != "" || target.Pass != "" {
		return true
	}
	return target.HasCrypt
}

func readBin(raw map[string]any, path string) (string, error) {
	value, ok := raw["rcloneBin"]
	if !ok {
		return "rclone", nil
	}
	text, isStr := value.(string)
	if !isStr || strings.TrimSpace(text) == "" {
		return "", fail(1, "%s rcloneBin must be a non-empty string", path)
	}
	return text, nil
}

func readSources(value any, path, home string) ([]Source, error) {
	items, err := needArr(value, path+" sources")
	if err != nil {
		return nil, err
	}
	sources := make([]Source, 0, len(items))
	for i, item := range items {
		src, err := readSource(item, fmt.Sprintf("%s sources[%d]", path, i), home)
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}
	names := make([]string, len(sources))
	for i, src := range sources {
		names[i] = src.Name
	}
	if err := assertUnique(names, path+" sources"); err != nil {
		return nil, err
	}
	return sources, nil
}

func readSource(value any, at, home string) (Source, error) {
	raw, err := needObj(value, at)
	if err != nil {
		return Source{}, err
	}
	if err := rejectUnknown(raw, sourceKeys, at); err != nil {
		return Source{}, err
	}
	name, err := needStr(raw, "name", at)
	if err != nil {
		return Source{}, err
	}
	abs, err := needAbs(raw, at, home)
	if err != nil {
		return Source{}, err
	}
	return Source{Name: name, Path: abs}, nil
}

func needAbs(raw map[string]any, at, home string) (string, error) {
	text, err := needStr(raw, "path", at)
	if err != nil {
		return "", err
	}
	path, err := expandHome(text, home)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(path, "/") {
		return "", fail(1, "%s.path must be absolute", at)
	}
	return path, nil
}

func expandHome(raw, home string) (string, error) {
	base, err := needHome(home)
	if err != nil {
		return "", err
	}
	switch {
	case raw == "$HOME" || raw == "${HOME}":
		return base, nil
	case strings.HasPrefix(raw, "$HOME/"):
		return base + raw[len("$HOME"):], nil
	case strings.HasPrefix(raw, "${HOME}/"):
		return base + raw[len("${HOME}"):], nil
	default:
		return raw, nil
	}
}

func needHome(home string) (string, error) {
	if home == "" {
		return "", fail(1, "HOME is required")
	}
	return home, nil
}

func readTargets(value any, path string) ([]Target, error) {
	items, err := needArr(value, path+" targets")
	if err != nil {
		return nil, err
	}
	targets := make([]Target, 0, len(items))
	for i, item := range items {
		tgt, err := readTarget(item, fmt.Sprintf("%s targets[%d]", path, i))
		if err != nil {
			return nil, err
		}
		targets = append(targets, tgt)
	}
	names := make([]string, len(targets))
	for i, tgt := range targets {
		names[i] = tgt.Name
	}
	if err := assertUnique(names, path+" targets"); err != nil {
		return nil, err
	}
	return targets, nil
}

func readTarget(value any, at string) (Target, error) {
	raw, err := needObj(value, at)
	if err != nil {
		return Target{}, err
	}
	if err := rejectUnknown(raw, targetKeys, at); err != nil {
		return Target{}, err
	}
	crypt, hasCrypt, err := readCryptField(raw, at+".crypt")
	if err != nil {
		return Target{}, err
	}
	name, err := needStr(raw, "name", at)
	if err != nil {
		return Target{}, err
	}
	url, err := needStr(raw, "url", at)
	if err != nil {
		return Target{}, err
	}
	vendor, err := optStr(raw, "vendor", at, "other")
	if err != nil {
		return Target{}, err
	}
	user, err := needText(raw, "user", at)
	if err != nil {
		return Target{}, err
	}
	pass, err := needText(raw, "pass", at)
	if err != nil {
		return Target{}, err
	}
	root, err := needStr(raw, "root", at)
	if err != nil {
		return Target{}, err
	}
	skipLinks, err := optBool(raw, "skipLinks", at, true)
	if err != nil {
		return Target{}, err
	}
	skipHidden, err := optBool(raw, "skipHidden", at, true)
	if err != nil {
		return Target{}, err
	}
	sizeOnly, err := optBool(raw, "sizeOnly", at, true)
	if err != nil {
		return Target{}, err
	}
	transfers, err := optNum(raw, "transfers", at, 1)
	if err != nil {
		return Target{}, err
	}
	checkers, err := optNum(raw, "checkers", at, 4)
	if err != nil {
		return Target{}, err
	}
	retries, err := optNum(raw, "retries", at, 10)
	if err != nil {
		return Target{}, err
	}
	retriesSleep, err := optStr(raw, "retriesSleep", at, "15s")
	if err != nil {
		return Target{}, err
	}
	low, err := optNum(raw, "lowLevelRetries", at, 20)
	if err != nil {
		return Target{}, err
	}
	timeout, err := optStr(raw, "timeout", at, "1h")
	if err != nil {
		return Target{}, err
	}
	contimeout, err := optStr(raw, "contimeout", at, "60s")
	if err != nil {
		return Target{}, err
	}
	tps, err := optNum(raw, "tpslimit", at, 2)
	if err != nil {
		return Target{}, err
	}
	pacer, err := optStr(raw, "pacerMinSleep", at, "200ms")
	if err != nil {
		return Target{}, err
	}
	encNames, err := optBool(raw, "encryptFilenames", at, false)
	if err != nil {
		return Target{}, err
	}
	if encNames && !hasCrypt {
		return Target{}, fail(1, "%s.encryptFilenames requires crypt", at)
	}
	return Target{
		Name: name, URL: url, Vendor: vendor, User: user, Pass: pass, Root: root,
		SkipLinks: skipLinks, SkipHidden: skipHidden, SizeOnly: sizeOnly,
		Transfers: transfers, Checkers: checkers, Retries: retries, RetriesSleep: retriesSleep,
		LowLevelRetries: low, Timeout: timeout, Contimeout: contimeout, Tpslimit: tps,
		PacerMinSleep: pacer, EncryptFilenames: encNames, Crypt: crypt, HasCrypt: hasCrypt,
	}, nil
}

func readCryptField(raw map[string]any, at string) ([]Crypt, bool, error) {
	value, present := raw["crypt"]
	if !present {
		return nil, false, nil
	}
	return readCrypt(value, at)
}

func readCrypt(value any, at string) ([]Crypt, bool, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, false, fail(1, "%s must be an array of key pairs", at)
	}
	if len(items) == 0 {
		return nil, false, fail(1, "%s must have at least one key pair", at)
	}
	pairs := make([]Crypt, 0, len(items))
	for i, item := range items {
		pair, err := readCryptPair(item, fmt.Sprintf("%s[%d]", at, i))
		if err != nil {
			return nil, false, err
		}
		pairs = append(pairs, pair)
	}
	names := make([]string, len(pairs))
	for i, pair := range pairs {
		names[i] = pair.Name
	}
	if err := assertUnique(names, at); err != nil {
		return nil, false, err
	}
	marked, err := markDefault(pairs, at)
	if err != nil {
		return nil, false, err
	}
	return marked, true, nil
}

func markDefault(pairs []Crypt, at string) ([]Crypt, error) {
	n := 0
	for _, item := range pairs {
		if item.Default {
			n++
		}
	}
	if n > 1 {
		return nil, fail(1, "%s requires exactly one default key pair", at)
	}
	if n == 1 {
		return pairs, nil
	}
	pairs[0].Default = true
	return pairs, nil
}

func readCryptPair(value any, at string) (Crypt, error) {
	raw, err := needObj(value, at)
	if err != nil {
		return Crypt{}, err
	}
	if _, ok := raw["password"]; ok {
		return Crypt{}, fail(1, "%s use key and salt, not password/password2", at)
	}
	if _, ok := raw["password2"]; ok {
		return Crypt{}, fail(1, "%s use key and salt, not password/password2", at)
	}
	if err := rejectUnknown(raw, cryptKeys, at); err != nil {
		return Crypt{}, err
	}
	algo, err := optStr(raw, "algorithm", at, "secretbox")
	if err != nil {
		return Crypt{}, err
	}
	if algo != "secretbox" {
		return Crypt{}, fail(1, "%s.algorithm must be secretbox", at)
	}
	name, err := needStr(raw, "name", at)
	if err != nil {
		return Crypt{}, err
	}
	key, err := needStr(raw, "key", at)
	if err != nil {
		return Crypt{}, err
	}
	salt, err := needStr(raw, "salt", at)
	if err != nil {
		return Crypt{}, err
	}
	def, err := optBool(raw, "default", at, false)
	if err != nil {
		return Crypt{}, err
	}
	return Crypt{Name: name, Key: key, Salt: salt, Algorithm: "secretbox", Default: def}, nil
}

func readTasks(value any, path string, sources []Source, targets []Target) ([]Task, error) {
	items, err := needArr(value, path+" tasks")
	if err != nil {
		return nil, err
	}
	tasks := make([]Task, 0, len(items))
	for i, item := range items {
		task, err := readTask(item, fmt.Sprintf("%s tasks[%d]", path, i), sources, targets)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	names := make([]string, len(tasks))
	for i, task := range tasks {
		names[i] = task.Name
	}
	if err := assertUnique(names, path+" tasks"); err != nil {
		return nil, err
	}
	return tasks, nil
}

func readTask(value any, at string, sources []Source, targets []Target) (Task, error) {
	raw, err := needObj(value, at)
	if err != nil {
		return Task{}, err
	}
	if err := rejectUnknown(raw, taskKeys, at); err != nil {
		return Task{}, err
	}
	source, err := needStr(raw, "source", at)
	if err != nil {
		return Task{}, err
	}
	targetName, err := needStr(raw, "target", at)
	if err != nil {
		return Task{}, err
	}
	if !sourceNamed(sources, source) {
		return Task{}, fail(1, "%s.source not found: %s", at, source)
	}
	tgt, ok := targetNamed(targets, targetName)
	if !ok {
		return Task{}, fail(1, "%s.target not found: %s", at, targetName)
	}
	name, err := needStr(raw, "name", at)
	if err != nil {
		return Task{}, err
	}
	excludes, err := readExcludesField(raw, at+".excludes")
	if err != nil {
		return Task{}, err
	}
	task := Task{Name: name, Source: source, Target: targetName, Excludes: excludes}
	if _, ok := raw["transfers"]; ok {
		n, err := needNum(raw, "transfers", at)
		if err != nil {
			return Task{}, err
		}
		task.Transfers = &n
	}
	if _, ok := raw["tpslimit"]; ok {
		n, err := needNum(raw, "tpslimit", at)
		if err != nil {
			return Task{}, err
		}
		task.Tpslimit = &n
	}
	if _, ok := raw["timeout"]; ok {
		text, err := needStr(raw, "timeout", at)
		if err != nil {
			return Task{}, err
		}
		task.Timeout = &text
	}
	if _, ok := raw["encryptFilenames"]; ok {
		flag, err := needBool(raw, "encryptFilenames", at)
		if err != nil {
			return Task{}, err
		}
		task.EncryptFilenames = &flag
	}
	if _, ok := raw["skipHidden"]; ok {
		flag, err := needBool(raw, "skipHidden", at)
		if err != nil {
			return Task{}, err
		}
		task.SkipHidden = &flag
	}
	encNames := tgt.EncryptFilenames
	if task.EncryptFilenames != nil {
		encNames = *task.EncryptFilenames
	}
	if encNames && !tgt.HasCrypt {
		return Task{}, fail(1, "%s.encryptFilenames requires crypt on the target", at)
	}
	return task, nil
}

func readExcludesField(raw map[string]any, at string) ([]string, error) {
	value, present := raw["excludes"]
	if !present {
		return []string{}, nil
	}
	return readExcludes(value, at)
}

func readExcludes(value any, at string) ([]string, error) {
	items, err := needArr(value, at)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fail(1, "%s[%d] must be a non-empty string", at, i)
		}
		out = append(out, text)
	}
	return out, nil
}

func sourceNamed(sources []Source, name string) bool {
	for _, item := range sources {
		if item.Name == name {
			return true
		}
	}
	return false
}

func targetNamed(targets []Target, name string) (Target, bool) {
	for _, item := range targets {
		if item.Name == name {
			return item, true
		}
	}
	return Target{}, false
}

func assertUnique(names []string, at string) error {
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			return fail(1, "%s name duplicates %s", at, name)
		}
		seen[name] = true
	}
	return nil
}

func rejectUnknown(raw map[string]any, allow []string, at string) error {
	for key := range raw {
		if !containsStr(allow, key) {
			return fail(1, "%s %s is not allowed", at, key)
		}
	}
	return nil
}

func containsStr(items []string, key string) bool {
	for _, item := range items {
		if item == key {
			return true
		}
	}
	return false
}

func needObj(value any, at string) (map[string]any, error) {
	raw, ok := value.(map[string]any)
	if !ok || value == nil {
		return nil, fail(1, "%s must be an object", at)
	}
	return raw, nil
}

func needArr(value any, at string) ([]any, error) {
	if _, ok := value.([]any); !ok {
		return nil, fail(1, "%s must be an array", at)
	}
	return value.([]any), nil
}

func needStr(raw map[string]any, key, at string) (string, error) {
	text, err := needText(raw, key, at)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", fail(1, "%s.%s must be a non-empty string", at, key)
	}
	return text, nil
}

func needText(raw map[string]any, key, at string) (string, error) {
	value, ok := raw[key]
	if !ok {
		return "", fail(1, "%s.%s is required", at, key)
	}
	text, isStr := value.(string)
	if !isStr {
		return "", fail(1, "%s.%s must be a string", at, key)
	}
	return text, nil
}

func needBool(raw map[string]any, key, at string) (bool, error) {
	if _, ok := raw[key]; !ok {
		return false, fail(1, "%s.%s is required", at, key)
	}
	return asBool(raw[key], at+"."+key)
}

func optBool(raw map[string]any, key, at string, whenOmit bool) (bool, error) {
	value, ok := raw[key]
	if !ok {
		return whenOmit, nil
	}
	return asBool(value, at+"."+key)
}

func asBool(value any, at string) (bool, error) {
	flag, ok := value.(bool)
	if !ok {
		return false, fail(1, "%s must be a boolean", at)
	}
	return flag, nil
}

func needNum(raw map[string]any, key, at string) (float64, error) {
	if _, ok := raw[key]; !ok {
		return 0, fail(1, "%s.%s is required", at, key)
	}
	return asNum(raw[key], at+"."+key)
}

func optNum(raw map[string]any, key, at string, whenOmit float64) (float64, error) {
	value, ok := raw[key]
	if !ok {
		return whenOmit, nil
	}
	return asNum(value, at+"."+key)
}

func asNum(value any, at string) (float64, error) {
	num, ok := value.(float64)
	if !ok || math.IsNaN(num) || math.IsInf(num, 0) {
		return 0, fail(1, "%s must be a number", at)
	}
	return num, nil
}

func optStr(raw map[string]any, key, at, whenOmit string) (string, error) {
	value, ok := raw[key]
	if !ok {
		return whenOmit, nil
	}
	text, isStr := value.(string)
	if !isStr || strings.TrimSpace(text) == "" {
		return "", fail(1, "%s.%s must be a non-empty string", at, key)
	}
	return text, nil
}

func dropComments(text string) string {
	var b strings.Builder
	inStr := false
	escape := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if inStr {
			b.WriteByte(ch)
			if escape {
				escape = false
			} else if ch == '\\' {
				escape = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		if ch == '"' {
			inStr = true
			b.WriteByte(ch)
			continue
		}
		if ch == '/' && i+1 < len(text) && text[i+1] == '/' {
			i++
			for i+1 < len(text) && text[i+1] != '\n' {
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(text) && text[i+1] == '*' {
			i += 2
			for i+1 < len(text) && !(text[i] == '*' && text[i+1] == '/') {
				i++
			}
			if i+1 < len(text) {
				i++
			}
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}
