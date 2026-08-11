# AGENTS.md

## 工作空间

- `ROOT_REPO` 是聚合多个 git worktree 的 workspace 根目录.
- 开始任何阶段前读取 `$ROOT_REPO/spec/context.md` 和 `$ROOT_REPO/project.yml`.
- 代码修改范围仅限 `project.yml` 中登记的项目.
- `spec/` 保存人维护的 PRD 和上下文. Agent 默认只读; 产物统一写入 `tasks/`.
- 项目内已有文档和构建脚本是编译、测试、部署命令的事实来源.

## 阶段状态机

```text
planning -> coding -> deploy -> completed
                    \-> completed
任意阶段 -> blocked
```

- `plan` 负责拆解任务并建立验证、部署决策.
- `code` 逐项目实施, 默认以编译通过为验收门禁.
- `deploy` 仅在计划要求或用户明确要求时执行; 跳过部署的任务可由 `code` 直接收口.
- 测试是显式验收项. 只有用户明确要求时才主动执行.
- 每次状态迁移同时更新 `tasks/task.md`, 保证下一阶段可直接接手.

## 任务账本

- `tasks/task.md` 是任务状态、项目顺序和全局决策的唯一总账.
- `tasks/{task-id}/{project}.md` 是单项目范围、步骤和结果的唯一分账.
- `tasks/{task-id}/deploy.md` 是部署与环境验证记录.
- 实际结果写入对应分账, 总账只保留状态和摘要, 避免重复维护.
