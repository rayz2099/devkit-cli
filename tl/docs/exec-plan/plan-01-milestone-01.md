# plan-01-milestone-01

## 目标

实现 `tl md en2zh|zh2en` 首版，支持文件路径和 `stdin` 输入，成功时仅向 `stdout` 输出翻译后的 Markdown，并在交互式终端将进度条输出到 `stderr`。

## 约束

- 使用 `goldmark` 解析 Markdown AST
- 保留 Markdown 结构，仅翻译自然语言节点
- Provider 首版采用 OpenAI-compatible 单一抽象
- Markdown 文本单元采用批量分组请求
- 单文件不超过 500 行

## 里程碑拆解

1. 搭建 Go 模块、`justfile` 和 CLI 基础骨架
2. 实现配置加载、OpenAI-compatible provider 抽象
3. 实现 Markdown AST 文本单元提取、批量翻译、回填
4. 实现 `stderr` 进度条和 `stdout` 输出契约
5. 完成测试、格式化和文档同步
