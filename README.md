# devkit-cli

[English](#english) | [中文](#zhongwen)

<a id="english"></a>
## English

`devkit-cli` is a monorepo of personal developer CLIs. The root control plane discovers modules and dispatches `list`, `clean`, `build`, and `install`.

### Tools

| Tool | Description |
| --- | --- |
| `code-ws` | Create VS Code workspaces from multi-repo profiles and worktrees |
| `olly-cli` | Query Prometheus / Uptrace / Graylog / Grafana |
| `har-cli` | Analyze HAR files and extract request URIs |
| `tl` | EN/ZH text and Markdown translation (`tl md --fast`) |
| `jenkins-cli` | Profile-based Jenkins client with human/agent output and Fish completion |
| `codeup-cli` | Profile-based Codeup client: repos, git push, change requests, webhook list |
| `dsn-cli` | Profile-based query CLI for mysql/doris/postgres/redis/mongodb/elasticsearch/kafka |
| `td-cli` | Read-only ThinkingData / TE client: session login, metadata, event and property stats |
| `nacos-cli` | Nacos config read/write/delete, service register/deregister, instance query |
| `rclone-cli` | Profile-based rclone transfers: push, pull, check, get, download, ls, runs |
| `gh-image-cli` | Register upstream images or Dockerfiles and build them through GitHub Actions |

`alist-cli` is not ported. Its TypeScript sources stay in `archive/`.

### Requirements

- Go via `mise`
- [just](https://github.com/casey/just)
- The CLIs are the Go binaries under `go/cmd`. Old TypeScript sources are in `archive/`
- `mysql` client (for `dsn-cli` Console)
- `redis-cli` / `mongosh` / `psql` (for `dsn-cli` Console)

### Quick start

From the repo root:

```bash
just list
just build
just install
just build code-ws
```

`just list`, `just clean`, `just build`, and `just install` run the Go `devkit-cli`. `build` compiles the selected command with `CGO_ENABLED=0`. `install` copies that binary to `$HOME/.local/bin` or `DEST`. They do not copy examples into the config directory and do not write Fish completions. `tl` stays its own Go module and is forwarded to `tl/justfile`.

Check and acceptance:

```bash
bash scripts/go/check.sh
bash scripts/go/acceptance.sh
```

`check.sh` runs `gofmt`, `go vet`, `go test`, and a cgo-free build of `go/cmd`. `acceptance.sh` builds into a temp directory, runs `--help` and `doctor` without touching a real config directory, and checks that `list` keeps `tl` and omits `alist-cli`.

Each CLI has its own `doctor`. It checks the local config and exits non-zero when a check fails. It does not create the config file.

Configs stay read-only. Create them yourself when a module needs them; install will not:

- `~/.config/code-ws/config.json` and `project.yml`
- `~/.config/olly-cli/config.json`
- `~/.config/tl/config.json`
- `~/.config/jenkins-cli/config.json`
- `~/.config/codeup-cli/config.json`
- `~/.config/dsn-cli/config.json`
- `~/.config/td-cli/config.json`
- `~/.config/nacos-cli/config.json`
- `~/.config/rclone-cli/config.json`
- `~/.config/gh-image-cli/config.json`

Examples live with the old TypeScript sources in `archive/<tool>/`. Copy one yourself before the first run.

### License

GPL-2.0-only. See [LICENSE](./LICENSE).

---

<a id="zhongwen"></a>
## 中文

`devkit-cli` 是个人开发者 CLI 工具 monorepo。根控制面只做 `list`、`clean`、`build`、`install`。

### 工具

| 工具 | 说明 |
| --- | --- |
| `code-ws` | 按 profile 和 worktree 生成 VS Code workspace |
| `olly-cli` | 查询 Prometheus / Uptrace / Graylog / Grafana |
| `har-cli` | 分析 HAR 并提取请求 URI |
| `tl` | 中英翻译，支持纯文本和 Markdown (`tl md --fast`) |
| `jenkins-cli` | 按 profile 的 Jenkins 客户端，含 human/agent 输出和 Fish completion |
| `codeup-cli` | 按 profile 的 Codeup 客户端：仓库、git push、Change Request、webhook 只读 |
| `dsn-cli` | 按 profile 查询 mysql/doris/postgres/redis/mongodb/elasticsearch/kafka |
| `td-cli` | ThinkingData / TE 只读客户端：会话登录、元数据、事件和属性统计 |
| `nacos-cli` | Nacos 配置读写删、服务注册与注销、实例查询 |
| `rclone-cli` | 按 profile 的 rclone 传输：push、pull、check、get、download、ls、runs |
| `gh-image-cli` | 登记上游镜像或 Dockerfile，经 GitHub Actions 构建 |

`alist-cli` 这次不移植，TypeScript 源码留在 `archive/`。

### 依赖

- Go，由 `mise` 管理
- just
- 命令是 `go/cmd` 下的 Go 二进制。旧的 TypeScript 实现在 `archive/`
- `mysql` 客户端（`dsn-cli` Console）
- `redis-cli` / `mongosh` / `psql`（`dsn-cli` Console）

### 快速开始

在仓库根目录：

```bash
just list
just build
just install
just build code-ws
```

`just` 走 Go 的 `devkit-cli`。`build` 用 `CGO_ENABLED=0` 编译。`install` 把二进制放到 `$HOME/.local/bin` 或 `DEST`，不写配置，也不装 Fish 补全。`tl` 仍是独立的 Go 工程，转给 `tl/justfile`。

检查和验收：

```bash
bash scripts/go/check.sh
bash scripts/go/acceptance.sh
```

每个 CLI 自己有 `doctor`，只检查本地配置，失败时非 0 退出，不会创建配置文件。

配置只读，需要时自己建：

- `~/.config/code-ws/config.json` 和 `project.yml`
- `~/.config/olly-cli/config.json`
- `~/.config/tl/config.json`
- `~/.config/jenkins-cli/config.json`
- `~/.config/codeup-cli/config.json`
- `~/.config/dsn-cli/config.json`
- `~/.config/td-cli/config.json`
- `~/.config/nacos-cli/config.json`
- `~/.config/rclone-cli/config.json`
- `~/.config/gh-image-cli/config.json`

示例文件在 `archive/<tool>/`。第一次跑之前自己拷贝。

### 许可证

GPL-2.0-only，见 [LICENSE](./LICENSE)。
