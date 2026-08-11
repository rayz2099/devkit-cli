# har-cli

`har-cli` 是一个基于 Bun 的 HAR 分析命令行工具，目标是从抓包结果中快速提取需要放权的 URI，用于权限白名单迭代和回归验证。

## 功能

- 读取一个或多个由 Chrome DevTools 导出的 HAR 文件
- 支持表达式过滤，例如 `body.status == 1 and http.status == 403`
- 默认输出命中的 HTTP 明文 request/response
- 支持通过字段投影只保留 `path`、`http.status`、`response.headers` 等字段
- 默认去重，减少重复放权项
- 支持生成 Fish shell 补全脚本
- 支持通过 `just` 执行构建和安装

## 环境要求

- Bun 1.3.x
- just 1.14+

## 构建

```bash
just build
```

构建产物输出到 `dist/har-cli`。

## 安装

```bash
just install
```

安装动作会写入：

- `~/.local/bin/har-cli`
- `~/.config/fish/completions/har-cli.fish`

## 用法

```bash
har-cli sample.har
```

自定义过滤条件：

```bash
har-cli sample.har -f 'body.status == 1 and http.status == 403'
```

只保留 path：

```bash
har-cli sample.har -f 'body.status == 1 and http.status == 403' -s path
```

保留自定义字段：

```bash
har-cli sample.har -s path,http.status
```

输出前缀聚合：

```bash
har-cli sample.har -s prefix -p 2
```

输出结构化 JSON：

```bash
har-cli sample.har -o json -s path,http.status
```

## 表达式上下文

过滤表达式可访问以下字段：

- `http.status`
- `http.method`
- `request.url`
- `request.path`
- `response.mimeType`
- `body`
- `uri.url`
- `uri.path`

支持的运算符：

- `==`、`!=`、`>`、`>=`、`<`、`<=`
- `and`、`or`、`not`
- `&&`、`||`、`!`

如果响应体不是合法 JSON，则 `body` 为 `undefined`。

## Help 与 Fish 补全

```bash
har-cli help
har-cli --help
har-cli completions fish
```

Fish 手动安装示例：

```fish
har-cli completions fish > ~/.config/fish/completions/har-cli.fish
```

## 开发命令

```bash
just clean
just build
just install
bun test
bun run typecheck
```

## 许可证

本项目使用 GNU General Public License v3.0 only，详见 [LICENSE](./LICENSE)。
