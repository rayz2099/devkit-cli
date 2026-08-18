# devkit-cli

[English](#english) | [中文](#zhongwen)

<a id="english"></a>
## English

`devkit-cli` is a monorepo of personal developer CLIs with a root control plane.

### Tools

| Tool | Description |
| --- | --- |
| `code-ws` | Create VS Code workspaces from multi-repo profiles and worktrees |
| `olly-cli` | Query Prometheus / Uptrace / Graylog |
| `mysql-cli` | Profile-based MySQL helper with Fish completion |
| `har-cli` | Analyze HAR files and extract request URIs |
| `tl` | Go CLI for EN/ZH text and Markdown (`tl md --fast`) |
| `jenkins-cli` | Profile-based Jenkins client with human/agent output and Fish completion |
| `codeup-cli` | Profile-based Codeup client: repos, git push, change requests, webhook list |

### Requirements

- [Bun](https://bun.sh)
- [just](https://github.com/casey/just)
- Go (for `tl`)
- `mysql` client (for `mysql-cli`)

### Quick start

```bash
just list
just build
just install
just build code-ws
```

Configs live in XDG paths:

- `~/.config/code-ws/config.json` + `project.yml`
- `~/.config/olly-cli/config.json`
- `~/.config/mysql-cli/config.json`
- `~/.config/tl/config.json`
- `~/.config/jenkins-cli/config.json`
- `~/.config/codeup-cli/config.json`

Copy examples first if needed:

```bash
cp code-ws/config.example.json ~/.config/code-ws/config.json
cp code-ws/project.example.yml ~/.config/code-ws/project.yml
cp olly-cli/config.example.json ~/.config/olly-cli/config.json
cp mysql-cli/config.example.json ~/.config/mysql-cli/config.json
cp tl/config.example.json ~/.config/tl/config.json
cp jenkins-cli/config.example.json ~/.config/jenkins-cli/config.json
cp codeup-cli/config.example.json ~/.config/codeup-cli/config.json
```

`just install` bootstraps missing XDG configs from `*.example.*` and never overwrites existing files.

### License

GPL-2.0-only. See [LICENSE](./LICENSE).

---

<a id="zhongwen"></a>
## 中文

`devkit-cli` 是个人开发者 CLI 工具 monorepo, 根目录提供统一控制面.

### 工具

| 工具 | 说明 |
| --- | --- |
| `code-ws` | 基于 profile/worktree 初始化 VS Code workspace |
| `olly-cli` | 查询 Prometheus / Uptrace / Graylog |
| `mysql-cli` | 基于 profile 的 MySQL 辅助工具, 含 Fish completion |
| `har-cli` | 分析 HAR 并提取请求 URI |
| `tl` | Go 中英翻译, 支持纯文本和 Markdown (`tl md --fast`) |
| `jenkins-cli` | 基于 profile 的 Jenkins 客户端, 含 human/agent 输出和 Fish completion |
| `codeup-cli` | 基于 profile 的 Codeup 客户端: 仓库, git push, Change Request, webhook 只读 |

### 依赖

- Bun
- just
- Go (`tl`)
- `mysql` 客户端 (`mysql-cli`)

### 快速开始

```bash
just list
just build
just install
just build code-ws
```

配置统一放在 XDG:

- `~/.config/code-ws/config.json` + `project.yml`
- `~/.config/olly-cli/config.json`
- `~/.config/mysql-cli/config.json`
- `~/.config/tl/config.json`
- `~/.config/jenkins-cli/config.json`
- `~/.config/codeup-cli/config.json`

`just install` 仅在配置缺失时从 `*.example.*` 引导, 不覆盖已有文件.

### 许可证

GPL-2.0-only, 见 [LICENSE](./LICENSE).
