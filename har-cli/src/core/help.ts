/**
 * 生成 CLI 帮助文本，统一维护参数说明与示例。
 */
export function renderHelp(programName = "har-cli"): string {
  return [
    `Usage:`,
    `  ${programName} <har-file...> [-f <expr>] [-s <fields>] [-p <n>] [-o <mode>] [--no-dedupe]`,
    `  ${programName} help`,
    `  ${programName} completions fish`,
    ``,
    `Options:`,
    `  -f, --filter <expr>    过滤表达式；未指定时不过滤`,
    `  -s, --select <fields>  输出字段投影，默认 http；快捷值: http | path | url | prefix`,
    `  -p, --prefix-depth <n> prefix 模式下按前 n 段聚合，默认 2`,
    `  -o, --format <mode>    输出格式: text | json，默认 text`,
    `  --no-dedupe            关闭去重输出`,
    `  -h, --help             显示帮助`,
    ``,
    `Examples:`,
    `  ${programName} sample.har`,
    `  ${programName} sample.har -f 'body.status == 1 and http.status == 403' -s path`,
    `  ${programName} sample.har -s path,http.status`,
    `  ${programName} sample.har -s prefix -p 2`,
  ].join("\n");
}
