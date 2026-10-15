package codeup

import "strings"

var rootCmds = []string{"init", "repos", "push", "cr", "webhook", "completion", "doctor", "version", "agent", "human"}

var crSubs = []string{"list", "get", "create", "merge"}

var hookSubs = []string{"list"}

// CompleteLines 只返回索引里已有的值。补全不能为了提示再打 OpenAPI。
func CompleteLines(tokens []string, current string) (string, error) {
	values := completeValues(tokens, current)
	var matched []string
	for _, item := range values {
		if strings.HasPrefix(item, current) {
			matched = append(matched, item)
		}
	}
	if len(matched) == 0 {
		return "", nil
	}
	return strings.Join(matched, "\n") + "\n", nil
}

func completeValues(tokens []string, current string) []string {
	last := lastToken(tokens)
	if last == "-c" || last == "--config" {
		return nil
	}
	flags, pos := safeTake(tokens)
	profile, _ := firstFlag(flags, "-p", "--profile")
	if last == "-p" || last == "--profile" {
		return loadProfiles(flags)
	}
	if last == "--repo" {
		return loadRepoHints(flags, profile)
	}
	if last == "--type" {
		return append([]string{}, MergeTypes...)
	}
	if len(pos) == 0 {
		return rootCmds
	}
	if len(pos) == 1 && current != "" && pos[0] == current {
		return rootCmds
	}
	head := pos[0]
	if head == audienceAgent || head == audienceHuman {
		return completeAfterAudience(pos[1:], current, flags, profile)
	}
	return completeCmd(pos, current, flags, profile)
}

func completeAfterAudience(pos []string, current string, flags map[string]string, profile string) []string {
	if len(pos) == 0 || (len(pos) == 1 && pos[0] == current) {
		return withoutAudience(rootCmds)
	}
	return completeCmd(pos, current, flags, profile)
}

func completeCmd(pos []string, current string, flags map[string]string, profile string) []string {
	head := ""
	if len(pos) > 0 {
		head = pos[0]
	}
	switch head {
	case "cr":
		if len(pos) <= 2 && (len(pos) < 2 || pos[1] == current) {
			return crSubs
		}
		if len(pos) > 1 && pos[1] == "list" && len(pos) <= 3 {
			return loadRepoHints(flags, profile)
		}
		return nil
	case "webhook":
		if len(pos) <= 2 && (len(pos) < 2 || pos[1] == current) {
			return hookSubs
		}
		if len(pos) > 1 && pos[1] == "list" && len(pos) <= 3 {
			return loadRepoHints(flags, profile)
		}
		return nil
	case "completion":
		if len(pos) <= 2 && (len(pos) < 2 || pos[1] == current) {
			return []string{"fish"}
		}
		return nil
	default:
		return nil
	}
}

func withoutAudience(cmds []string) []string {
	out := make([]string, 0, len(cmds))
	for _, item := range cmds {
		if item == audienceAgent || item == audienceHuman {
			continue
		}
		out = append(out, item)
	}
	return out
}

func safeTake(tokens []string) (map[string]string, []string) {
	flags, pos, err := takeCmd(tokens)
	if err != nil {
		var plain []string
		for _, item := range tokens {
			if !strings.HasPrefix(item, "-") {
				plain = append(plain, item)
			}
		}
		return map[string]string{}, plain
	}
	return flags, pos
}

func loadProfiles(flags map[string]string) []string {
	cfg, err := loadCfgQuiet(flags)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Profiles))
	for _, item := range cfg.Profiles {
		names = append(names, item.Name)
	}
	return names
}

func loadRepoHints(flags map[string]string, profile string) []string {
	cfg, err := loadCfgQuiet(flags)
	if err != nil {
		return nil
	}
	prof, err := PickProfile(cfg, profile)
	if err != nil {
		return nil
	}
	orgID, err := ParseOrgID(prof.URL)
	if err != nil {
		return nil
	}
	idx, err := LoadIndex(prof.Name, orgID)
	if err != nil {
		return nil
	}
	return HintRepos(idx.Repos)
}

func loadCfgQuiet(flags map[string]string) (FileCfg, error) {
	path, _ := firstFlag(flags, "-c", "--config")
	return LoadFileCfg(path)
}

func lastToken(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	return tokens[len(tokens)-1]
}
