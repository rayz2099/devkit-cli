package jenkins

import "strings"

// root 补全不含 profile 名。旧 CLI 的 loadProfilesIfFlag 无论有没有 -p 都返回空，这里保持一样。
var rootCmds = []string{"job", "run", "log", "queue", "completion", "agent", "human"}

var agentCmds = []string{"status", "list", "info", "log", "trigger", "job", "run", "queue"}

var jobSubs = []string{"ls", "view"}

var runSubs = []string{"ls", "view", "start", "cancel", "rerun"}

var queueSubs = []string{"ls"}

// fishScript 只打印补全脚本，不往配置目录安装。安装会写文件，这个 CLI 不碰配置目录。
func fishScript() string {
	return `function __jenkins_cli_complete
    set -l tokens (commandline -opc)
    set -e tokens[1]
    set -l results (jenkins-cli __complete $tokens (commandline -ct))
    if test (count $results) -eq 0
        return
    end
    printf '%s\n' $results
end

complete -c jenkins-cli -e
complete -c jenkins-cli -f
complete -c jenkins-cli -a '(__jenkins_cli_complete)'
complete -c jenkins-cli -s c -l config -r -d 'config file'
complete -c jenkins-cli -s p -l profile -x -a '(__jenkins_cli_complete)' -d 'profile name'
complete -c jenkins-cli -l limit -r -d 'run list limit'
complete -c jenkins-cli -l tail -r -d 'log tail lines'
complete -c jenkins-cli -s h -l help -d 'show help'
`
}

// completeLines 按当前 token 过滤候选。没有把握的 JobPath 不猜，避免补全写出不存在的任务。
func completeLines(tokens []string, current string) string {
	vals := completeValues(tokens, current)
	matched := make([]string, 0, len(vals))
	for _, item := range vals {
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
	last := ""
	if len(tokens) > 0 {
		last = tokens[len(tokens)-1]
	}
	if last == "-c" || last == "--config" {
		return nil
	}
	flags, pos := safeTake(tokens)
	if last == "-p" || last == "--profile" {
		return loadNames(flags)
	}
	if len(pos) == 0 {
		return rootCmds
	}
	if len(pos) == 1 && current != "" && pos[0] == current {
		return rootCmds
	}
	head := pos[0]
	if head == "agent" || head == "human" {
		return completeAud(pos[1:], current)
	}
	return completeCmd(pos, current)
}

func completeAud(pos []string, current string) []string {
	if len(pos) == 0 || (len(pos) == 1 && pos[0] == current) {
		return agentCmds
	}
	return completeCmd(pos, current)
}

func completeCmd(pos []string, current string) []string {
	if len(pos) == 0 {
		return nil
	}
	switch pos[0] {
	case "job":
		return subIf(pos, current, jobSubs)
	case "run":
		return subIf(pos, current, runSubs)
	case "queue":
		return subIf(pos, current, queueSubs)
	case "completion":
		return subIf(pos, current, []string{"fish"})
	case "status", "list", "info", "trigger", "log":
		return nil
	default:
		return nil
	}
}

func subIf(pos []string, current string, subs []string) []string {
	if len(pos) <= 2 && (len(pos) < 2 || pos[1] == current) {
		return subs
	}
	return nil
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

func loadNames(flags map[string]string) []string {
	cfg, err := LoadFileCfg(firstNonEmpty(flags["-c"], flags["--config"]))
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Profiles))
	for _, prof := range cfg.Profiles {
		names = append(names, prof.Name)
	}
	return names
}
