package codeup

import "strings"

var valueFlags = map[string]struct{}{
	"-p":          {},
	"--profile":   {},
	"--repo":      {},
	"--search":    {},
	"--page":      {},
	"--per-page":  {},
	"--state":     {},
	"--source":    {},
	"--target":    {},
	"--title":     {},
	"--body":      {},
	"--body-file": {},
	"--message":   {},
	"--type":      {},
	"--remote":    {},
	"-c":          {},
	"--config":    {},
}

var boolFlags = map[string]struct{}{
	"-h":             {},
	"--help":         {},
	"--show-secrets": {},
}

// ParseArgs 自己扫 argv。audience 前缀和 -p 可以插在命令前后，不能交给只认固定位置的解析器。
func ParseArgs(argv []string) (Cmd, error) {
	if len(argv) > 0 && argv[0] == "completion" {
		if len(argv) > 1 && argv[1] == "fish" {
			return Cmd{Kind: kindFish}, nil
		}
		return Cmd{}, fail(1, "usage: codeup-cli completion fish")
	}
	if len(argv) > 0 && argv[0] == "__complete" {
		return parseComplete(argv[1:]), nil
	}
	flags, pos, err := takeCmd(argv)
	if err != nil {
		return Cmd{}, err
	}
	cmd, err := parseCmd(flags, pos)
	if err != nil {
		return Cmd{}, err
	}
	return attachCfg(cmd, flags), nil
}

func parseComplete(argv []string) Cmd {
	tokens := append([]string{}, argv...)
	current := ""
	if len(tokens) > 0 {
		current = tokens[len(tokens)-1]
		tokens = tokens[:len(tokens)-1]
	}
	return Cmd{Kind: kindComplete, Tokens: tokens, Current: current}
}

func parseCmd(flags map[string]string, pos []string) (Cmd, error) {
	if _, help := flags["-h"]; help {
		return helpCmd(pos, false), nil
	}
	if _, help := flags["--help"]; help {
		return helpCmd(pos, false), nil
	}
	if len(pos) > 0 && pos[0] == "help" {
		return helpCmd(pos, true), nil
	}
	audience := audienceHuman
	rest := append([]string{}, pos...)
	if len(rest) > 0 && (rest[0] == audienceAgent || rest[0] == audienceHuman) {
		audience = rest[0]
		rest = rest[1:]
	}
	profile, _ := firstFlag(flags, "-p", "--profile")
	if len(rest) == 0 {
		return Cmd{Kind: kindHelp}, nil
	}
	head := rest[0]
	switch head {
	case "init":
		return Cmd{Kind: kindInit, Audience: audience, Profile: profile}, nil
	case "repos":
		return parseRepos(flags, audience, profile)
	case "push":
		return parsePush(rest[1:], flags, audience, profile), nil
	case "cr":
		return parseCr(rest[1:], flags, audience, profile)
	case "webhook":
		return parseWebhook(rest[1:], flags, audience, profile)
	case "doctor":
		return Cmd{Kind: kindDoctor, Audience: audience, Profile: profile}, nil
	case "version":
		return Cmd{Kind: kindVersion}, nil
	default:
		return Cmd{}, failf(1, "unknown command: %s", head)
	}
}

func helpCmd(pos []string, helpWord bool) Cmd {
	topic := ""
	if helpWord {
		if len(pos) > 1 {
			topic = pos[1]
		}
		return Cmd{Kind: kindHelp, Topic: topic}
	}
	if len(pos) > 0 {
		topic = pos[0]
	}
	return Cmd{Kind: kindHelp, Topic: topic}
}

func parseRepos(flags map[string]string, audience, profile string) (Cmd, error) {
	page, err := readIntFlag(flags, "--page", 1)
	if err != nil {
		return Cmd{}, err
	}
	per, err := readIntFlag(flags, "--per-page", 50)
	if err != nil {
		return Cmd{}, err
	}
	search, _ := flags["--search"]
	return Cmd{
		Kind:     kindRepos,
		Audience: audience,
		Profile:  profile,
		Search:   search,
		Page:     page,
		PerPage:  per,
	}, nil
}

func parsePush(argv []string, flags map[string]string, audience, profile string) Cmd {
	remote := "origin"
	if value, ok := flags["--remote"]; ok {
		remote = value
	}
	cmd := Cmd{
		Kind:     kindPush,
		Audience: audience,
		Profile:  profile,
		Remote:   remote,
	}
	if len(argv) > 0 {
		cmd.Branch = argv[0]
		cmd.HasBranch = true
	}
	return cmd
}

func parseCr(argv []string, flags map[string]string, audience, profile string) (Cmd, error) {
	if len(argv) == 0 || argv[0] == "help" {
		return Cmd{Kind: kindHelp, Topic: "cr"}, nil
	}
	sub := argv[0]
	switch sub {
	case "list":
		return parseCrList(argv, flags, audience, profile)
	case "get":
		return parseCrGet(argv, flags, audience, profile)
	case "create":
		return parseCrCreate(flags, audience, profile)
	case "merge":
		return parseCrMerge(argv, flags, audience, profile)
	default:
		return Cmd{}, failf(1, "unknown command: cr %s", sub)
	}
}

func parseCrList(argv []string, flags map[string]string, audience, profile string) (Cmd, error) {
	page, err := readIntFlag(flags, "--page", 1)
	if err != nil {
		return Cmd{}, err
	}
	per, err := readIntFlag(flags, "--per-page", 20)
	if err != nil {
		return Cmd{}, err
	}
	state := "opened"
	if value, ok := flags["--state"]; ok {
		state = value
	}
	repo := ""
	if value, ok := firstFlag(flags, "--repo"); ok {
		repo = value
	} else if len(argv) > 1 && strings.TrimSpace(argv[1]) != "" {
		repo = argv[1]
	}
	cmd := Cmd{
		Kind:     kindCrList,
		Audience: audience,
		Profile:  profile,
		Repo:     repo,
		State:    state,
		Search:   flags["--search"],
		Page:     page,
		PerPage:  per,
	}
	if value, ok := flags["--source"]; ok {
		src := value
		cmd.Source = &src
	}
	if value, ok := flags["--target"]; ok {
		tgt := value
		cmd.Target = &tgt
	}
	return cmd, nil
}

func parseCrGet(argv []string, flags map[string]string, audience, profile string) (Cmd, error) {
	localID, err := needArg(argv, 1, "cr get: missing <localId>")
	if err != nil {
		return Cmd{}, err
	}
	repo, _ := flags["--repo"]
	return Cmd{
		Kind:     kindCrGet,
		Audience: audience,
		Profile:  profile,
		Repo:     repo,
		LocalID:  localID,
	}, nil
}

func parseCrCreate(flags map[string]string, audience, profile string) (Cmd, error) {
	source, err := needFlag(flags, "--source", "cr create: missing --source")
	if err != nil {
		return Cmd{}, err
	}
	title, err := needFlag(flags, "--title", "cr create: missing --title")
	if err != nil {
		return Cmd{}, err
	}
	cmd := Cmd{
		Kind:     kindCrCreate,
		Audience: audience,
		Profile:  profile,
		Repo:     flags["--repo"],
		Source:   &source,
		Title:    title,
	}
	if value, ok := flags["--target"]; ok {
		tgt := value
		cmd.Target = &tgt
	}
	if value, ok := flags["--body"]; ok {
		body := value
		cmd.Body = &body
	}
	if value, ok := flags["--body-file"]; ok {
		file := value
		cmd.BodyFile = &file
	}
	return cmd, nil
}

func parseCrMerge(argv []string, flags map[string]string, audience, profile string) (Cmd, error) {
	localID, err := needArg(argv, 1, "cr merge: missing <localId>")
	if err != nil {
		return Cmd{}, err
	}
	kind, err := parseMergeType(flags)
	if err != nil {
		return Cmd{}, err
	}
	cmd := Cmd{
		Kind:      kindCrMerge,
		Audience:  audience,
		Profile:   profile,
		Repo:      flags["--repo"],
		LocalID:   localID,
		MergeType: kind,
	}
	if value, ok := flags["--message"]; ok {
		msg := value
		cmd.Message = &msg
	}
	return cmd, nil
}

func parseWebhook(argv []string, flags map[string]string, audience, profile string) (Cmd, error) {
	if len(argv) == 0 || argv[0] == "help" {
		return Cmd{Kind: kindHelp, Topic: "webhook"}, nil
	}
	if argv[0] != "list" {
		return Cmd{}, failf(1, "unknown command: webhook %s", argv[0])
	}
	page, err := readIntFlag(flags, "--page", 1)
	if err != nil {
		return Cmd{}, err
	}
	per, err := readIntFlag(flags, "--per-page", 50)
	if err != nil {
		return Cmd{}, err
	}
	repo := ""
	if value, ok := firstFlag(flags, "--repo"); ok {
		repo = value
	} else if len(argv) > 1 && strings.TrimSpace(argv[1]) != "" {
		repo = argv[1]
	}
	_, show := flags["--show-secrets"]
	return Cmd{
		Kind:        kindHookList,
		Audience:    audience,
		Profile:     profile,
		Repo:        repo,
		ShowSecrets: show,
		Page:        page,
		PerPage:     per,
	}, nil
}

// ParseMergeType 省略才是 squash。未知值立刻失败，不能静默落到默认策略。
func ParseMergeType(raw string, ok bool) (string, error) {
	if !ok || strings.TrimSpace(raw) == "" {
		return "squash", nil
	}
	for _, item := range MergeTypes {
		if item == raw {
			return item, nil
		}
	}
	return "", failf(1, "cr merge: unknown --type %s", raw)
}

func parseMergeType(flags map[string]string) (string, error) {
	raw, ok := flags["--type"]
	return ParseMergeType(raw, ok)
}

func takeCmd(argv []string) (map[string]string, []string, error) {
	flags := map[string]string{}
	var pos []string
	for i := 0; i < len(argv); i++ {
		token := argv[i]
		if _, ok := valueFlags[token]; ok {
			if i+1 >= len(argv) {
				return nil, nil, failf(1, "%s requires a value", token)
			}
			flags[token] = argv[i+1]
			i++
			continue
		}
		if _, ok := boolFlags[token]; ok {
			flags[token] = "true"
			continue
		}
		if strings.HasPrefix(token, "-") {
			return nil, nil, failf(1, "unknown flag: %s", token)
		}
		pos = append(pos, token)
	}
	return flags, pos, nil
}

func readIntFlag(flags map[string]string, key string, whenOmit int) (int, error) {
	raw, ok := flags[key]
	if !ok {
		return whenOmit, nil
	}
	value, good := jsInt(raw)
	if !good || value <= 0 {
		return 0, failf(1, "invalid number: %s", raw)
	}
	return value, nil
}

func jsInt(raw string) (int, bool) {
	i := 0
	for i < len(raw) {
		switch raw[i] {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			i++
			continue
		}
		break
	}
	if i >= len(raw) {
		return 0, false
	}
	sign := 1
	if raw[i] == '+' || raw[i] == '-' {
		if raw[i] == '-' {
			sign = -1
		}
		i++
	}
	if i >= len(raw) || raw[i] < '0' || raw[i] > '9' {
		return 0, false
	}
	n := 0
	for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
		next := n*10 + int(raw[i]-'0')
		if next < n {
			return 0, false
		}
		n = next
		i++
	}
	return sign * n, true
}

func attachCfg(cmd Cmd, flags map[string]string) Cmd {
	if value, ok := firstFlag(flags, "-c", "--config"); ok {
		cmd.Config = value
	}
	return cmd
}

func firstFlag(flags map[string]string, keys ...string) (string, bool) {
	for _, key := range keys {
		value, ok := flags[key]
		if ok && strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	return "", false
}

func needFlag(flags map[string]string, key, msg string) (string, error) {
	value, ok := flags[key]
	if !ok || strings.TrimSpace(value) == "" {
		return "", fail(1, msg)
	}
	return value, nil
}

func needArg(argv []string, index int, msg string) (string, error) {
	if index >= len(argv) || strings.TrimSpace(argv[index]) == "" {
		return "", fail(1, msg)
	}
	return argv[index], nil
}

// HelpText 是人读的用法。agent 不走这里，机器输出只在各命令的 JSON。
func HelpText(topic string) string {
	switch topic {
	case "cr":
		return `codeup-cli cr

Usage:
  codeup-cli cr list [--repo group/project] [--state opened|merged|closed|all]
  codeup-cli cr get <localId> [--repo group/project]
  codeup-cli cr create --source <branch> --title <title> [--target <branch>]
  codeup-cli cr merge <localId> [--repo group/project] [--type squash|ff-only|no-fast-forward|rebase] [--message <text>]
`
	case "webhook":
		return `codeup-cli webhook

Usage:
  codeup-cli webhook list [--repo group/project] [--show-secrets]
`
	case "agent":
		return `codeup-cli agent

Usage:
  codeup-cli agent init
  codeup-cli agent repos
  codeup-cli agent push
  codeup-cli agent cr list|get|create|merge
  codeup-cli agent webhook list
`
	default:
		return `codeup-cli

Usage:
  codeup-cli [-c config.json] [-p profile] [agent|human] <command>

Commands:
  init
  repos [--search S]
  push [--remote origin] [branch]
  cr list|get|create|merge
  webhook list
  doctor
  version
  completion fish
`
	}
}
