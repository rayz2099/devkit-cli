# todo-06

## 目标

为 `tl md` 增加 `--fast`, 按文档大小合并翻译请求, 默认逐 unit 行为不变.

## 子任务

1. 按文档体积计算 pack budget, 只合并 unit 索引
2. 多 unit 用 marker 打包/拆包, 回填仍走原 AST 偏移
3. CLI 只剥离 `--fast`, 更新 help 与 fish completion
4. 补默认路径与 fast 路径测试
