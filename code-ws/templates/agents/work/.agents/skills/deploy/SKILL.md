---
name: deploy
description: 部署已完成编译的 workspace 任务. 当 tasks/task.md 处于 deploy, 或用户明确要求部署、环境验证、记录人工验收时使用; 按目标环境选择项目已定义的部署方式并记录证据.
---

# Deploy

## 目标

解析目标环境对应的部署方式, 执行经用户授权的部署和验证, 将全过程记录到 `tasks/{task-id}/deploy.md`.

## 步骤

### 1. 校验交接

读取 `$ROOT_REPO/spec/context.md`、`$ROOT_REPO/project.yml`、`tasks/task.md` 和所有已执行项目的 `Result`.

执行条件:

- task 状态为 `deploy`, 或用户明确要求本次部署.
- 所有必做 project 已完成编译, 且没有未处理阻塞项.
- 部署目标和范围明确.
- `deploy: optional` 已获得用户确认.

用户接受跳过时, 写入 `deploy.md` 并将 task 更新为 `completed`.

完成标准: 部署授权、目标环境、project/branch/version 均可核对.

### 2. 解析环境适配器

按以下优先级寻找当前环境的部署事实:

1. `tasks/{task-id}/deploy.md` 中已确认的命令和目标.
2. `spec/context.md` 指向的部署说明.
3. 各项目的 AGENTS、README、构建脚本或 CI 配置.
4. 已安装部署 CLI 的 `--help`.

环境适配器可能是 Prism、Kubernetes、Terraform、CI/CD pipeline、本地进程或人工部署. 只使用事实来源明确支持当前环境的方式. 当前方式不可用时标记 `blocked`, 将所需信息或权限交给用户.

将选定方式、事实来源、目标环境、执行命令和验证方式先写入 `deploy.md` 的 `Plan`.

完成标准: 唯一选定环境适配器, 且命令参数可从事实来源逐项解释.

### 3. 执行部署

按已记录计划执行:

- 确认待部署构建与 branch/commit/version 匹配.
- 创建或复用目标环境.
- 部署计划内 projects.
- 每个有副作用的阶段失败后停止后续动作, 保存关键输出和可恢复位置.

凭据只通过部署工具的既有配置使用, 输出中保留脱敏证据.

完成标准: 每个目标 project 都有明确的部署成功证据, 或 deploy 状态为 `failed`/`blocked` 且失败点完整.

### 4. 执行约定验证

只执行总账或 `deploy.md` 已约定的验证:

- 用户手工验收: 记录执行人、结论和证据.
- HTTP/RPC smoke: 使用文档或用户给出的 target.
- 自动测试: 仅在用户明确要求时执行.
- 未约定 target 或验收方式: 记录 `not run`, 等待用户补充或接受跳过.

完成标准: 每项约定验证均有 `passed`、`failed`、`skipped` 或 `not run` 状态, 且证据与结论一致.

### 5. 收口

更新 `deploy.md` 和 `tasks/task.md`:

- 部署与约定验证完成, 或用户明确接受跳过: `completed`.
- 部署命令执行失败: `failed`, task 保持 `deploy`.
- 缺少环境、权限或关键输入: `blocked`.

完成标准: 最终状态、环境标识、部署版本、验证范围和后续动作均已记录.

## deploy.md 格式

```markdown
# Deploy

## Input

- task:
- projects:
- branch/commit/version:
- target:

## Plan

- adapter:
- source:
- commands:
- validation:

## Execution

- environment:
- deployments:
- result:

## Validation

- command:
- result:

## Manual Validation

- owner:
- result:
- notes:

## Final Status

- status: completed | skipped | failed | blocked
- reason:
- next:
```
