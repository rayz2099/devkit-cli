# devkit-cli 项目索引

`devkit-cli` 用于集中维护本地 CLI 工具。根目录统一控制面:

```bash
just clean
just build
just install
just build code-ws
```

## 项目

| 项目 | 功能 |
| --- | --- |
| `code-ws` | VS Code workspace 初始化 CLI, 基于配置创建 worktree, 生成 `.code-workspace` 和 `.agents`. |
| `olly-cli` | Prometheus / Uptrace / Graylog 查询 CLI. |
| `tl` | Go 实现的命令行翻译工具. |
| `har-cli` | HAR 文件分析 CLI. |
| `alist-cli` | Alist 客户端: ls / put / mkdir / 目录 sync. |
| `dsn-cli` | 多数据源查询 CLI: mysql / doris / postgres / redis / mongodb / elasticsearch / kafka. |
