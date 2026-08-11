---
name: code
description: Use for the code phase of a cross-repo workspace task. Each agent works on exactly one project sub-task from tasks/{task-id}/{project}.md, updates only that project section in tasks/task.md, and records the result.
---

# 任务 code 阶段

## 目标

`code` 阶段按 project 拆分执行. 每个 subagent 只处理一个 `tasks/{task-id}/{project}.md`, 修改对应 repo, 并回写该 project 的执行状态.

## 门禁

开始前必须确认:

- 已读取 `$ROOT_REPO/spec/context.md`
- 已读取 `tasks/task.md`
- `tasks/task.md` 中该 task 当前状态是 `coding`, 或用户明确要求进入 code 阶段
- 已读取且只领取一个 `tasks/{task-id}/{project}.md`
- 已检查该 project 对应 repo 的当前 diff

条件不满足时, 不要修改代码.

## 适用场景

- subagent 执行单个 project sub-task.
- 用户改了 DTO, Agent 修当前 project 内调用方.
- 用户改了后端方法签名, Agent 修当前 project 编译错误.
- 用户手动实现业务逻辑, Agent 整理当前 project 的 import、配置和边缘调用.

## 工作规则

- 一个 subagent 只能负责一个 project.
- 只能修改该 project 对应 repo.
- 可以读取其他 project 文档, 但不能修改其他 project 的代码或状态.
- 只能更新 `tasks/task.md` 中自己 task 和 project 下的 checkbox.
- 必须更新自己的 `tasks/{task-id}/{project}.md` 的 `Result`.
- 用户改动视为权威.
- 修改前先看 diff.
- 不 revert 用户改动, 除非用户明确要求.
- 如果用户改动突破原计划, 只更新自己 project 文档并说明偏差.
- 如果发现依赖 project 缺失、状态不对或计划错误, 标记当前 project 为 `blocked`, 不跨界修.

## 执行顺序

默认按 `tasks/task.md` 和 project 依赖顺序执行:

```text
shared-lib -> 后端服务 -> API/BFF/网关接入 -> 前端界面
```

共享库如 `shared-lib`、`dt-base` 必须先完成, 下游 project 才能开始.

## 产物

更新:

```text
tasks/{task-id}/{project}.md
tasks/task.md
```

记录:

- 完成了哪些 checklist
- 实际修改了哪些文件
- 与计划不同的地方
- 编译/测试结果
- 遇到的问题
- 是否 blocked

## 质量门禁

- 修改代码后默认执行 `mcc`.
- 修改 `shared-lib` 后需要使用 `mvn_version` 升级为 `SNAPSHOT` 版本, 然后执行 `mcd`.
- 不要求 e2e 测试, 除非用户明确要求.
- 不能把未执行的验证写成已通过.

## 完成条件

单个 project 完成条件:

- `tasks/{task-id}/{project}.md` 的 `Checklist` 已全部完成或明确标记跳过原因
- `Result` 已记录修改文件和编译结果
- `tasks/task.md` 中该 project 的 checkbox 已同步更新

task 完成条件由主 Agent 判断:

- 所有 project section 都完成
- 每个 project 都有 `Result`
- 没有未处理 blocked 项

## 禁止事项

- 不把用户代码重写成 Agent 自己的方案.
- 不扩大修改范围.
- 不用格式化或重构掩盖实际改动.
- 不修改 `spec/` 中的人类 PRD 或补充文档.
- 不修改其他 project 的状态.
