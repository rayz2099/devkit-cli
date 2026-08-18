import type { ConfigItem } from "./types";

/** 为什么: fish 只转发整行, 候选由 __complete 按 cobra ValidArgs 语义计算. */
export function fishScript(): string {
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
complete -c nacos-cli -s o -l output -x -a '(__nacos_cli_complete)' -d 'output format'
complete -c nacos-cli -l server-addr -r -d 'nacos server address'
complete -c nacos-cli -l username -r -d 'nacos username'
complete -c nacos-cli -l password -r -d 'nacos password'
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
`;
}

/** 为什么: group 补全必须跟已选 dataId 绑定, 否则会把别的配置组灌进去. */
export function groupsByDataId(
  items: ConfigItem[],
  dataId: string,
  prefix: string,
): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of items) {
    if (item.dataId !== dataId) {
      continue;
    }
    const group = item.group.trim();
    if (group === "" || !group.startsWith(prefix) || seen.has(group)) {
      continue;
    }
    seen.add(group);
    result.push(group);
  }
  if (result.length === 0 && "COMMON".startsWith(prefix)) {
    return ["COMMON"];
  }
  return result;
}

/** 为什么: dataId 补全要去重, 否则同一配置多 group 会把候选刷屏. */
export function uniqueDataIds(items: ConfigItem[], prefix: string): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of items) {
    const id = item.dataId.trim();
    if (id === "" || !id.startsWith(prefix) || seen.has(id)) {
      continue;
    }
    seen.add(id);
    result.push(id);
  }
  return result;
}
