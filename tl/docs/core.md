# Core.md

## 功能

- 和 fish 集成, 支持提示
- 当前里程碑优先实现 `tl md en2zh|zh2en` 与根命令 `tl en2zh|zh2en`
- 支持 2 级命令
  - md: 解析 markdown ast 分段翻译
    - tl md en2zh
    - tl md zh2en
  - tl en2zh
  - tl zh2en
- `md` 子命令首版必须同时支持文件路径输入和 `stdin` 输入
- `md` 子命令首版成功结果仅允许输出到 `stdout`
- `md` 子命令翻译进度条仅允许输出到 `stderr`
- `md` 子命令统一使用 `goldmark` 作为 Markdown AST 库
- `md` 子命令首版采用“保留结构、仅翻译自然语言节点”的策略
- `md` 子命令提取策略优先按标题和 Markdown block 聚合, 再按长度优雅切分, 单段建议不超过 `1000` 字符
- `md` 子命令首版禁止整篇 Markdown 直接交给 LLM 改写
- Markdown 文本单元按单请求执行, 不做 batch
- `providers.openai.concurrency` 为可选配置, 默认值 `8`
- Provider 首版按 OpenAI-compatible 单一抽象实现

- 翻译使用 `openai` 或者兼容 `openai` api 的 `llm` 进行翻译

## 配置文件

- 优先级 `user-scope` (~/.config/tl/config.json) > env variable

`config.json`
```json
{
  "providers":{
    "openai":{
      "base_url":"",
      "token":"",
      "model":"",
      "concurrency": 8,
      "custom_prompt":" 可以缺省".
    }
  }
}
```

