---
name: plan
description: 规划跨项目 workspace 任务. 当用户要求分析需求、拆分项目、制定实施计划, 或 code 阶段缺少可执行分账时使用; 产出可直接交给 code 的任务账本.
---

# Plan

## 目标

把 PRD 转换成按依赖排序、可独立实施的项目分账, 并明确编译、测试和部署决策.

## 步骤

### 1. 建立事实基线

读取:

- `$ROOT_REPO/spec/context.md` 及其指向的本次任务文档.
- `$ROOT_REPO/project.yml`.
- 涉及项目的开发说明、构建脚本和当前 diff.
- 已存在的 `tasks/task.md` 和同 task 分账.

事实存在冲突时, 在风险中记录冲突和采用依据. 缺少会改变实现范围的关键事实时, 将 task 标记为 `blocked` 并列出待确认项.

完成标准: 需求来源、允许修改的项目、现有改动和项目约束均已登记.

### 2. 拆分依赖图

每个 project 建立一个 `tasks/{task-id}/{project}.md`. 分账必须只覆盖一个 repo, 并包含:

- 范围和不改范围.
- 前置 project.
- 必读文件.
- 预期修改的模块、接口和数据契约.
- 按顺序执行的 checklist.
- 项目原生编译命令.
- 仅在用户明确要求时写入测试命令和测试验收项.
- 风险、回滚方式和 `Result` 占位.

依赖通常按以下方向排列, 实际顺序以代码依赖为准:

```text
shared-lib -> 后端服务 -> API/BFF/网关 -> 前端 -> deploy
```

完成标准: 每个涉及项目都有一份可单独交给 code 执行的分账, 且所有跨项目依赖均有前置节点.

### 3. 建立总账和交接决策

创建或更新 `tasks/task.md` 中当前 task:

- `status: planning`.
- 需求来源、目标和不改范围.
- 按依赖排序的 projects 和 checklist 摘要.
- `verification: build` 作为默认值; 用户明确要求测试时追加具体类型.
- `deploy: required | optional | skip`.
- 已知部署目标或环境标识; 未知信息保留为待确认项.
- 全局风险和阻塞项.

实际步骤只写在项目分账, 总账不复制完整计划.

完成标准: 总账可以唯一确定 project 执行顺序、验收门禁和 code 完成后的下一状态.

### 4. 移交 code

检查所有分账后, 将 task 状态更新为 `coding`, 并向用户摘要说明执行顺序、默认只编译验收以及部署决策.

以下任一情况保持 `planning` 或标记 `blocked`:

- 关键需求仍会改变实现范围.
- project 不在 `project.yml` 中.
- 项目分账缺失或无法独立执行.
- 编译命令尚未从项目事实来源解析.

完成标准: `tasks/task.md` 为 `coding`, 且 code 无需重新推导范围即可领取首个未完成 project.

## 文件格式

`tasks/task.md` 的 task 条目至少包含:

```markdown
## settings-add-fields

- status: planning
- verification: build
- deploy: optional
- source:
  - spec/prd.md

### Projects

- [ ] shared-lib
- [ ] app-api

### Risks
```

项目分账至少包含:

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

## Verification

- build: `<project-native-command>`
- tests: user-requested only

## Checklist

- [ ] ...

## Result

- modified files:
- build:
- tests:
- risks:
```
