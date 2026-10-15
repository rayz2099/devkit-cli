# code-ws

Bun + TypeScript 实现的 VS Code workspace 初始化 CLI。

```bash
bun run devkit-cli/code-ws/src/main.ts list
bun run devkit-cli/code-ws/src/main.ts config check
bun run devkit-cli/code-ws/src/main.ts init feature/spec101 -t work-01
bun run devkit-cli/code-ws/src/main.ts init feature/brush -t work --project dt-brush
bun run devkit-cli/code-ws/src/main.ts init feature/spec101 shared-lib
bun run devkit-cli/code-ws/src/main.ts add project shared-lib
bun run devkit-cli/code-ws/src/main.ts add project my-room --branch feature/room-organizing
bun run devkit-cli/code-ws/src/main.ts init feature/baize/mail -t game-room --from my-room=V0.0.1
bun run devkit-cli/code-ws/src/main.ts sync master --from my-room=V0.0.1
bun run devkit-cli/code-ws/src/main.ts fork feature/spec102
bun run devkit-cli/code-ws/src/main.ts destroy
bun run devkit-cli/code-ws/src/main.ts completion fish
```

默认读取 `$HOME/.config/code-ws/config.json`, `just install code-ws` 会把 `conf/` 下配置链接过去。

`init <branch> -t <profile>` 会从每个 repo 配置的主分支拉取并创建同名分支 worktree, 未声明时默认使用 `master`, 然后生成 `.code-workspace` 和 `project.yml`, 并把 `$HOME/.config/code-ws/templates/agents/<name>/AGENTS.md` symlink 到 workspace 根目录. 可重复 `--from <repo=branch>` 把该仓库的合并目标改成远端分支, 不改变任务分支的切出点. 分支名和远端 `refs/heads` 完全一致. `project.yml` 里每个仓库都会写出 `merge`, agent 的 CR 和 diff 读这个字段.

`init <branch> <project>` 只用单个项目初始化 workspace, 不读取 profile 的 repo 列表, 但仍按 config 顶层 `initAgentsTemplate` link XDG 里的 `AGENTS.md`. 适合先创建一个只有首个项目的 workspace, 后续再通过 `add project <repo>` 逐个追加.

`init <branch> -t <profile> --project <repo>` 从 profile 取 agents 模板, 仓库集合替换为指定项目（项目来自全局 project catalog, 不限于 profile 的 repos）。`-p` 是 `--project` 的短写；省略 `-t` 时使用 `initAgentsTemplate`。一次只接受一个项目, 追加仓库在新 workspace 内运行 `add project <repo>`。

初始化成功会输出 `workspace directory: <path>`。agent 应把该路径作为后续命令的工作目录, 读取生成的 `project.yml` 和 `AGENTS.md` 后进入开发流程；CLI 子进程不能改变调用者的当前目录。

workspace 内的 `project.yml` 会写入固定 `branch` 和项目 `description`, 供 agent 直接读取项目语义。

`add project <repo>` 需要在 workspace 目录或其子目录执行, 它会按 workspace `project.yml` 中记录的固定 `branch` 新增 worktree, 并同步更新 `.code-workspace` 和 `project.yml`。只读项目需要固定到特定分支时可传 `--branch <branch>`, workspace 内会创建 detached worktree 指向 `origin/<branch>`, 不占用同名本地分支; 生成的 workspace `project.yml` 会在该 repo 下保存 `branch`, 后续 `sync master` 和 `fork` 会继续复用。

`sync master` 需要在 workspace 目录或其子目录执行, 它只会把每个项目的合并基线更新到远程同名分支, 不会 checkout、merge 或 push。合并基线来自 workspace `project.yml` 的 `merge`, 没有该字段时用仓库主分支。只读仓库的 `branch` 仍同步那条固定 checkout。`--from <repo=branch>` 可重复, 先把对应仓库的 `merge` 改写成远端上存在的分支名, 再按新的 `merge` 同步; 任务分支不动。

`fork <branch>` 需要在 workspace 目录或其子目录执行, 它会先检查当前 workspace 中所有 repo 没有未提交改动, 再从当前 repo worktree 的 `HEAD` 派生目标分支 worktree。它会复制固定枚举的任务上下文: `project.yml`, `docs/`, `spec/`, `tasks/`, `.agents/`, 再把 XDG `AGENTS.md` 重新 link 到目标 workspace, 并生成 `.code-workspace` 和 `project.yml` 分支名, 各仓库的 `merge` 会一起带过去. 不复制根 `README.md`, 避免把旧阶段图带进新 workspace.

`destroy` 需要在 workspace 目录或其子目录执行. 它会逐仓执行安全检查和 `git worktree remove --force`; 单仓失败不阻断其他仓库, 全部处理后汇总失败并返回非零. 安全检查会拒绝未提交改动和未合入 `origin/<merge>` 的提交, 未写 `merge` 时用仓库主分支, 同时识别 squash/rebase/cherry-pick 产生的等价 patch. 不会改写 `project.yml` / `.code-workspace`, 也不会删除 workspace 目录, 方便后续淬炼复用.

`projects` 会输出 config 解析后的 project name, 供 fish completion 动态提示项目名。`-t <profile>` 只列出该 profile, `--workspace` 只列出当前 workspace。`branches <repo>` 列出该仓库远端分支短名, 给 `--from` 补全用。

Fish 集成使用 `fish/completions/code-ws.fish`, 内容为:

```fish
code-ws completion fish | source
```
