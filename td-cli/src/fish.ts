/** 为什么: fish 只转发整行, 候选由 __complete 按当前 token 计算. */
export function fishScript(): string {
  return `function __td_cli_complete
    set -l tokens (commandline -opc)
    set -e tokens[1]
    set -l results (td-cli __complete $tokens (commandline -ct))
    if test (count $results) -eq 0
        return
    end
    printf '%s\\n' $results
end

complete -c td-cli -e
complete -c td-cli -f
complete -c td-cli -a '(__td_cli_complete)'
complete -c td-cli -s c -l config -r -d 'config file'
complete -c td-cli -s p -l profile -x -a '(__td_cli_complete)' -d 'profile name'
complete -c td-cli -l project -r -d 'project id'
complete -c td-cli -l event -r -d 'event name'
complete -c td-cli -l id -r -d 'report id'
complete -c td-cli -l analysis -r -d 'analysis code'
complete -c td-cli -l model -r -d 'event model'
complete -c td-cli -l qp -r -d 'qp json file'
complete -c td-cli -l timeout -r -d 'query timeout seconds'
complete -c td-cli -s h -l help -d 'show help'
`;
}
