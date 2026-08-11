/**
 * 生成 Fish completion, 因为 project 候选需要从 code-ws 配置实时读取。
 */
export function renderFishCompletion(): string {
  return [
    "complete -c code-ws -f",
    "function __code_ws_projects",
    "  code-ws projects 2>/dev/null",
    "end",
    "function __code_ws_needs_init_project",
    "  set -l tokens (commandline -opc)",
    "  test (count $tokens) -eq 3",
    "  and test $tokens[2] = init",
    "end",
    "complete -c code-ws -s v -l verbose -d '打印 git 命令'",
    "complete -c code-ws -l config -r -d '配置文件路径'",
    "complete -c code-ws -s t -l template -x -d 'profile 名称'",
    "complete -c code-ws -n '__fish_use_subcommand' -xa 'init add remove sync destroy fork list projects config serve completion help' -d '功能命令'",
    "complete -c code-ws -n '__code_ws_needs_init_project' -ka '(__code_ws_projects)' -d '项目名'",
    "complete -c code-ws -n '__fish_seen_subcommand_from add remove' -xa 'project' -d '项目操作'",
    "complete -c code-ws -n '__fish_seen_subcommand_from project' -ka '(__code_ws_projects)' -d '项目名'",
    "complete -c code-ws -n '__fish_seen_subcommand_from sync' -xa 'master' -d '同步主分支'",
    "complete -c code-ws -n '__fish_seen_subcommand_from config' -xa 'check' -d '检查配置'",
    "complete -c code-ws -n '__fish_seen_subcommand_from completion' -xa 'fish' -d '生成 fish completion'",
    "complete -c code-ws -n '__fish_seen_subcommand_from serve' -l lan -d '监听 0.0.0.0 (默认)'",
    "complete -c code-ws -n '__fish_seen_subcommand_from serve' -l local -d '仅监听 127.0.0.1'",
    "complete -c code-ws -n '__fish_seen_subcommand_from serve' -l port -r -d '端口, 默认 7001'",
    "complete -c code-ws -n '__fish_seen_subcommand_from serve' -l no-watch -d '关闭文件变更自动刷新'",
  ].join("\n");
}
