package nacos

import "strings"

// fishScript 只把当前命令行交回 __complete。
// 候选仍按 cobra 的 :directive 协议算，shell 侧不再自己拼 namespace。
func fishScript() string {
	return `function __nacos_cli_complete
    set -l tokens (commandline -opc)
    set -e tokens[1]
    set -l results (nacos-cli __complete $tokens (commandline -ct))
    if test (count $results) -eq 0
        return
    end
    if string match -q -- ':*' $results[-1]
        set results $results[1..-2]
    end
    printf '%s\n' $results
end

complete -c nacos-cli -e
complete -c nacos-cli -f
complete -c nacos-cli -a '(__nacos_cli_complete)'
complete -c nacos-cli -s c -l config -r -d 'config file'
complete -c nacos-cli -s o -l output -x -a '(__nacos_cli_complete)' -d 'output format'
complete -c nacos-cli -l server-addr -r -d 'nacos server address'
complete -c nacos-cli -l username -r -d 'nacos username'
complete -c nacos-cli -l password -r -d 'nacos password'
complete -c nacos-cli -o ns -x -a '(__nacos_cli_complete)' -d 'nacos namespace'
complete -c nacos-cli -l namespace -x -a '(__nacos_cli_complete)' -d 'nacos namespace'
complete -c nacos-cli -l data-id -x -a '(__nacos_cli_complete)' -d 'config data id'
complete -c nacos-cli -l group -x -a '(__nacos_cli_complete)' -d 'config group'
complete -c nacos-cli -l content -r -d 'config content'
complete -c nacos-cli -l search -x -a '(__nacos_cli_complete)' -d 'search mode'
complete -c nacos-cli -l page-no -r -d 'page number'
complete -c nacos-cli -l page-size -r -d 'page size'
complete -c nacos-cli -l service -r -d 'service name'
complete -c nacos-cli -l ip -r -d 'instance ip'
complete -c nacos-cli -l port -r -d 'instance port'
complete -c nacos-cli -l cluster -r -d 'cluster name'
complete -c nacos-cli -l clusters -r -d 'comma separated clusters'
complete -c nacos-cli -l weight -r -d 'instance weight'
complete -c nacos-cli -l ephemeral -x -a 'true false' -d 'ephemeral instance'
complete -c nacos-cli -l healthy-only -x -a 'true false' -d 'only healthy instances'
complete -c nacos-cli -l dev -d 'enable dev log'
complete -c nacos-cli -s h -l help -d 'show help'
`
}

// groupsByDataID 只返回这个 dataId 下的 group。没有匹配时才回落到 COMMON。
func groupsByDataID(items []configItem, dataID, prefix string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if item.dataID != dataID {
			continue
		}
		group := strings.TrimSpace(item.group)
		if group == "" || !strings.HasPrefix(group, prefix) {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		out = append(out, group)
	}
	if len(out) == 0 && strings.HasPrefix("COMMON", prefix) {
		return []string{"COMMON"}
	}
	if out == nil {
		return []string{}
	}
	return out
}

// uniqueDataIDs 去掉重复 dataId，避免同一个配置的多个 group 把补全刷屏。
func uniqueDataIDs(items []configItem, prefix string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		id := strings.TrimSpace(item.dataID)
		if id == "" || !strings.HasPrefix(id, prefix) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if out == nil {
		return []string{}
	}
	return out
}
