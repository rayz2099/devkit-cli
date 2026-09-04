/** 为什么: fish 只转发整行, 候选由 __complete 按当前 token 计算. */
export function fishScript(): string {
  return `function __dsn_cli_complete
    set -l tokens (commandline -opc)
    set -e tokens[1]
    set -l results (dsn-cli __complete $tokens (commandline -ct))
    if test (count $results) -eq 0
        return
    end
    printf '%s\\n' $results
end

complete -c dsn-cli -e
complete -c dsn-cli -f
complete -c dsn-cli -a '(__dsn_cli_complete)'
complete -c dsn-cli -s c -l config -r -d 'config file'
complete -c dsn-cli -s p -l profile -x -a '(__dsn_cli_complete)' -d 'profile name'
complete -c dsn-cli -l output -x -a 'json csv plain' -d 'human query format'
complete -c dsn-cli -l pretty -d 'pretty-print json output'
complete -c dsn-cli -l limit -r -d 'agent row cap'
complete -c dsn-cli -l timeout -r -d 'query timeout seconds'
complete -c dsn-cli -l connect-timeout -r -d 'connect timeout seconds'
complete -c dsn-cli -s h -l help -d 'show help'
`;
}
