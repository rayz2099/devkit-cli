import { cpSync, existsSync, mkdirSync, symlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const forkCopyEntries = [
  "project.yml",
  "docs",
  "spec",
  "tasks",
  "README.md",
  ".agents",
];

const agentsFile = "AGENTS.md";

/**
 * 只 link 路由说明, 避免 workspace 持有可被 git 仓改写的副本.
 */
export function applyAgentsTemplate(
  tplDir: string,
  wsDir: string,
): void {
  const src = join(tplDir, agentsFile);
  if (!existsSync(src)) {
    throw new Error(`agents template not found: ${src}`);
  }

  mkdirSync(wsDir, {
    recursive: true,
  });

  const dst = join(wsDir, agentsFile);
  if (existsSync(dst)) {
    throw new Error(`workspace template target already exists: ${dst}`);
  }

  symlinkSync(src, dst);

  const specDir = join(wsDir, "spec");
  const tasksDir = join(wsDir, "tasks");
  mkdirSync(specDir, {
    recursive: true,
  });
  mkdirSync(tasksDir, {
    recursive: true,
  });

  const ctx = join(specDir, "context.md");
  if (!existsSync(ctx)) {
    writeFileSync(
      ctx,
      "# Context\n\nDescribe the current task, target services, and links to PRD files here.\n",
    );
  }
}

/**
 * fork 只复制 workspace 根目录的任务上下文白名单, 因为 repo worktree 必须由 git 重新创建.
 * AGENTS.md 不在此列, 由 applyAgentsTemplate 重新 link 到 XDG.
 */
export function copyForkWorkspaceEntries(
  srcWsDir: string,
  dstWsDir: string,
): string[] {
  mkdirSync(dstWsDir, {
    recursive: true,
  });

  const copied: string[] = [];
  for (const entry of forkCopyEntries) {
    const src = join(srcWsDir, entry);
    if (!existsSync(src)) {
      continue;
    }

    const dst = join(dstWsDir, entry);
    if (existsSync(dst)) {
      throw new Error(`workspace fork target already exists: ${dst}`);
    }

    cpSync(
      src,
      dst,
      {
        recursive: true,
        errorOnExist: true,
        force: false,
      },
    );
    copied.push(entry);
  }

  return copied;
}
