package td

import "strings"

var rootWords = []string{"auth", "project", "meta", "report", "query", "completion", "agent", "human", "help"}
var authWords = []string{"login", "status"}
var projectWords = []string{"ls"}
var metaWords = []string{"events", "props"}
var reportWords = []string{"ls", "get"}
var queryWords = []string{"event", "property", "run", "report"}

// completeLines 只补已经能确定的命令词. 不去猜项目号和事件名.
func completeLines(tokens []string, current string) string {
	values := completeValues(tokens, current)
	var matched []string
	for _, item := range values {
		if strings.HasPrefix(item, current) {
			matched = append(matched, item)
		}
	}
	if len(matched) == 0 {
		return ""
	}
	return strings.Join(matched, "\n") + "\n"
}

func completeValues(tokens []string, current string) []string {
	var last string
	if len(tokens) > 0 {
		last = tokens[len(tokens)-1]
	}
	if last == "-c" || last == "--config" || last == "--qp" {
		return nil
	}
	flags, pos, err := takeCmd(tokens)
	if err != nil {
		return nil
	}
	if last == "-p" || last == "--profile" {
		return profileCandidates(flags)
	}
	if len(pos) == 0 || (len(pos) == 1 && pos[0] == current) {
		return rootWords
	}
	head := pos[0]
	rest := pos
	if head == "agent" || head == "human" {
		rest = pos[1:]
	}
	if len(rest) == 0 || (len(rest) == 1 && rest[0] == current) {
		return withoutAudience(rootWords)
	}
	return subWords(rest, current)
}

func withoutAudience(words []string) []string {
	var out []string
	for _, item := range words {
		if item == "agent" || item == "human" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func subWords(pos []string, current string) []string {
	if len(pos) == 0 {
		return nil
	}
	head := pos[0]
	var child string
	if len(pos) > 1 {
		child = pos[1]
	}
	if len(pos) < 2 || child == current {
		switch head {
		case "auth":
			return authWords
		case "project":
			return projectWords
		case "meta":
			return metaWords
		case "report":
			return reportWords
		case "query":
			return queryWords
		}
	}
	return nil
}

func profileCandidates(flags map[string]string) []string {
	path := flags["-c"]
	if path == "" {
		path = flags["--config"]
	}
	if path == "" {
		var err error
		path, err = cfgPath()
		if err != nil {
			return nil
		}
	}
	file, err := loadFileCfg(path)
	if err != nil {
		return nil
	}
	return profileNames(file)
}
