/**
 * 生成 Fish shell completion 脚本。
 */
export function renderFishCompletion(programName = "har-cli"): string {
  return [
    `complete -c ${programName} -F`,
    `complete -c ${programName} -n '__fish_use_subcommand' -a help -d '显示帮助'`,
    `complete -c ${programName} -n '__fish_use_subcommand' -a completions -d '输出 shell 补全脚本'`,
    `complete -c ${programName} -n '__fish_seen_subcommand_from completions' -a fish -d '生成 Fish 补全脚本'`,
    `complete -c ${programName} -s f -l filter -d '设置过滤表达式' -r`,
    `complete -c ${programName} -s s -l select -d '设置输出字段投影' -a 'http path url prefix' -r`,
    `complete -c ${programName} -s p -l prefix-depth -d '设置前缀聚合深度' -r`,
    `complete -c ${programName} -s o -l format -d '设置输出格式' -a 'text json'`,
    `complete -c ${programName} -l no-dedupe -d '关闭去重输出'`,
    `complete -c ${programName} -s h -l help -d '显示帮助'`,
  ].join("\n");
}
