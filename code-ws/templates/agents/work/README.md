# work-01 agents

任务级 workspace 的 `.agents` 模板目录, 面向跨 repo 的 dt 任务开发.

## Workflow

只使用三个阶段:

```text
plan -> code -> deploy
```

- `plan`: 读取 `spec/` 中的人类 PRD 和补充文档, 生成任务总计划和按 project 拆分的 sub-task.
- `code`: 每个 subagent 执行一个 `tasks/{task-id}/{project}.md`, 修改对应 repo, 并更新 `tasks/task.md` 状态.
- `deploy`: 可选阶段, 记录部署、验证、人工验收和失败信息.

## Directory

`spec/` 是输入上下文目录, 默认由人维护. Agent 必须读取, 但不能修改已有 PRD 或补充文档.

Agent 输出统一写入:

```text
tasks
├── task.md
└── {task-id}
    ├── {project}.md
    └── deploy.md
```

`tasks/task.md` 是总账和根索引, `tasks/{task-id}/{project}.md` 是分账. task 完成状态以 `tasks/task.md` 中对应 task 的 checklist 全部完成和每个 project 有执行结果为准.
