/** 为什么: fish 只把整行交过来, 任务名必须现读配置, 不能在生成补全脚本时写死. */
export function fishScript(): string {
  return `function __rclone_cli_complete
    set -l tokens (commandline -opc)
    set -e tokens[1]
    set -l results (rclone-cli __complete $tokens (commandline -ct))
    if test (count $results) -eq 0
        return
    end
    printf '%s\\n' $results
end

complete -c rclone-cli -e
complete -c rclone-cli -f
complete -c rclone-cli -n '__fish_seen_subcommand_from push pull check get download ls' -F
complete -c rclone-cli -a '(__rclone_cli_complete)'
complete -c rclone-cli -l dry-run -d 'trial run, copy only'
complete -c rclone-cli -s v -l verbose -d 'print rclone logs'
complete -c rclone-cli -s h -l help -d 'show help'
`;
}
