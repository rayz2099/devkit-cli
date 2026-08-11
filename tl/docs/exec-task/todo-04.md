# todo-04

## 目标

修复 OpenAI-compatible provider 在 `gpt-5.4-mini` 下非流式响应正文为空导致的翻译失败。

## 子任务

1. 复现 `tl md en2zh|zh2en` 的真实失败路径
2. 对照验证兼容层在 `stream:true` 下会返回 `delta.content`
3. 为 provider client 增加 SSE 流式解析测试
4. 将 provider client 切换为流式 `chat/completions` 聚合输出
5. 完成回归测试并验证 CLI 实际翻译成功
