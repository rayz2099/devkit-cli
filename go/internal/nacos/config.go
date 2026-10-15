package nacos

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/xdg"
)

const (
	defaultServer = "127.0.0.1:8848"
	defaultNS     = "public"
	defaultOutput = "text"
	exampleRel    = "nacos-cli/config.example.json"
	appName       = "nacos-cli"
)

// fileConfig 是磁盘上的 JSON。字段名沿用旧配置，本机已有的 config.json 不用改名。
type fileConfig struct {
	server     string
	user       string
	pass       string
	namespace  string
	namespaces []string
	output     string
}

// runtime 是旗标、环境变量和文件叠完后的最终连接参数。
type runtime struct {
	server    string
	user      string
	pass      string
	namespace string
	output    string
	dev       bool
}

func configPath(explicit string) (string, bool, error) {
	if explicit != "" {
		return explicit, true, nil
	}
	root, err := xdg.ConfigRoot()
	if err != nil {
		return "", false, errors.New("HOME is required")
	}
	return filepath.Join(root, appName, "config.json"), false, nil
}

// loadCfg 读取配置。缺默认文件是常态，不能当成启动失败；显式 -c 缺失才报错。
func loadCfg(explicit string) (fileConfig, error) {
	path, explicitSet, err := configPath(explicit)
	if err != nil {
		return fileConfig{}, err
	}
	buf, err := cfgfile.Read(path, exampleRel)
	if err != nil {
		if errors.Is(err, cfgfile.ErrMissing) {
			if explicitSet {
				return fileConfig{}, fmt.Errorf("config file not found: %s", path)
			}
			return fileConfig{namespaces: []string{}}, nil
		}
		return fileConfig{}, err
	}
	return parseFile(buf, path)
}

func parseFile(buf []byte, path string) (fileConfig, error) {
	var parsed any
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return fileConfig{}, fmt.Errorf("invalid config file %s", path)
	}
	obj, ok := parsed.(map[string]any)
	if !ok {
		return fileConfig{}, fmt.Errorf("invalid config file %s", path)
	}
	ns, err := readNamespaces(obj, path)
	if err != nil {
		return fileConfig{}, err
	}
	return fileConfig{
		server:     readStr(obj["nacos_server_addr"]),
		user:       readStr(obj["nacos_username"]),
		pass:       readStr(obj["nacos_password"]),
		namespace:  readStr(obj["nacos_namespace"]),
		namespaces: ns,
		output:     readStr(obj["nacos_output"]),
	}, nil
}

func readNamespaces(obj map[string]any, path string) ([]string, error) {
	value, present := obj["namespaces"]
	if !present {
		return []string{}, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid config file %s: namespaces must be an array", path)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if ok {
			out = append(out, text)
		}
	}
	return out, nil
}

func readStr(value any) string {
	text, _ := value.(string)
	return text
}

// resolveRuntime 按旗标、环境变量、文件、默认值的顺序取值。
// 空字符串的旗标和环境变量也算写过，不能被文件里的旧值顶回来。
func resolveRuntime(g globals, file fileConfig) (runtime, error) {
	server := resolveStr(g.server, []string{"nacos_server_addr", "NACOS_SERVER_ADDR"}, file.server, defaultServer)
	user := resolveStr(g.user, []string{"nacos_username", "NACOS_USERNAME"}, file.user, "")
	pass := resolveStr(g.pass, []string{"nacos_password", "NACOS_PASSWORD"}, file.pass, "")
	ns := resolveNS(g.namespace, file)
	output := defaultOutput
	if g.output != nil {
		output = *g.output
	} else {
		output = strings.ToLower(resolveStr(nil, []string{"nacos_output", "NACOS_OUTPUT"}, file.output, defaultOutput))
	}
	if output != "text" && output != "json" {
		return runtime{}, fmt.Errorf("invalid output: %s", output)
	}
	if strings.TrimSpace(server) == "" {
		return runtime{}, errors.New("server-addr is required")
	}
	return runtime{
		server:    server,
		user:      user,
		pass:      pass,
		namespace: ns,
		output:    output,
		dev:       g.dev,
	}, nil
}

func resolveNS(flag *string, file fileConfig) string {
	if flag != nil {
		return *flag
	}
	for _, key := range []string{"nacos_namespace", "NACOS_NAMESPACE"} {
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
	}
	if strings.TrimSpace(file.namespace) != "" {
		return file.namespace
	}
	for _, item := range file.namespaces {
		val := strings.TrimSpace(item)
		if val != "" {
			return val
		}
	}
	return defaultNS
}

func resolveStr(flag *string, keys []string, fileVal, def string) string {
	if flag != nil {
		return *flag
	}
	for _, key := range keys {
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
	}
	if strings.TrimSpace(fileVal) != "" {
		return fileVal
	}
	return def
}

// nsCands 只暴露配置里声明过的 namespace，最后补上 public，避免补全把线上 ID 手打错。
func nsCands(file fileConfig) []string {
	var out []string
	seen := map[string]struct{}{}
	add := func(value string) {
		item := strings.TrimSpace(value)
		if item == "" {
			return
		}
		if _, ok := seen[item]; ok {
			return
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	for _, item := range file.namespaces {
		add(item)
	}
	add(file.namespace)
	add(defaultNS)
	if len(out) == 0 {
		return []string{defaultNS}
	}
	return out
}

// validateFile 只检查文件里已经写下的值。空着的字段交给默认值，不算配置损坏。
func validateFile(file fileConfig) error {
	if strings.TrimSpace(file.output) != "" {
		raw := strings.ToLower(file.output)
		if raw != "text" && raw != "json" {
			return fmt.Errorf("invalid output: %s", raw)
		}
	}
	if strings.TrimSpace(file.server) != "" {
		if _, err := parseServerAddr(file.server); err != nil {
			return err
		}
	}
	return nil
}
