import { lstatSync, readdirSync } from "node:fs";
import { join } from "node:path";

export type LocalFile = {
  srcPath: string;
  relPath: string;
};

export type LocalPlan =
  | { kind: "file"; srcPath: string }
  | { kind: "dir"; files: LocalFile[] };

/** 为什么: sync 必须先把本机树摊平, 才能一对一映射到 Alist POSIX 路径. */
export function planLocal(src: string): LocalPlan {
  let st: ReturnType<typeof lstatSync>;
  try {
    st = lstatSync(src);
  } catch {
    throw new Error(`path not found: ${src}`);
  }
  if (st.isSymbolicLink()) {
    throw new Error(`symlink not supported: ${src}`);
  }
  if (st.isFile()) {
    return { kind: "file", srcPath: src };
  }
  if (!st.isDirectory()) {
    throw new Error(`not a file or directory: ${src}`);
  }
  const files = walkDir(src, "");
  files.sort((left, right) => left.relPath.localeCompare(right.relPath));
  return { kind: "dir", files };
}

function walkDir(root: string, rel: string): LocalFile[] {
  const dir = rel === "" ? root : join(root, rel);
  const entries = readdirSync(dir, { withFileTypes: true });
  const files: LocalFile[] = [];
  for (const entry of entries) {
    const childRel = rel === "" ? entry.name : `${rel}/${entry.name}`;
    const full = join(root, childRel);
    if (entry.isSymbolicLink()) {
      throw new Error(`symlink not supported: ${full}`);
    }
    if (entry.isDirectory()) {
      files.push(...walkDir(root, childRel));
      continue;
    }
    if (!entry.isFile()) {
      throw new Error(`not a file or directory: ${full}`);
    }
    files.push({ srcPath: full, relPath: childRel });
  }
  return files;
}
