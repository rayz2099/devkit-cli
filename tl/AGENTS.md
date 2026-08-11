# TL
这是一个命令行翻译工具, 支持翻译英译中, 和中译英

## Key Point

- 遵循 `ddd` 架构.
- 单个文件不允许超过 500 行, 否则拆分功能.
- 项目技术栈使用 `golang`, 并使用`justfile` 的 `just`进行构建
- 所有的 plan 都要参考并沉淀到 [plan](./docs/exec-plan/plan.md) 中, 以 plan-x-milestone-y.md 命名
- 功能描述参考 [core](./docs/core.md)
- 处理任务一定要拆解成更小的任务一步一步完成, 所有的 todo 放在 [todo](./docs/exec-task/ todolist.md) 中, 任务以 todo-01.md, todo-02.md ... 命名
- bugfix和优化部分不必单独的plan, 放在 todo 中即可

## Dos
- 你可以使用 context7 查阅对应的 API 文档.

## Tests

- `$HOME/.config/tl/config.json`已存在, 直接使用这个进行测试.

## Don'ts
- 不允许使用废弃的 API
- 不允许使用 brew, apt, dnf 等包管理工具, 可以把命令贴出来由开发者手动执行
- 不允许操作 git add, commit, push
