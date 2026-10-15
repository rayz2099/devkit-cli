package har

import "strings"

// Help 是原来的帮助文本。
// 参数说明和示例都从 TypeScript 原样搬过来，避免帮助里出现旧的 --where 或 cobra 自己的用法。
func Help() string {
	return strings.Join([]string{
		"Usage:",
		"  har-cli <har-file...> [-f <expr>] [-s <fields>] [-p <n>] [-o <mode>] [--no-dedupe]",
		"  har-cli help",
		"  har-cli completions fish",
		"",
		"Options:",
		"  -f, --filter <expr>    过滤表达式；未指定时不过滤",
		"  -s, --select <fields>  输出字段投影，默认 http；快捷值: http | path | url | prefix",
		"  -p, --prefix-depth <n> prefix 模式下按前 n 段聚合，默认 2",
		"  -o, --format <mode>    输出格式: text | json，默认 text",
		"  --no-dedupe            关闭去重输出",
		"  -h, --help             显示帮助",
		"",
		"Examples:",
		"  har-cli sample.har",
		"  har-cli sample.har -f 'body.status == 1 and http.status == 403' -s path",
		"  har-cli sample.har -s path,http.status",
		"  har-cli sample.har -s prefix -p 2",
	}, "\n")
}

// Fish 是原来的 fish 补全脚本。
// 必须保留 complete -c 和 -F，shell 才能按文件名前缀补 HAR，不能换成 cobra 生成的补全。
func Fish() string {
	return strings.Join([]string{
		"complete -c har-cli -F",
		"complete -c har-cli -n '__fish_use_subcommand' -a help -d '显示帮助'",
		"complete -c har-cli -n '__fish_use_subcommand' -a completions -d '输出 shell 补全脚本'",
		"complete -c har-cli -n '__fish_seen_subcommand_from completions' -a fish -d '生成 Fish 补全脚本'",
		"complete -c har-cli -s f -l filter -d '设置过滤表达式' -r",
		"complete -c har-cli -s s -l select -d '设置输出字段投影' -a 'http path url prefix' -r",
		"complete -c har-cli -s p -l prefix-depth -d '设置前缀聚合深度' -r",
		"complete -c har-cli -s o -l format -d '设置输出格式' -a 'text json'",
		"complete -c har-cli -l no-dedupe -d '关闭去重输出'",
		"complete -c har-cli -s h -l help -d '显示帮助'",
	}, "\n")
}
