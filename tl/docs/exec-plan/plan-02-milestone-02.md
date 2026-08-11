# plan-02-milestone-02

## 目标

实现根命令 `tl en2zh [text]` 与 `tl zh2en [text]`，支持单个位置参数和 `stdin` 输入，成功时仅向 `stdout` 输出翻译结果。

## 约束

- 沿用现有 OpenAI-compatible provider 抽象
- plain-text 根命令首版不输出进度条
- 位置参数模式仅允许单个文本参数，多参数直接报 usage 错误
- `md` 子命令现有行为不回归
- 单文件不超过 500 行

## 里程碑拆解

1. 更新执行计划与 todo 文档索引
2. 为 root 命令、help、completion 先补失败测试
3. 新增 plain-text 应用服务并接入 CLI 分发
4. 更新帮助文案与 fish completion
5. 完成回归测试与验证
