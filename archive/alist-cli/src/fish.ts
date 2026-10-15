/** 为什么: fish 只转发整行, 候选由 __complete 按当前 token 计算. */
export function fishScript(): string {
  return `function __alist_cli_complete
    set -l tokens (commandline -opc)
    set -e tokens[1]
    set -l results (alist-cli __complete $tokens (commandline -ct))
    if test (count $results) -eq 0
        return
    end
    printf '%s\\n' $results
end

complete -c alist-cli -e
complete -c alist-cli -f
complete -c alist-cli -n '__fish_seen_subcommand_from put sync' -F
complete -c alist-cli -a '(__alist_cli_complete)'
complete -c alist-cli -s c -l config -r -d 'config file'
complete -c alist-cli -s p -l profile -x -a '(__alist_cli_complete)' -d 'profile name'
complete -c alist-cli -l page -r -d 'page number'
complete -c alist-cli -l size -r -d 'page size'
complete -c alist-cli -l password -r -d 'directory password'
complete -c alist-cli -l src -r -d 'local source path'
complete -c alist-cli -l dst -r -d 'remote destination path'
complete -c alist-cli -l refresh -d 'refresh directory listing'
complete -c alist-cli -l as-task -d 'upload as alist task'
complete -c alist-cli -s h -l help -d 'show help'
`;
}
