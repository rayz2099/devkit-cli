import {
  existsSync,
  lstatSync,
  readdirSync,
  readFileSync,
  statSync,
} from "node:fs";
import {
  basename,
  extname,
  join,
  relative,
  resolve,
  sep,
} from "node:path";

/** 目录项, 供侧栏树与 API 序列化. */
export type TreeEntry = {
  name: string;
  path: string;
  type: "file" | "dir";
};

/** 文件 blob 元数据; 文本才带 content, 避免把二进制塞进 JSON. */
export type BlobPayload = {
  path: string;
  name: string;
  type: "file" | "dir";
  size: number;
  binary: boolean;
  language: string;
  content?: string;
  entries?: TreeEntry[];
  /** 目录下 GitHub 风格默认 README 相对路径, 有则前端并列渲染. */
  readme?: string;
};

const TEXT_MAX_BYTES = 2 * 1024 * 1024;

const DENY_DIR_NAMES = new Set([
  ".git",
  ".svn",
  ".hg",
  ".ssh",
]);

/** 索引/浏览都跳过的构建与依赖目录, 避免 Cmd+K 被 .class 等噪声挤爆. */
const SKIP_INDEX_DIRS = new Set([
  "node_modules",
  "target",
  "build",
  "dist",
  "out",
  "classes",
  ".gradle",
  ".idea",
  ".vscode",
  "vendor",
  "coverage",
  "__pycache__",
  ".next",
  ".nuxt",
  ".turbo",
  ".cache",
  ".terraform",
  "Pods",
  "bower_components",
]);

/** 检索索引不收录的扩展名: 二进制/产物对跳转无意义. */
const SKIP_INDEX_EXTS = new Set([
  ".class",
  ".jar",
  ".war",
  ".ear",
  ".o",
  ".a",
  ".so",
  ".dylib",
  ".dll",
  ".exe",
  ".bin",
  ".png",
  ".jpg",
  ".jpeg",
  ".gif",
  ".webp",
  ".ico",
  ".bmp",
  ".pdf",
  ".zip",
  ".tar",
  ".gz",
  ".tgz",
  ".7z",
  ".rar",
  ".woff",
  ".woff2",
  ".ttf",
  ".eot",
  ".map",
  ".min.js",
  ".min.css",
]);

/** 源码/文档优先入索引, 配额紧时保住 README 与业务代码. */
const PRIORITY_INDEX_EXTS = new Set([
  ".md",
  ".markdown",
  ".kt",
  ".kts",
  ".java",
  ".ts",
  ".tsx",
  ".js",
  ".jsx",
  ".mjs",
  ".cjs",
  ".go",
  ".rs",
  ".py",
  ".json",
  ".yml",
  ".yaml",
  ".toml",
  ".xml",
  ".gradle",
  ".sql",
  ".proto",
  ".graphql",
  ".sh",
  ".bash",
  ".zsh",
  ".fish",
  ".css",
  ".scss",
  ".html",
  ".txt",
]);

const DENY_FILE_RES: RegExp[] = [
  /^\.env(\..+)?$/i,
  /^id_rsa/i,
  /^id_ed25519/i,
  /^id_dsa/i,
  /\.pem$/i,
  /\.p12$/i,
  /\.pfx$/i,
  /\.key$/i,
  /secret/i,
  /credentials/i,
];

const LANG_BY_EXT: Record<string, string> = {
  ".ts": "typescript",
  ".tsx": "tsx",
  ".js": "javascript",
  ".jsx": "jsx",
  ".mjs": "javascript",
  ".cjs": "javascript",
  ".json": "json",
  ".md": "markdown",
  ".markdown": "markdown",
  ".yml": "yaml",
  ".yaml": "yaml",
  ".toml": "toml",
  ".rs": "rust",
  ".go": "go",
  ".py": "python",
  ".java": "java",
  ".kt": "kotlin",
  ".kts": "kotlin",
  ".css": "css",
  ".scss": "scss",
  ".html": "html",
  ".htm": "html",
  ".xml": "xml",
  ".sh": "bash",
  ".bash": "bash",
  ".zsh": "bash",
  ".fish": "bash",
  ".sql": "sql",
  ".graphql": "graphql",
  ".proto": "protobuf",
  ".c": "c",
  ".h": "c",
  ".cpp": "cpp",
  ".cc": "cpp",
  ".hpp": "cpp",
  ".swift": "swift",
  ".rb": "ruby",
  ".php": "php",
  ".lua": "lua",
  ".r": "r",
  ".dockerfile": "dockerfile",
  ".txt": "plaintext",
  ".svg": "xml",
};

const IMAGE_EXTS = new Set([
  ".png",
  ".jpg",
  ".jpeg",
  ".gif",
  ".webp",
  ".svg",
  ".ico",
  ".bmp",
]);

/**
 * 规范化 serve 根目录, 因为 CLI 允许缺省 cwd 与相对 path.
 */
export function resolveServeRoot(pathArg: string | undefined, cwd: string): string {
  const raw = pathArg === undefined || pathArg.length === 0 ? cwd : pathArg;
  const abs = resolve(cwd, raw);
  if (!existsSync(abs)) {
    throw new Error(`serve path not found: ${abs}`);
  }
  const st = statSync(abs);
  if (!st.isDirectory()) {
    throw new Error(`serve path is not a directory: ${abs}`);
  }
  return abs;
}

/**
 * 判断相对路径是否命中 deny-list, 因为无鉴权时要用路径闸门替代 token.
 */
export function isDeniedRelPath(relPath: string): boolean {
  const parts = relPath
    .replace(/\\/g, "/")
    .split("/")
    .filter((part) => part.length > 0 && part !== ".");

  for (const part of parts) {
    if (part === "..") {
      return true;
    }
    if (DENY_DIR_NAMES.has(part)) {
      return true;
    }
    for (const re of DENY_FILE_RES) {
      if (re.test(part)) {
        return true;
      }
    }
  }
  return false;
}

/**
 * watch 与搜索索引共享噪声目录边界, 避免构建任务把浏览器推送队列打满。
 */
export function isWatchIgnoredRelPath(relPath: string): boolean {
  const clean = relPath.replace(/\\/g, "/");
  if (isDeniedRelPath(clean)) {
    return true;
  }
  return clean
    .split("/")
    .filter((part) => part.length > 0)
    .some((part) => SKIP_INDEX_DIRS.has(part));
}

/**
 * 把 URL/查询 path 安全映射到 root 内绝对路径, 防止 path traversal.
 */
export function resolveUnderRoot(root: string, relPath: string): string {
  const cleaned = relPath
    .replace(/\\/g, "/")
    .replace(/^\/+/, "")
    .replace(/\/+$/, "");

  if (cleaned.includes("\0")) {
    throw new Error("invalid path");
  }

  if (isDeniedRelPath(cleaned)) {
    throw new Error("path denied");
  }

  const rootAbs = resolve(root);
  const abs = cleaned.length === 0 ? rootAbs : resolve(rootAbs, cleaned);
  if (abs !== rootAbs && !abs.startsWith(rootAbs + sep)) {
    throw new Error("path escapes root");
  }
  if (isDeniedRelPath(relative(rootAbs, abs))) {
    throw new Error("path denied");
  }
  return abs;
}

/**
 * 转成 POSIX 风格相对路径, 统一前后端与深链格式.
 */
export function toPosixRel(root: string, absPath: string): string {
  const rel = relative(resolve(root), resolve(absPath));
  if (rel === "") {
    return "";
  }
  return rel.split(sep).join("/");
}

export function isImagePath(relPath: string): boolean {
  return IMAGE_EXTS.has(extname(relPath).toLowerCase());
}

export function languageOf(relPath: string): string {
  const base = basename(relPath).toLowerCase();
  if (base === "dockerfile") {
    return "dockerfile";
  }
  if (base === "makefile") {
    return "makefile";
  }
  const ext = extname(base).toLowerCase();
  return LANG_BY_EXT[ext] ?? "plaintext";
}

export function isMarkdownPath(relPath: string): boolean {
  const ext = extname(relPath).toLowerCase();
  return ext === ".md" || ext === ".markdown";
}

/**
 * 按 GitHub 优先序挑选目录 README, 因为仓库首页默认要同时给目录树与说明文档.
 */
export function pickReadmePath(entries: TreeEntry[]): string | undefined {
  let best: TreeEntry | undefined;
  let bestScore = -1;

  for (const entry of entries) {
    if (entry.type !== "file") {
      continue;
    }
    const score = readmeScore(entry.name);
    if (score > bestScore) {
      best = entry;
      bestScore = score;
    }
  }
  return best?.path;
}

function readmeScore(name: string): number {
  const n = name.toLowerCase();
  if (n === "readme.md") return 100;
  if (n === "readme.markdown") return 90;
  if (n === "readme.mdown") return 80;
  if (n === "readme.mkd") return 70;
  if (n === "readme.rst") return 60;
  if (n === "readme.txt") return 50;
  if (n === "readme") return 40;
  if (n.startsWith("readme.")) return 30;
  return -1;
}

function listDirEntries(root: string, absDir: string): TreeEntry[] {
  const names = readdirSync(absDir);
  const entries: TreeEntry[] = [];

  for (const name of names) {
    if (DENY_DIR_NAMES.has(name)) {
      continue;
    }
    let denied = false;
    for (const re of DENY_FILE_RES) {
      if (re.test(name)) {
        denied = true;
        break;
      }
    }
    if (denied) {
      continue;
    }

    const abs = join(absDir, name);
    const rel = toPosixRel(root, abs);
    if (isDeniedRelPath(rel)) {
      continue;
    }

    let st;
    try {
      st = lstatSync(abs);
    } catch {
      continue;
    }
    // 侧栏不展开 symlink, 与索引一致, 避免环状目录.
    if (st.isSymbolicLink()) {
      continue;
    }

    if (st.isDirectory()) {
      entries.push({
        name,
        path: rel,
        type: "dir",
      });
      continue;
    }
    if (st.isFile()) {
      entries.push({
        name,
        path: rel,
        type: "file",
      });
    }
  }

  entries.sort((a, b) => {
    if (a.type !== b.type) {
      return a.type === "dir" ? -1 : 1;
    }
    return a.name.localeCompare(b.name);
  });
  return entries;
}

/**
 * 读取目录或文件 blob; 目录返回 entries, 文件按大小/类型决定是否内联文本.
 */
export function readBlob(root: string, relPath: string): BlobPayload {
  const abs = resolveUnderRoot(root, relPath);
  if (!existsSync(abs)) {
    throw new Error("not found");
  }

  const st = statSync(abs);
  const path = toPosixRel(root, abs);
  const name = path === "" ? basename(root) : basename(abs);

  if (st.isDirectory()) {
    const entries = listDirEntries(root, abs);
    return {
      path,
      name,
      type: "dir",
      size: 0,
      binary: false,
      language: "plaintext",
      entries,
      readme: pickReadmePath(entries),
    };
  }

  if (!st.isFile()) {
    throw new Error("unsupported file type");
  }

  const size = st.size;
  if (isImagePath(path)) {
    return {
      path,
      name,
      type: "file",
      size,
      binary: true,
      language: "image",
    };
  }

  if (size > TEXT_MAX_BYTES) {
    return {
      path,
      name,
      type: "file",
      size,
      binary: true,
      language: languageOf(path),
    };
  }

  const buf = readFileSync(abs);
  if (looksBinary(buf)) {
    return {
      path,
      name,
      type: "file",
      size,
      binary: true,
      language: languageOf(path),
    };
  }

  return {
    path,
    name,
    type: "file",
    size,
    binary: false,
    language: languageOf(path),
    content: buf.toString("utf8"),
  };
}

/**
 * 收集可检索文件路径, 供 Cmd+K 模糊跳转, 不走 LSP.
 * 优先收录源码/文档, 跳过构建产物, 避免 5000 上限被 .class 占满导致 README 失踪.
 */
export function listFileIndex(root: string, maxFiles = 30000): string[] {
  const priority: string[] = [];
  const rest: string[] = [];
  walkIndexFiles(root, root, priority, rest, maxFiles);
  const out = priority.length >= maxFiles
    ? priority.slice(0, maxFiles)
    : priority.concat(rest.slice(0, maxFiles - priority.length));
  out.sort((a, b) => a.localeCompare(b));
  return out;
}

function isIndexableFile(name: string): boolean {
  const lower = name.toLowerCase();
  const ext = extname(lower);
  if (SKIP_INDEX_EXTS.has(ext)) {
    return false;
  }
  // Dockerfile/Makefile 等无扩展名源文件仍值得检索.
  if (ext === "" && !/^(dockerfile|makefile|justfile|procfile|gemfile|rakefile)$/i.test(lower)) {
    return lower.startsWith("readme") || lower.startsWith("license") || lower.startsWith("changelog");
  }
  return true;
}

function isPriorityFile(name: string): boolean {
  const lower = name.toLowerCase();
  if (lower.startsWith("readme") || lower === "agents.md") {
    return true;
  }
  return PRIORITY_INDEX_EXTS.has(extname(lower));
}

/** 索引递归最大深度, 防止异常目录结构把调用栈打爆. */
const INDEX_MAX_DEPTH = 64;

function walkIndexFiles(
  root: string,
  absDir: string,
  priority: string[],
  rest: string[],
  maxFiles: number,
  depth = 0,
  visited: Set<string> = new Set(),
): void {
  // priority 收满即可停: 源码/文档已优先, 无需为 rest 继续深挖整树.
  if (priority.length >= maxFiles) {
    return;
  }
  if (depth > INDEX_MAX_DEPTH) {
    return;
  }

  const dirKey = resolve(absDir);
  if (visited.has(dirKey)) {
    return;
  }
  visited.add(dirKey);

  let names: string[];
  try {
    names = readdirSync(absDir);
  } catch {
    return;
  }
  names.sort((a, b) => a.localeCompare(b));

  for (const name of names) {
    if (priority.length >= maxFiles) {
      return;
    }
    if (DENY_DIR_NAMES.has(name) || SKIP_INDEX_DIRS.has(name)) {
      continue;
    }
    let denied = false;
    for (const re of DENY_FILE_RES) {
      if (re.test(name)) {
        denied = true;
        break;
      }
    }
    if (denied) {
      continue;
    }

    const abs = join(absDir, name);
    const rel = toPosixRel(root, abs);
    if (isDeniedRelPath(rel)) {
      continue;
    }

    // lstat 不跟随 symlink, 避免 a -> a 环把 index 卡死或路径爆炸.
    let st;
    try {
      st = lstatSync(abs);
    } catch {
      continue;
    }
    if (st.isSymbolicLink()) {
      continue;
    }

    if (st.isDirectory()) {
      walkIndexFiles(root, abs, priority, rest, maxFiles, depth + 1, visited);
      continue;
    }
    if (!st.isFile() || !isIndexableFile(name)) {
      continue;
    }
    // rest 也设硬顶, 避免超大仓在 priority 不足时无限堆积.
    if (isPriorityFile(name)) {
      priority.push(rel);
    } else if (priority.length + rest.length < maxFiles) {
      rest.push(rel);
    }
  }
}

/**
 * 启发式检测二进制, 避免把乱码当源码渲染.
 */
export function looksBinary(buf: Buffer): boolean {
  if (buf.length === 0) {
    return false;
  }
  const sample = buf.subarray(0, Math.min(buf.length, 8000));
  let suspicious = 0;
  for (const byte of sample) {
    if (byte === 0) {
      return true;
    }
    if (byte < 7 || (byte > 13 && byte < 32)) {
      suspicious += 1;
    }
  }
  return suspicious / sample.length > 0.3;
}

/**
 * 读取 raw 字节给图片等静态资源响应.
 */
export function readRawFile(root: string, relPath: string): {
  abs: string;
  buf: Buffer;
  contentType: string;
} {
  const abs = resolveUnderRoot(root, relPath);
  if (!existsSync(abs) || !statSync(abs).isFile()) {
    throw new Error("not found");
  }
  const buf = readFileSync(abs);
  return {
    abs,
    buf,
    contentType: contentTypeOf(relPath),
  };
}

function contentTypeOf(relPath: string): string {
  const ext = extname(relPath).toLowerCase();
  switch (ext) {
    case ".png":
      return "image/png";
    case ".jpg":
    case ".jpeg":
      return "image/jpeg";
    case ".gif":
      return "image/gif";
    case ".webp":
      return "image/webp";
    case ".svg":
      return "image/svg+xml";
    case ".ico":
      return "image/x-icon";
    case ".bmp":
      return "image/bmp";
    case ".pdf":
      return "application/pdf";
    case ".json":
      return "application/json; charset=utf-8";
    case ".css":
      return "text/css; charset=utf-8";
    case ".js":
    case ".mjs":
      return "text/javascript; charset=utf-8";
    case ".html":
    case ".htm":
      return "text/html; charset=utf-8";
    case ".md":
    case ".markdown":
    case ".txt":
      return "text/plain; charset=utf-8";
    default:
      return "application/octet-stream";
  }
}
