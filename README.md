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
| `grafana-cli` | Read and update Grafana dashboards by profile |
| `har-cli` | Analyze HAR files and extract request URIs |
| `issue-cli` | Personal Issue Tracker client: work items, views, activities, knowledge, relations, templates and trash |
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
- The CLIs are the Go binaries under `cmd/`. Old TypeScript sources are in `archive/`
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

`check.sh` runs `gofmt`, `go vet`, `go test`, and a cgo-free build of `cmd/`. `acceptance.sh` builds into a temp directory, runs `--help` and `doctor` without touching a real config directory, and checks that `list` keeps `tl` and omits `alist-cli`.

Each CLI has its own `doctor`. It checks the local config and exits non-zero when a check fails. It does not create the config file.

Configs stay read-only. Create them yourself when a module needs them; install will not:

- `~/.config/code-ws/config.json` and `project.yml`
- `~/.config/olly-cli/config.json`
- `~/.config/grafana-cli/config.json`
- `~/.config/issue-cli/config.json`（示例见 [issue-cli](#issue-cli)）
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
| `grafana-cli` | 按 profile 读取或更新 Grafana 看板 |
| `har-cli` | 分析 HAR 并提取请求 URI |
| `issue-cli` | 单 URL/Token 的 Issue Tracker 客户端：个人视图、工作项、活动、知识、关联、模板与回收站 |
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
- 命令是 `cmd/` 下的 Go 二进制。旧的 TypeScript 实现在 `archive/`
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
- `~/.config/grafana-cli/config.json`
- `~/.config/issue-cli/config.json`（示例见 [issue-cli](#issue-cli)）
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

## issue-cli

Issue Tracker 的个人客户端，不支持 profile；Token 自带用户身份及绑定空间。只使用后端 Token 白名单内的业务 API，不提供任意 HTTP 请求，也不提供账号、Token、成员、角色或飞书管理。

### 配置与本地检查

用户自行创建 `~/.config/issue-cli/config.json`（遵循 `XDG_CONFIG_HOME`，可用 `--config FILE` 指定）。示例中均为占位符，实际 Token 只存放在你自己的配置文件，不应提交版本库；建议权限 `0600`。URL 为服务根地址，不含 `/api`、用户名密码、查询或片段。

```json
{
  "url": "https://issue.example.invalid",
  "token": "REPLACE_WITH_PERSONAL_API_TOKEN"
}
```

以下命令在**本机**执行，逐行命令兼容 fish；需要 Go 的命令使用 mise。`doctor` 只检查本地配置，不联网、不创建文件。

```bash
mise exec -- go run ./cmd/issue-cli --help
mise exec -- go run ./cmd/issue-cli --config /path/to/config.json doctor
just build issue-cli
just install issue-cli
issue-cli me list
issue-cli me groups
issue-cli me unfinished
issue-cli me doing
issue-cli me due --days 3
issue-cli me overdue
issue-cli me done --audience agent
```

`me` 返回平铺节点，默认严格限制 `ownerId` 为当前用户；只有显式 `--owner ID` 才看其他负责人。`unfinished` 排除 COMPLETED/CANCELLED（包括被祖先暂停跟踪的未结束项）；`doing` 为仍在跟踪的 IN_PROGRESS；`done` 为 COMPLETED。`due` 按空间时区取今天到第 N 天（含边界）的仍在跟踪项，也可能包含今天已逾期项；`overdue` 严格使用后端计算的 OVERDUE。个人视图和 `items subtree` 使用完整 snapshot，不受搜索根分页或根状态筛选影响；它们会获取空间全部未删项，但只输出所选范围。

### fish 补全

本机 fish 执行一次，后续新会话自动加载（安装 CLI 不会自动修改 shell 配置）：

```fish
issue-cli completion fish > ~/.config/fish/completions/issue-cli.fish
source ~/.config/fish/completions/issue-cli.fish
```

补全包括资源、子命令、flags 和 `--audience human/agent`，不读取凭据、不联网。`me list` 是个人未完成列表，`me groups` 按顶层事项分组；human 展示简洁表格，agent 保留结构化数据。标记完成使用 `issue-cli items done ID`，自动读取当前版本；冲突不重试，遇到未结束子项需显式确认。

### 资源命令

| 资源 | 命令 | 说明 |
| --- | --- | --- |
| `items` | `list/get/children/subtree/preview/create/update/move/delete/transition/done` | 工作项、结构、子树及流程状态 |
| `dashboard` | `my-work/owned/issues` | 后端定义的我的工作、我负责和异常树（可包含协作者或其他负责人节点） |
| `activities` | `list/feed/create/update/revisions` | 时间线、评论、进展、系统历史、编辑修订 |
| `knowledge` | `get/update` | 资料链接、结论、置顶；action 为 save/pin/unpin |
| `relations` | `list/create/update/delete` | 关联 CRUD；update/delete 参数为事项 ID、关联 ID |
| `successors` | `create` | 创建后续事项 |
| `templates` | `list/save/update/preview/instantiate` | 模板保存、启停、排期预览及实例化 |
| `trash` | `list/get/restore` | 按删除批次读取、恢复 |
| `workspace` | `snapshot/workflow/workgroups/dictionary` | 全量空间元数据、流程发布、分工目录、category/tag 字典 |

ID 作为位置参数；每条命令的准确参数见 `issue-cli RESOURCE COMMAND --help`。除 `items done` 快捷操作外，所有非 GET 命令（包括模板 preview）都要求 `--body-file FILE`，文件内容为**完整后端请求对象**，不展开为大量 flags。字段由后端严格验证；客户端额外校验并发版本、删除与后代终止确认。模板更新为 `templates save` + body.id；模板启停为 `templates update ID`。空间流程基于 sourceId 发布新版本，而不是覆盖流程 ID。

```bash
issue-cli items list --query 'q=发布' --query 'limit=50' --query 'offset=0' --audience agent
issue-cli items get ITEM_ID
issue-cli items subtree ITEM_ID
issue-cli activities list ITEM_ID --query 'scope=SUBTREE' --query 'limit=30'
issue-cli activities feed --query 'kind=PROGRESS_UPDATE' --audience agent
issue-cli knowledge get ITEM_ID
issue-cli templates list --query 'all=true'
issue-cli trash get BATCH_ID
issue-cli workspace snapshot --audience agent
```

`--query key=value` 可重复传入不同的 key；仅接受命令帮助列出的后端参数。`items list` / dashboard 的 `items` 是**匹配树的根事项**，`treeItems` 是这些根的**完整树**，都不代表平铺的匹配节点；`status` 仅筛根状态。根分页 offset/limit（limit 最大 100），`nextOffset`、`total` 原样保留，不自动翻页。活动游标同样不转换，下一页将返回的游标按后端格式放回 `--query 'cursor=...'`。

### 写入、版本与显式确认

先读取对象版本，手动准备 JSON 文件，再提交一次。示例 `update.json`：

```json
{"expectedVersion": 12, "changes": {"title": "发布检查"}}
```

ITEM 流转示例 `transition.json`；目标 ID 来自 snapshot 流程元数据，不能假定宏状态名等于 ID：

```json
{
  "expectedVersion": 12,
  "changes": {},
  "transitionTarget": {"type": "STATE", "id": "TARGET_STATE_ID"},
  "expectedTreeVersion": 20,
  "confirmUnfinishedDescendants": true
}
```

非 ITEM 的简单状态通过 `changes.status` 提交。只有显式 `--confirm` 才允许 `confirmUnfinishedDescendants=true`，且必须携带**根事项**的 expectedTreeVersion。普通流转不需要这个确认时，请不要发送该字段。删除子树 `delete.json`：

```json
{"expectedVersion": 12, "expectedTreeVersion": 20}
```

```bash
issue-cli items update ITEM_ID --body-file update.json
issue-cli items transition ITEM_ID --body-file transition.json --confirm
issue-cli items delete ITEM_ID --body-file delete.json --confirm
issue-cli trash restore BATCH_ID --body-file restore.json
```

删除命令在确认后添加后端要求的 `confirmed=true`。restore.json 为 `{"expectedVersion":1}`，此版本属于**删除批次**而非事项。移动需要 `expectedVersion`、根 `expectedTreeVersion` 和 `targetParentId`；子节点创建也应按后端要求携带根 expectedTreeVersion。knowledge 的 expectedVersion 属于知识对象，初始为 0，不是事项版本。活动编辑、关联更新/删除和模板启停使用各自对象版本。分工目录所有 action（含 create）需要目录 expectedVersion；字典/模板有 id、流程有 sourceId 时同样强制版本。

正常结果只写 stdout；默认 human 用字段层级展示完整结果，`--audience agent` 返回后端 JSON；错误只写 stderr，agent 错误格式为 `{"error":{"code":"...","message":"...","status":409,"details":{},"requestId":"..."}}`，本地错误 code 为 CLIENT_ERROR，失败退出 1。服务返回的错误码、版本冲突和追踪号保留，Token 回显会脱敏。不跟随重定向，不自动重试，不生成或发送幂等键；网络/读取失败的写入结果可能未知，应先查询核实，不要盲目重放。配置缺失或不合法直接失败，不自动创建或选择其他配置。

本地验证，不访问线上：

```bash
mise exec -- go test ./internal/issue ./internal/mods ./cmd/issue-cli
mise exec -- go vet ./internal/issue ./cmd/issue-cli
```

范围限制：复杂请求字段由实际后端契约约束，不在客户端复制全部业务校验；不提供交互提示、自动取版本覆盖、自动分页、批量写入和上传附件。快照响应最多 32MiB，超限显式报错；不做降级请求。HTTP 仅用于本机开发，实际服务建议 HTTPS。
