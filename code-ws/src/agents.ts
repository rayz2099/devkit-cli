import { cpSync, existsSync, mkdirSync, readdirSync } from "node:fs";
import { join } from "node:path";

const forkCopyEntries = [
  "project.yml",
  "docs",
  "spec",
  "tasks",
  "AGENTS.md",
  "README.md",
  ".agents",
];

/**
 * 复制模板内容而不是模板目录本身, 因为 work-01 已经表达 workspace 根结构。
 */
export function copyAgentsTemplate(
  tplDir: string,
  wsDir: string,
): void {
  if (!existsSync(tplDir)) {
    throw new Error(`agents template not found: ${tplDir}`);
  }

  mkdirSync(wsDir, {
    recursive: true,
  });

  const entries = readdirSync(tplDir);
  for (const entry of entries) {
    const dst = join(wsDir, entry);
    if (existsSync(dst)) {
      throw new Error(`workspace template target already exists: ${dst}`);
    }
  }

  for (const entry of entries) {
    cpSync(
      join(tplDir, entry),
      join(wsDir, entry),
      {
        recursive: true,
        errorOnExist: true,
        force: false,
      },
    );
  }
}

/**
 * fork 只复制 workspace 根目录的任务上下文白名单, 因为 repo worktree 必须由 git 重新创建。
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
