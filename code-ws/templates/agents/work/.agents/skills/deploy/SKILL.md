---
name: deploy
description: Use for the deploy phase of a cross-repo workspace task after project sub-tasks are complete. Records optional test-environment deployment, build tracking, smoke validation, manual validation, or failure notes in tasks/{task-id}/deploy.md and updates tasks/task.md.
---

# 任务 deploy 阶段

## 目标

`deploy` 阶段负责部署到测试环境并记录验证. 这个阶段当前是可选的, 可能由用户手动验证.

部署和验证优先使用 `prism-cli`. 该 CLI 位于 `$ROOT_REPO/devkit-cli/prism-cli`, 默认配置文件是 `~/.config/prism-cli/config.json`.

## 门禁

开始前必须确认:

- 已读取 `$ROOT_REPO/spec/context.md`
- 已读取 `tasks/task.md`
- `tasks/task.md` 中该 task 当前状态是 `deploy`, 或用户明确要求部署或记录验证
- `tasks/task.md` 中该 task 没有未处理的必做项, 或用户明确接受跳过
- 用户明确要求部署或记录验证
- 每个已执行 project 都有 `tasks/{task-id}/{project}.md` 的 `Result`

如果用户说这一步先不做, 记录跳过, 不要部署.

## prism-cli

`prism-cli` 用于查询构建、创建或复用 Prism 测试环境、更新部署列表和执行 HTTP smoke check. Agent 不需要阅读 prism-cli 源码, 优先通过 help 获取当前命令格式.

### 命令格式

本地 fish 里执行:

```bash
prism-cli <resource> <action> [options]
```

如果 PATH 中没有 `prism-cli`, 在 workspace 内用源码执行:

```bash
bun run devkit-cli/prism-cli/src/main.ts <resource> <action> [options]
```

执行部署前先看 help:

```bash
prism-cli --help
prism-cli build --help
prism-cli env --help
prism-cli health --help
```

源码方式:

```bash
bun run devkit-cli/prism-cli/src/main.ts --help
bun run devkit-cli/prism-cli/src/main.ts build --help
bun run devkit-cli/prism-cli/src/main.ts env --help
bun run devkit-cli/prism-cli/src/main.ts health --help
```

全局参数:

```text
--mode agent|human
--output json|table
--config <path>
```

默认配置文件是 `~/.config/prism-cli/config.json`. 禁止直接读取或打印配置文件内容, 因为其中包含 token.

### 构建检查

部署前先确认每个 project 的构建已经成功:

```bash
prism-cli build check \
  --projects shared-lib,app-api,app-gw \
  --branch feature/xxx \
  --hashes '{"app-api":"abc123"}' \
  --timeout 600 \
  --poll 30
```

规则:

- `--projects` 是逗号分隔的 project 名.
- `--branch` 是目标分支.
- `--hashes` 是可选的 project 到 commit hash 映射, 用来避免部署旧构建.
- 结果不是成功时, 停止部署并记录到 `deploy.md`.

只查当前状态、不等待:

```bash
prism-cli build check \
  --projects app-api \
  --branch feature/xxx \
  --no-wait
```

### 创建或复用环境

创建新环境必须传 `--deployments`:

```bash
prism-cli env deploy \
  --env 034 \
  --name task-id \
  --deployments '[{"name":"app-api","buildNo":"12345","branch":"feature/xxx"}]' \
  --timeout 300 \
  --poll 10
```

复用已有环境:

```bash
prism-cli env deploy \
  --env-code 034 \
  --deployments '[{"name":"app-api","buildNo":"12345","branch":"feature/xxx"}]'
```

或按 env id 复用:

```bash
prism-cli env deploy \
  --env-id ENV_ID \
  --deployments '[{"name":"app-api","buildNo":"12345","branch":"feature/xxx"}]'
```

规则:

- `--env-id` 和 `--env-code` 不能同时使用.
- 新环境没有 `--deployments` 会失败.
- `--deployments` 格式以 `prism-cli env --help` 为准: `[{ "name": "...", "buildNo": "...", "branch": "..." }]`.
- deploy 失败或超时时, 停止并记录失败.

### 查询环境

```bash
prism-cli env get --env-code 034
prism-cli env get --env-id ENV_ID
```

必须把输出中的环境标识、状态和 deployment 信息记录到 `deploy.md`.

### 更新环境部署列表

更新部署列表:

```bash
prism-cli env update \
  --env-id ENV_ID \
  --deployments '[{"name":"app-api","buildNo":"12345","branch":"feature/xxx"}]'
```

需要真正部署并等待时, 优先使用 `env deploy`.

### HTTP smoke check

环境 RUNNING 后, 对用户指定或 `deploy.md` 中计划的 URL 做 smoke check:

```bash
prism-cli health check \
  --targets https://example.com/health,https://example.com/api/ping \
  --timeout 120 \
  --interval 2
```

规则:

- 未提供 target 时, 不要臆造 URL, 记录为未执行并等待用户补充.
- smoke check 失败不能写成验证通过.

## 产物

写入或更新:

```text
tasks/{task-id}/deploy.md
tasks/task.md
```

内容包括:

- repo / branch / build number
- 测试环境
- prism-cli 命令和关键输出
- 部署动作
- 验证范围
- 用户手动验证记录
- 自动验证结果
- 失败原因和后续动作

## 验证规则

验证可以是:

- 用户手动验证
- 编译结果
- 单元测试
- HTTP smoke test
- RPC 直测
- 测试环境走查

验证不是强制自动化. 用户手动验证时, Agent 只负责记录结论和证据.

## deploy.md 推荐结构

```markdown
# Deploy

## Input

- task:
- branch:
- projects:

## Build Check

- command:
- result:

## Environment

- envId:
- env:
- stateLabel:
- deployments:

## Deploy Action

- command:
- result:

## Smoke Check

- command:
- result:

## Manual Validation

- owner:
- result:
- notes:

## Final Status

- status: completed | skipped | failed | blocked
- reason:
```

## 完成条件

可以把 `tasks/task.md` 中该 task 更新为 `completed` 的条件:

- 用户确认验证完成, 或明确接受跳过验证
- `deploy.md` 记录了最终状态
- `tasks/task.md` 中该 task 没有未处理阻塞项
- `tasks/task.md` 中该 task 已更新为 `completed`

## 禁止事项

- 不默认部署.
- 不默认要求自动验证.
- 不把未验证说成已验证.
- 部署失败时不直接忽略, 必须记录失败现象和下一步.
- 不直接读取、打印或复制 prism-cli 配置文件内容.
- 不修改 `spec/` 中的人类 PRD 或补充文档.
