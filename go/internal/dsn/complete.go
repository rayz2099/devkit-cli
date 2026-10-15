package dsn

import "strings"

// completeLines 是 fish 调用的隐藏协议：一行一个候选，没有候选就空输出。
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
	if len(tokens) > 0 {
		last := tokens[len(tokens)-1]
		if last == "-c" || last == "--config" {
			return []string{}
		}
		if last == "-p" || last == "--profile" {
			return loadProfiles(safeFlags(tokens))
		}
		if last == "--output" {
			return []string{"json", "csv", "plain"}
		}
	}
	_, pos := safeTake(tokens)
	if len(pos) == 0 || (len(pos) == 1 && pos[0] == current) {
		return []string{"query", "doctor", "ds", "completion", "agent", "human", "version"}
	}
	if pos[0] == "agent" || pos[0] == "human" {
		rest := pos[1:]
		if len(rest) == 0 || (len(rest) == 1 && rest[0] == current) {
			return []string{"query", "doctor", "ds"}
		}
		if rest[0] == "completion" {
			return []string{"fish"}
		}
	}
	if pos[0] == "completion" {
		if len(pos) <= 2 && (len(pos) == 1 || pos[1] == current) {
			return []string{"fish"}
		}
		return []string{}
	}
	return kindStmtComplete(tokens, pos)
}

func safeTake(tokens []string) (map[string]string, []string) {
	flags, pos, err := takeCmd(tokens)
	if err != nil {
		var kept []string
		for _, item := range tokens {
			if !strings.HasPrefix(item, "-") {
				kept = append(kept, item)
			}
		}
		return map[string]string{}, kept
	}
	return flags, pos
}

func safeFlags(tokens []string) map[string]string {
	flags, _ := safeTake(tokens)
	return flags
}

func loadProfiles(flags map[string]string) []string {
	path, err := ConfigPath(firstNonEmpty(flags["-c"], flags["--config"]))
	if err != nil {
		return []string{}
	}
	file, err := Load(path)
	if err != nil {
		return []string{}
	}
	var lines []string
	for _, item := range file.Profiles {
		if item.Description == "" {
			lines = append(lines, item.Name)
			continue
		}
		lines = append(lines, item.Name+"\t"+item.Description)
	}
	return lines
}

func kindStmtComplete(tokens, pos []string) []string {
	rest := queryRest(pos)
	if rest == nil {
		return []string{}
	}
	flags, _ := safeTake(tokens)
	name := flags["-p"]
	if name == "" {
		name = flags["--profile"]
	}
	if name == "" {
		return []string{}
	}
	path, err := ConfigPath(firstNonEmpty(flags["-c"], flags["--config"]))
	if err != nil {
		return []string{}
	}
	file, err := Load(path)
	if err != nil {
		return []string{}
	}
	prof, err := Pick(file, name)
	if err != nil {
		return []string{}
	}
	if prof.Kind == "kafka" {
		topics, err := readTopicCache(prof.Name)
		if err != nil {
			return []string{}
		}
		return kafkaComplete(rest, topics)
	}
	if prof.Kind == "postgres" {
		return pgCatalogComplete(rest)
	}
	return []string{}
}

func queryRest(pos []string) []string {
	rest := pos
	if len(rest) > 0 && (rest[0] == "agent" || rest[0] == "human") {
		rest = rest[1:]
	}
	if len(rest) == 0 || rest[0] != "query" {
		return nil
	}
	return rest[1:]
}
