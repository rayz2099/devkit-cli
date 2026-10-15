package rclone

import "strings"

const fishScript = "" +
	"function __rclone_cli_complete\n" +
	"    set -l tokens (commandline -opc)\n" +
	"    set -e tokens[1]\n" +
	"    set -l results (rclone-cli __complete $tokens (commandline -ct))\n" +
	"    if test (count $results) -eq 0\n" +
	"        return\n" +
	"    end\n" +
	"    printf '%s\\n' $results\n" +
	"end\n" +
	"\n" +
	"complete -c rclone-cli -e\n" +
	"complete -c rclone-cli -f\n" +
	"complete -c rclone-cli -n '__fish_seen_subcommand_from push pull check get download ls' -F\n" +
	"complete -c rclone-cli -a '(__rclone_cli_complete)'\n" +
	"complete -c rclone-cli -l dry-run -d 'trial run, copy only'\n" +
	"complete -c rclone-cli -s v -l verbose -d 'print rclone logs'\n" +
	"complete -c rclone-cli -s h -l help -d 'show help'\n"

var completeRoot = []string{"push", "pull", "check", "get", "download", "ls", "runs", "completion", "agent", "human"}
var completeAfter = []string{"push", "pull", "check", "get", "download", "ls", "runs", "completion"}

func completeLines(tokens []string, current, home, cfgFlag string) string {
	values := completeValues(tokens, current, home, cfgFlag)
	var matched []string
	for _, item := range values {
		if strings.HasPrefix(item, current) {
			matched = append(matched, item)
		}
	}
	if len(matched) == 0 {
		return ""
	}
	return joinLines(matched) + "\n"
}

func completeValues(tokens []string, current, home, cfgFlag string) []string {
	var pos []string
	for _, item := range tokens {
		if len(item) == 0 || item[0] != '-' {
			pos = append(pos, item)
		}
	}
	head := first(pos)
	if head == "" || (len(pos) == 1 && head == current) {
		return completeRoot
	}
	if head == "agent" || head == "human" {
		return completeAfterAud(pos[1:], current, home, cfgFlag)
	}
	return completeCmd(pos, current, home, cfgFlag)
}

func completeAfterAud(pos []string, current, home, cfgFlag string) []string {
	head := first(pos)
	if head == "" || (len(pos) == 1 && head == current) {
		return completeAfter
	}
	return completeCmd(pos, current, home, cfgFlag)
}

func completeCmd(pos []string, current, home, cfgFlag string) []string {
	head := first(pos)
	if head == "completion" {
		if len(pos) <= 2 && (len(pos) < 2 || pos[1] == current) {
			return []string{"fish"}
		}
		return nil
	}
	if head == "push" || head == "pull" || head == "check" || head == "get" || head == "download" || head == "ls" {
		if len(pos) <= 2 && (len(pos) < 2 || pos[1] == current) {
			return loadTaskNames(home, cfgFlag)
		}
		if head == "push" || head == "pull" || head == "get" || head == "download" {
			var flags []string
			if !containsStr(pos, "--dry-run") {
				flags = append(flags, "--dry-run")
			}
			if (head == "get" || head == "download") && !containsStr(pos, "-v") && !containsStr(pos, "--verbose") {
				flags = append(flags, "--verbose")
			}
			return flags
		}
		if head == "ls" && !containsStr(pos, "--limit") && !containsStr(pos, "-n") {
			return []string{"--limit"}
		}
	}
	return nil
}

func loadTaskNames(home, cfgFlag string) []string {
	path, err := cfgFile(cfgFlag)
	if err != nil {
		return nil
	}
	cfg, err := Load(path, home)
	if err != nil {
		// 补全发生在命令还没写完时。配置缺失要给出空候选，不能把错误文本塞进候选列表。
		return nil
	}
	return taskNames(cfg)
}
