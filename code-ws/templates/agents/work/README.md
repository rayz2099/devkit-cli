# work agents

面向跨 repo workspace 的任务模板.

## 阶段

```text
plan -> code -> deploy
                ^ optional
```

- `plan`: 读取 `spec/` 和项目事实, 生成总账与按 project 拆分的可执行分账.
- `code`: 按依赖顺序实施分账, 默认只执行编译验收; 测试需用户明确要求.
- `deploy`: 根据目标环境选择项目已定义的部署方式. 该阶段由计划或用户决定是否执行.

## 目录

```text
spec/
tasks/
├── task.md
└── {task-id}/
    ├── {project}.md
    └── deploy.md
```

- `spec/` 是人维护的只读输入.
- `tasks/task.md` 保存状态、依赖顺序和跨阶段决策.
- `{project}.md` 保存单项目计划与实施结果.
- `deploy.md` 保存环境适配器、部署和验证证据.
