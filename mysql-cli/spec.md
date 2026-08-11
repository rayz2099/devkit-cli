# mysql-cli v1

## Summary

- v1 只做本机 `mysql` 包装器, 通过 `which mysql` 定位已安装客户端并转发执行.
- 支持两种核心模式: `-e` 执行 SQL 后退出, 以及进入 `mysql` 交互式 shell.
- 屏蔽大部分 mysql 原生参数, 只暴露 profile 选择, `-e`, `--output`, `help`.

## 技术栈

- `bun + ts`

## Install

- 使用 `justfile`.
- 支持 `just clean`, `just build`, `just install`.

## CLI

- `mysql-cli -p <profile>`: 使用 profile 进入交互式 mysql shell.
- `mysql-cli -p <profile> -e "select 1"`: 转发到 mysql 执行 SQL 后退出.
- `mysql-cli -p <profile> -e "select 1" --output json`: 输出 JSON.
- `mysql-cli -p <profile> -e "select 1" --output csv`: 输出 CSV.
- `mysql-cli help` / `mysql-cli --help`: 展示本工具支持的最小参数.

## Output

- `--output` 只对 `-e` 生效.
- 缺省不传 `--output` 时, 保持 mysql 原生输出.
- v1 支持 `json`, `csv`.
- `json` 输出 NDJSON/JSON Lines, 一行一个 JSON object, column name 作为 key.
- `csv` 输出包含 header 行.
- `--output json|csv` 内部使用 mysql batch/raw 输出作为中间格式, 再转换为目标格式.
- `--output json` 采用流式转换: 读取 header 后, 每收到一行 mysql 结果就输出一个 JSON object.

## Fish 集成

- `mysql-cli -p <tab>` 从配置文件 profiles 中补全 profile name.

## 配置文件

位置: `~/.config/mysql-cli/config.json`

```json
{
  "profiles": [
    {
      "name": "testdb",
      "jdbcUrl": "jdbc:mysql://127.0.0.1:3306/?user=adm&password=secret&useUnicode=true&characterEncoding=utf-8&serverTimezone=Asia/Shanghai"
    }
  ]
}
```

解析规则:

- v1 使用 `jdbcUrl` 保存连接信息, 可包含密码.
- database 可缺省, 缺省时不指定默认 database.
- 只解析 `host`, `port`, `user`, `password`, `database`.
- 其他 JDBC query 参数忽略, 例如 `useUnicode`, `characterEncoding`, `serverTimezone`.
- query 中的 `password` 需要 URL decode.

`jdbcUrl` 解析为 mysql 参数:

- host -> `-h`
- port -> `-P`
- user -> `-u`
- password -> `-p<decodedPassword>`
- path database 非空且不是 `/` 时, 作为 mysql 最后的位置参数.

## Error Handling

- 找不到 profile: 报错并列出可用 profile.
- 找不到本机 `mysql`: 报错提示安装 mysql client.
- `jdbcUrl` 非法: 报错并指出 profile name.
- `--output` 未搭配 `-e`: 报错.
- `--output` 不是 `json|csv`: 报错.

## Out of Scope

- 透传 mysql 大量原生参数.
- 自实现 MySQL 协议 client.
- 密钥管理或 password prompt 管理.

## Test Plan

- `help` 输出只包含 v1 支持参数.
- config 解析:
  - 带 database 的 jdbcUrl.
  - 不带 database 的 jdbcUrl.
  - 带 URL encoded password 的 jdbcUrl.
  - 带无关 JDBC 参数的 jdbcUrl.
- 命令构造:
  - `mysql-cli -p testdb` 构造交互式 mysql 命令.
  - `mysql-cli -p testdb -e "select 1"` 构造非交互执行命令.
  - `mysql-cli -p testdb -e "select 1" --output json|csv` 构造 batch 查询并转换输出.
- output 转换:
  - JSON 正确处理多列, 多行, 空结果, 并按 NDJSON 流式输出.
  - CSV 正确输出 header, 多列, 多行, 逗号/引号/换行转义.
- fish completion:
  - 能从 config profiles 输出补全候选.
- 错误场景:
  - config 不存在.
  - profile 不存在.
  - jdbcUrl 非法.
  - `--output` 传入不支持类型.
  - `--output` 未搭配 `-e`.
  - `which mysql` 找不到客户端.

## Assumptions

- `-p` 在本工具中固定表示 profile, 不兼容 mysql 原生 password 参数.
- v1 允许密码存在 `jdbcUrl` 中, 用户自行控制 config 文件权限.
- `--output` 当前只支持 `json` 和 `csv`, 后续可扩展.
