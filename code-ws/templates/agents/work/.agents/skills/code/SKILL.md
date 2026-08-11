---
name: code
description: 实施 workspace 项目分账. 当 tasks/task.md 处于 coding、用户要求按计划编码, 或需要修复当前项目编译问题时使用; 默认编译验收, 完成后按总账移交 deploy 或收口.
---

# Code

## 目标

一次只实施一个项目分账, 保留用户现有改动, 以项目原生编译命令形成可验证结果.

## 步骤

### 1. 领取一个 project

读取 `$ROOT_REPO/spec/context.md`、`$ROOT_REPO/project.yml`、`tasks/task.md` 和一个未完成的 `tasks/{task-id}/{project}.md`.

领取条件:

- task 状态为 `coding`, 或用户明确要求实施该分账.
- project 已登记在 `project.yml`.
- 所有前置 project 已完成.
- 当前 repo 的 diff 已检查, 用户改动和计划修改可区分.

条件不成立时, 在当前分账记录原因并标记 `blocked`.

完成标准: 已唯一锁定一个 project、一个 repo 和一份 checklist.

### 2. 实施分账

按 checklist 顺序修改当前 repo. 用户现有改动是事实基线; 方案发生偏差时, 在当前分账记录实际路径和原因.

当前 project 之外的依赖缺口记录为 `blocked`, 留给对应 project 分账处理.

完成标准: checklist 中每项均已完成, 或有明确的跳过、阻塞原因; 修改范围仍属于当前 repo.

### 3. 编译验收

执行分账中从项目事实来源解析出的编译命令. 默认验收止于编译:

- 用户明确要求测试时, 再执行分账列出的测试命令.
- 编译命令会隐式运行测试时, 先向用户说明, 再按用户决定执行或改用项目提供的纯编译命令.
- 共享库版本变更遵循项目现有版本策略. 发布或部署动作留给 deploy 阶段.
- 无法解析可靠编译命令时, 记录 `blocked`, 不猜测命令.

完成标准: 编译退出码为 0; 或分账准确记录失败命令、关键错误和阻塞状态. 未执行的验证保持 `not run`.

### 4. 回写结果

更新当前 `tasks/{task-id}/{project}.md` 的 `Result`:

- 修改文件.
- checklist 结果.
- 计划偏差.
- 编译命令、退出结果和关键输出.
- 测试结果, 默认 `not run`.
- 风险或阻塞.

仅同步 `tasks/task.md` 中当前 project 的 checkbox 和摘要状态.

完成标准: 分账保存完整证据, 总账与分账状态一致.

### 5. 阶段交接

当前 project 完成后, 按依赖顺序领取下一个未完成 project. 所有 project 完成且没有阻塞项时:

- `deploy: required`: 将 task 更新为 `deploy`.
- `deploy: optional`: 询问用户是否部署; 选择部署则更新为 `deploy`, 接受跳过则更新为 `completed`.
- `deploy: skip`: 将 task 更新为 `completed`.

状态迁移时向用户汇总各项目编译结果、测试执行情况和下一阶段.

完成标准: task 已进入 `deploy`、`completed` 或 `blocked`, 下一阶段可从账本直接继续.
