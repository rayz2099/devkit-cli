---
name: plan
description: Use for the plan phase of a cross-repo workspace task. Must read $ROOT_REPO/spec/context.md and the PRD files it references, then update tasks/task.md and create tasks/{task-id}/{project}.md files.
---

# 任务 plan 阶段

## 目标

`plan` 阶段读取 `spec/` 中的人类 PRD 和补充上下文, 把一个 task 拆成多个 project sub-task, 并产出可被 subagent 执行的清单.

## 门禁

开始前必须确认:

- 已读取 `$ROOT_REPO/spec/context.md`
- 已按 `context.md` 读取本次任务需要的 PRD 和补充文档
- 已确认或创建 `tasks/`
- 用户已确认进入 `plan` 阶段

条件不满足时, 停下来说明缺什么.

## 产物

任务根文件:

```text
tasks/task.md
```

每个 project 一个 sub-task:

```text
tasks/{task-id}/{project}.md
```

## tasks/task.md 内容

`tasks/task.md` 是所有 task 的根索引, 每个 task 一段:

- task 背景和目标
- 需求来源文件
- 当前状态: `planning`、`coding`、`deploy`、`completed`、`blocked`
- 涉及 project
- project 依赖顺序
- 按 project 分组的总 checklist
- 不改范围
- 全局风险

推荐结构:

```markdown
# Tasks

## settings-add-fields

- status: planning
- source:
  - spec/prd.md
- branch:
- it-env-no:

### Projects

- shared-lib
- app-api

### Checklist

#### shared-lib

- [ ] 新增字段定义
- [ ] mcc 编译通过

#### app-api

- [ ] 接入 metadata 字段
- [ ] mcc 编译通过
```

## tasks/{task-id}/{project}.md 内容

每个 project 计划必须具体到可执行:

- 只能修改哪个 repo
- 必读上下文文件
- 依赖哪些 project
- 改哪些文件或模块
- 新增/修改哪些字段
- DTO、接口、API、数据库、配置或 UI 变化
- 实现顺序
- 编译或测试命令
- 风险和回滚
- 不改范围
- 执行结果占位

推荐结构:

```markdown
# {project}

## Scope

只修改 `{project}` 对应 repo.

## Depends On

- ...

## Must Read

- spec/context.md
- tasks/task.md

## Plan

- ...

## Checklist

- [ ] ...

## Result

- modified files:
- build:
- risks:
```

## 排序规则

通常按这个顺序:

```text
shared-lib -> 后端服务 -> API/BFF/网关接入 -> 前端界面 -> deploy
```

共享库如 `shared-lib`、`dt-base` 必须排在下游服务之前.

## 完成条件

只有满足以下条件, 才能把 `tasks/task.md` 中该 task 状态更新为 `coding`:

- `tasks/task.md` 已生成或已更新
- 每个涉及 project 都有 `tasks/{task-id}/{project}.md`
- 清单顺序体现依赖关系
- 每个 project sub-task 都可以交给单独 subagent 执行
- 用户确认可以开始 code

## 禁止事项

- 不写业务代码.
- 不修改 `spec/` 中的人类 PRD 或补充文档.
- 不跳过 `tasks/{task-id}/{project}.md` 只写一个泛泛计划.
- 不把 deploy 当成必做项.
