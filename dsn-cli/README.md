# dsn-cli

按 profile 查询 mysql / doris / redis / mongodb / elasticsearch。人可以进官方客户端；agent 只能走 `query`。

没有 `defaultProfile`。`query` / Console 必须带 `-p`；`doctor` 默认探全部 profile。没有统一查询语言：每种 kind 用它自己的语句。

## 安装

在 `devkit-cli` 根目录：

```bash
just build dsn-cli
just install dsn-cli
```

安装到 `~/.local/bin/dsn-cli`，Fish 补全写到 `~/.config/fish/completions/dsn-cli.fish`。

Console 额外依赖 PATH 上的官方客户端：

| kind | Console |
| --- | --- |
| `mysql` / `doris` | `mysql` |
| `redis` | `redis-cli` |
| `mongodb` | `mongosh` |
| `elasticsearch` | 无，只能 `query` |

## 配置

路径：`~/.config/dsn-cli/config.json`

允许 JSONC（注释、尾逗号）。每次执行读盘。

```jsonc
{
  "profiles": [
    {
      "name": "buy",
      "kind": "mysql",
      "url": "mysql://readonly@127.0.0.1:3306/buy"
    }
  ]
}
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `name` | 是 | `-p` 用的名字 |
| `kind` | 是 | `mysql` / `doris` / `redis` / `mongodb` / `elasticsearch` |
| `url` | 是 | 标准连接 URL，scheme 必须和 kind 对齐 |
| `access` | 否 | `read`（默认，走 Gate）或 `write`（原样转发，不跑 Gate） |

kind 和 URL scheme 对不上会直接拒绝配置。密码里的特殊字符按 URL 做 percent-encode。

仓库里的 `config.example.json` 可直接拷：

```bash
cp dsn-cli/config.example.json ~/.config/dsn-cli/config.json
```

`just install` 只在配置缺失时从 example 引导，不覆盖已有文件。

### mysql

```jsonc
{
  "name": "buy",
  "kind": "mysql",
  "url": "mysql://readonly@127.0.0.1:3306/buy"
}
```

带密码、写权限：

```jsonc
{
  "name": "buy-rw",
  "kind": "mysql",
  "url": "mysql://app:secret@127.0.0.1:3306/buy",
  "access": "write"
}
```

scheme 只能是 `mysql://`。

### doris

Doris 走 MySQL 协议，FE 查询口默认 `9030`。url 仍是 `mysql://`，kind 必须写成 `doris`（Gate 允许集和 mysql 不同，多一个 `SWITCH`）。

```jsonc
{
  "name": "dw",
  "kind": "doris",
  "url": "mysql://readonly@127.0.0.1:9030/dw"
}
```

### redis

```jsonc
{
  "name": "cache",
  "kind": "redis",
  "url": "redis://127.0.0.1:6379/0"
}
```

带密码、指定 DB：

```jsonc
{
  "name": "cache",
  "kind": "redis",
  "url": "redis://:s3cret@127.0.0.1:6379/2"
}
```

TLS 用 `rediss://`：

```jsonc
{
  "name": "cache-tls",
  "kind": "redis",
  "url": "rediss://:s3cret@127.0.0.1:6380/0"
}
```

### mongodb

```jsonc
{
  "name": "mongo",
  "kind": "mongodb",
  "url": "mongodb://127.0.0.1:27017/app"
}
```

带认证：

```jsonc
{
  "name": "mongo",
  "kind": "mongodb",
  "url": "mongodb://app:secret@127.0.0.1:27017/app"
}
```

Atlas / SRV：

```jsonc
{
  "name": "mongo-atlas",
  "kind": "mongodb",
  "url": "mongodb+srv://app:secret@cluster.mongodb.net/app"
}
```

path 里的库名会传给 `db.command`。没有 path 时用驱动默认库。

### elasticsearch

```jsonc
{
  "name": "es",
  "kind": "elasticsearch",
  "url": "https://127.0.0.1:9200"
}
```

明文、带 basic auth：

```jsonc
{
  "name": "es-local",
  "kind": "elasticsearch",
  "url": "http://elastic:secret@127.0.0.1:9200"
}
```

scheme 只能是 `http://` 或 `https://`。没有 Console。

## 用法

```bash
dsn-cli -p <profile>
dsn-cli -p <profile> query '<stmt>' [--output json|csv|plain] [--timeout S] [--connect-timeout S]
dsn-cli agent -p <profile> query '<stmt>' [--limit N] [--timeout S] [--connect-timeout S]
dsn-cli doctor [-p <profile>] [--timeout S] [--connect-timeout S]
dsn-cli agent doctor [-p <profile>]
dsn-cli completion fish
```

- 省略 audience 就是 `human`。
- TTY 下只写 `-p` 会 exec 官方客户端；`agent`、管道、elasticsearch 都不能进 Console。
- `query` 的语句必须是一个 argv 参数。
- `--timeout` 默认 30s（执行），`--connect-timeout` 默认 3s。
- `--output` 只对人有效：默认 table，`json` 是 NDJSON，还有 `csv` / `plain`。
- `--limit` 只对 agent 有效：默认 1000 行，`0` 表示不截。人的结果不截。
- Gate、`--output`、`--limit` 都不作用于 Console。
- `doctor` 最多 4 路并发探测，单个 profile 挂死不会拖住其余项。

## doctor

```bash
dsn-cli doctor
dsn-cli doctor -p buy
dsn-cli doctor --timeout 5 --connect-timeout 3
dsn-cli agent doctor
```

默认探配置里全部 profile，`-p` 只探一个。最多 4 路并发，deadline 是 `--connect-timeout`（默认 3s）加 `--timeout`（默认 5s）。不走 Gate。全开并行会把握手挤过短超时，单独 `query` 能通的库会被误报 `ETIMEDOUT`。

ping 语句按 kind 固定：

| kind | ping |
| --- | --- |
| `mysql` / `doris` | `SELECT 1` |
| `redis` | `PING` |
| `mongodb` | `{"ping":1}` |
| `elasticsearch` | `GET /` |

人默认 table，列是 `name kind status ms error`。任一失败退出码 `3`，仍打印全表。agent 是 `{ rows, truncated }`。

## 各 kind 的 query

### mysql

```bash
dsn-cli -p buy query 'SELECT id, name FROM orders LIMIT 10'
dsn-cli -p buy query 'SHOW TABLES'
dsn-cli -p buy query 'EXPLAIN SELECT 1'
dsn-cli -p buy query --output json 'SELECT id, name FROM orders LIMIT 3'
```

`access: read` 允许头：`SELECT` / `WITH` / `SHOW` / `EXPLAIN` / `DESC` / `DESCRIBE` / `USE`。

会被拒：`INSERT` / `UPDATE` / `DELETE` / `SET`、`SELECT … FOR UPDATE`、`SELECT … INTO OUTFILE`、多语句。

写库把该 profile 的 `access` 改成 `write`：

```bash
dsn-cli -p buy-rw query 'INSERT INTO t (name) VALUES ("x")'
```

### doris

```bash
dsn-cli -p dw query 'SHOW BACKENDS'
dsn-cli -p dw query 'SWITCH CATALOG hive'
dsn-cli -p dw query 'SELECT COUNT(*) FROM dw.orders'
```

相对 mysql，read 额外允许 `SWITCH`。`LOAD` / `EXPORT` / `INSERT` 在 read 下仍拒绝。

### redis

空白 + 引号切参数，第一个 token 是命令头。

```bash
dsn-cli -p cache query 'PING'
dsn-cli -p cache query 'GET user:1'
dsn-cli -p cache query 'HGETALL session:abc'
dsn-cli -p cache query 'LRANGE queue 0 20'
dsn-cli -p cache query 'SCAN 0 MATCH user:* COUNT 100'
```

`access: read` 允许头：`GET` `MGET` `HGET` `HGETALL` `LRANGE` `SMEMBERS` `ZRANGE` `KEYS` `SCAN` `TYPE` `TTL` `EXISTS` `INFO` `PING` `LLEN` `SCARD` `ZCARD` `HLEN` `STRLEN` `DBSIZE`。

`SET` / `DEL` / `EVAL` 在 read 下拒绝。

### mongodb

一条语句就是一个 `runCommand` JSON 对象，Gate 看第一个 key。不是 mongosh JS。

```bash
dsn-cli -p mongo query '{"find":"orders","filter":{"status":"paid"},"limit":10}'
dsn-cli -p mongo query '{"aggregate":"orders","pipeline":[{"$match":{"status":"paid"}},{"$limit":10}],"cursor":{}}'
dsn-cli -p mongo query '{"count":"orders","query":{"status":"paid"}}'
dsn-cli -p mongo query '{"listCollections":1}'
dsn-cli -p mongo query '{"ping":1}'
```

`access: read` 允许头：`find` `aggregate` `count` `distinct` `listCollections` `listIndexes` `listDatabases` `collStats` `dbStats` `explain` `ping` `hello` `getMore` `countDocuments` `estimatedDocumentCount`。

`insert` / `update` / `delete` 在 read 下拒绝。

### elasticsearch

一行 REST：`METHOD /path`，可选 JSON body 跟在同一参数里。read 只放行 `GET` / `HEAD`。`POST /_search` 也会被拒，因为方法粒度拦写入。

```bash
dsn-cli -p es query 'GET /_cluster/health'
dsn-cli -p es query 'HEAD /orders'
dsn-cli -p es query 'GET /orders/_search {"query":{"match_all":{}},"size":10}'
dsn-cli -p es query 'GET /orders/_doc/1'
```

写请求需要 `access: write`：

```bash
dsn-cli -p es-rw query 'POST /orders/_search {"query":{"match_all":{}}}'
dsn-cli -p es-rw query 'POST /orders/_bulk {}'
```

## agent

```bash
dsn-cli agent -p buy query 'SELECT id, name FROM orders'
```

stdout 永远是一个 JSON 对象，忽略 `--output`：

```json
{
  "rows": [{ "id": 1, "name": "a" }],
  "truncated": false
}
```

默认截到 1000 行并标 `truncated`。`--limit 0` 关闭截断。不会改写用户语句（不会偷偷加 SQL `LIMIT`）。

agent 进不了 Console。

## 退出码

| 码 | 含义 |
| --- | --- |
| `0` | 成功，空结果也是成功 |
| `1` | 用法 / 配置错误 |
| `2` | Gate 拒绝 |
| `3` | 驱动 / 服务端错误；`doctor` 任一 profile 连不上 |

Console 的退出码是官方客户端自己的，不翻译。

## 领域名词

完整定义见 [CONTEXT.md](./CONTEXT.md)。决策见 [docs/adr](./docs/adr)。

## License

GPL-2.0-only.
