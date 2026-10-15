package nacos

import "strings"

var valueFlags = map[string]struct{}{
	"-c":             {},
	"--config":       {},
	"--server-addr":  {},
	"--username":     {},
	"--password":     {},
	"-ns":            {},
	"--namespace":    {},
	"-o":             {},
	"--output":       {},
	"--data-id":      {},
	"--group":        {},
	"--content":      {},
	"--search":       {},
	"--page-no":      {},
	"--page-size":    {},
	"--service":      {},
	"--ip":           {},
	"--port":         {},
	"--cluster":      {},
	"--clusters":     {},
	"--weight":       {},
	"--ephemeral":    {},
	"--healthy-only": {},
	"--prefix":       {},
	"--to-complete":  {},
}

var boolFlags = map[string]struct{}{
	"--dev":  {},
	"-h":     {},
	"--help": {},
}

func posArgs(tokens []string) []string {
	var out []string
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if _, ok := valueFlags[token]; ok {
			i++
			continue
		}
		if _, ok := boolFlags[token]; ok || strings.HasPrefix(token, "-") {
			continue
		}
		out = append(out, token)
	}
	return out
}

func lastValueFlag(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	last := tokens[len(tokens)-1]
	if _, ok := valueFlags[last]; ok {
		return last
	}
	return ""
}

func flagValue(tokens []string, name string) string {
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == name {
			if i+1 < len(tokens) {
				return tokens[i+1]
			}
			return ""
		}
	}
	return ""
}

func filterPrefix(values []string, prefix string) []string {
	out := make([]string, 0)
	for _, item := range values {
		if strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return out
}

// completeLine 用当前命令行里的旗标决定补什么。
// 还没选定 namespace 时先给 -ns，避免把别的空间的 dataId 补进来。
func completeLine(tokens []string, current string, file fileConfig) ([]string, error) {
	last := lastValueFlag(tokens)
	switch last {
	case "-c", "--config":
		return []string{}, nil
	case "-ns", "--namespace":
		return filterPrefix(nsCands(file), current), nil
	case "-o", "--output":
		return filterPrefix([]string{"text", "json"}, current), nil
	case "--search":
		return filterPrefix([]string{"blur", "accurate"}, current), nil
	case "--ephemeral", "--healthy-only":
		return filterPrefix([]string{"true", "false"}, current), nil
	}

	positionals := posArgs(tokens)
	needData := last == "--data-id" || (last == "" && at(positionals, 0) == "config" && at(positionals, 1) == "get" && len(positionals) == 2)
	needGroups := last == "--group" || (last == "" && at(positionals, 0) == "config" && at(positionals, 1) == "get" && len(positionals) == 3)
	if needData || needGroups {
		g := looseGlobals(tokens)
		if needData && g.namespace == nil {
			return namespaceArg(current), nil
		}
		rt, err := resolveRuntime(g, file)
		if err != nil {
			return nil, err
		}
		client, err := newClient(rt)
		if err != nil {
			return nil, err
		}
		items, err := client.listItemsCached()
		if err != nil {
			return nil, err
		}
		if needData {
			return uniqueDataIDs(items, current), nil
		}
		return groupsByDataID(items, resolveDataID(tokens, positionals, last), current), nil
	}
	if last != "" {
		return []string{}, nil
	}
	if len(positionals) == 0 {
		return filterPrefix([]string{"config", "naming", "completion"}, current), nil
	}
	if positionals[0] == "config" && len(positionals) == 1 {
		return filterPrefix([]string{"get", "put", "delete", "list"}, current), nil
	}
	if positionals[0] == "naming" && len(positionals) == 1 {
		return filterPrefix([]string{"register", "deregister", "instances"}, current), nil
	}
	if positionals[0] == "completion" && len(positionals) == 1 {
		return filterPrefix([]string{"fish"}, current), nil
	}
	return []string{}, nil
}

func namespaceArg(current string) []string {
	if strings.HasPrefix("-ns", current) {
		return []string{"-ns"}
	}
	return []string{}
}

func looseGlobals(tokens []string) globals {
	var g globals
	for _, token := range tokens {
		if token == "--dev" {
			g.dev = true
			break
		}
	}
	read := func(name string) *string {
		val := flagValue(tokens, name)
		if val == "" {
			return nil
		}
		return &val
	}
	g.server = read("--server-addr")
	g.user = read("--username")
	g.pass = read("--password")
	g.namespace = read("-ns")
	if g.namespace == nil {
		g.namespace = read("--namespace")
	}
	out := read("--output")
	if out == nil {
		out = read("-o")
	}
	if out != nil && (*out == "text" || *out == "json") {
		g.output = out
	}
	return g
}

func resolveDataID(tokens, positionals []string, last string) string {
	if last == "--group" {
		flagged := flagValue(tokens, "--data-id")
		if flagged != "" {
			return flagged
		}
	}
	return at(positionals, 2)
}

func splitToComplete(args []string) (tokens []string, current string) {
	currentSet := false
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--to-complete" {
			if i+1 < len(args) {
				current = args[i+1]
			}
			currentSet = true
			i++
			continue
		}
		tokens = append(tokens, token)
	}
	if !currentSet {
		if len(tokens) > 0 {
			current = tokens[len(tokens)-1]
			tokens = tokens[:len(tokens)-1]
		}
	}
	if tokens == nil {
		tokens = []string{}
	}
	return tokens, current
}

// cobraComp 末尾的 :4 关掉文件名补全。缺了这一行，shell 会把 COMMON 拿去当路径。
func cobraComp(values []string) string {
	if len(values) == 0 {
		return ":4\n"
	}
	return strings.Join(values, "\n") + "\n:4\n"
}

func completeLines(values []string, prefix string) string {
	matched := filterPrefix(values, prefix)
	if len(matched) == 0 {
		return ""
	}
	return strings.Join(matched, "\n") + "\n"
}
