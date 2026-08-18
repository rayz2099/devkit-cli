/** 为什么: fish 只转发整行, 候选由 __complete 按当前 token 计算. */
export function fishScript(): string {
  return `function __codeup_cli_complete
    set -l tokens (commandline -opc)
    set -e tokens[1]
    set -l results (codeup-cli __complete $tokens (commandline -ct))
    if test (count $results) -eq 0
        return
    end
    printf '%s\\n' $results
end

complete -c codeup-cli -e
complete -c codeup-cli -f
complete -c codeup-cli -a '(__codeup_cli_complete)'
complete -c codeup-cli -s p -l profile -x -a '(__codeup_cli_complete)' -d 'profile name'
complete -c codeup-cli -l repo -r -d 'group/project or numeric id'
complete -c codeup-cli -l search -r -d 'name or title filter'
complete -c codeup-cli -l source -r -d 'source branch'
complete -c codeup-cli -l target -r -d 'target branch'
complete -c codeup-cli -l title -r -d 'change request title'
complete -c codeup-cli -l body -r -d 'change request body'
complete -c codeup-cli -l body-file -r -d 'read body from file'
complete -c codeup-cli -l remote -r -d 'git remote'
complete -c codeup-cli -l show-secrets -d 'show webhook secretToken'
complete -c codeup-cli -s h -l help -d 'show help'
`;
}
