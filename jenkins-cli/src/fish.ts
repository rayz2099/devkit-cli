/** 为什么: fish 只转发整行, 候选由 __complete 按当前 token 计算. */
export function fishScript(): string {
  return `function __jenkins_cli_complete
    set -l tokens (commandline -opc)
    set -e tokens[1]
    set -l results (jenkins-cli __complete $tokens (commandline -ct))
    if test (count $results) -eq 0
        return
    end
    printf '%s\\n' $results
end

complete -c jenkins-cli -e
complete -c jenkins-cli -f
complete -c jenkins-cli -a '(__jenkins_cli_complete)'
complete -c jenkins-cli -s p -l profile -x -a '(__jenkins_cli_complete)' -d 'profile name'
complete -c jenkins-cli -l limit -r -d 'run list limit'
complete -c jenkins-cli -l tail -r -d 'log tail lines'
complete -c jenkins-cli -s h -l help -d 'show help'
`;
}
