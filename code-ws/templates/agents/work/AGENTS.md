# AGENTS.md

## Rule

- `ROOT_REPO` is the local workspace root that aggregates multiple git worktree checkouts.
- Workflow has three phases: `plan`, `code`, `deploy`.
- `spec/` is human-maintained PRD and context. Agents must read it, and must not edit existing human docs unless the user asks.
- Agent outputs go to `tasks/`, not `spec/`, unless the user explicitly asks for an `agent_*.md` note.
- Always read `$ROOT_REPO/spec/context.md` before starting work.
- Only change projects listed in `$ROOT_REPO/project.yml`. That file also records the working git branch and optional env id.

## Tools

- Prefer project-native build/test commands discovered from each repo.
- Do not invent company-specific deploy tooling. Use whatever the workspace docs describe.

## Spec Rule

- `$ROOT_REPO/spec` stores PRDs and human context for the current change.
- Non-`agent_` documents under `spec/` are human-owned.

## Tasks Rule

- `$ROOT_REPO/tasks` stores plans, execution notes, and deploy notes.
- `tasks/task.md` is the index of all tasks.
- Optional per-task directory name: `{project-name}-{task-name}`.
