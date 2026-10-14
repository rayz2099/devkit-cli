import { basename, extname } from "node:path";
import { readIndexText } from "./serve-fs";

/** 定义在前, 引用在后, 前端据此决定直跳还是浮层。 */
export type SymbolHit = {
  path: string;
  line: number;
  kind: "def" | "ref";
  text: string;
};

export type SymbolResult = {
  query: string;
  hits: SymbolHit[];
  truncated: boolean;
};

/** 单字符几乎行行命中, 没有跳转价值。 */
const SYMBOL_RE = /^[A-Za-z_][A-Za-z0-9_]{1,79}$/;

/** 单文件上限。再大的文本不参与一次点击触发的扫描。 */
const FILE_MAX = 1024 * 1024;

/** 整次查询的读取预算, 防止大仓把只读服务堵死。 */
const BYTE_BUDGET = 32 * 1024 * 1024;

const DEF_CAP = 20;
const REF_CAP = 30;

/**
 * 在已索引文件里找符号的定义和引用。
 * 不启 LSP: serve 要秒开, 语言服务的启动成本超出只读预览。
 */
export function searchSymbol(
  root: string,
  files: string[],
  query: string,
): SymbolResult {
  if (!SYMBOL_RE.test(query)) {
    return { query, hits: [], truncated: false };
  }

  const ordered = files.slice().sort((a, b) => rankFile(a, query) - rankFile(b, query));
  const defs = defResOf(query);
  const word = wordReOf(query);
  const hits: SymbolHit[] = [];
  const seen = new Set<string>();
  let bytes = 0;
  let defN = 0;
  let refN = 0;
  let truncated = false;
  let stop = false;

  for (const rel of ordered) {
    if (stop) {
      break;
    }
    if (bytes >= BYTE_BUDGET) {
      truncated = true;
      break;
    }
    const text = readSafe(root, rel);
    if (text === null) {
      continue;
    }
    bytes += text.length;
    const lines = text.split("\n");
    for (let i = 0; i < lines.length; i += 1) {
      if (defN >= DEF_CAP && refN >= REF_CAP) {
        truncated = true;
        stop = true;
        break;
      }
      const line = lines[i] ?? "";
      if (!word.test(line)) {
        continue;
      }
      const isDef = defs.some((re) => re.test(line));
      if (isDef && defN >= DEF_CAP) {
        continue;
      }
      if (!isDef && refN >= REF_CAP) {
        continue;
      }
      const lineNo = i + 1;
      const key = `${rel}:${lineNo}`;
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);
      if (isDef) {
        defN += 1;
      } else {
        refN += 1;
      }
      hits.push({
        path: rel,
        line: lineNo,
        kind: isDef ? "def" : "ref",
        text: clipLine(line),
      });
    }
  }

  hits.sort(compareHit);
  return { query, hits, truncated };
}

function readSafe(root: string, rel: string): string | null {
  try {
    return readIndexText(root, rel, FILE_MAX);
  } catch {
    // 索引可能刚过期, 单条路径失败不能让整次符号查询失败。
    return null;
  }
}

function rankFile(file: string, query: string): number {
  const base = basename(file, extname(file));
  if (base === query) {
    return 0;
  }
  return 1;
}

function clipLine(line: string): string {
  const trimmed = line.trim();
  if (trimmed.length <= 180) {
    return trimmed;
  }
  return trimmed.slice(0, 180);
}

function compareHit(a: SymbolHit, b: SymbolHit): number {
  if (a.kind !== b.kind) {
    return a.kind === "def" ? -1 : 1;
  }
  if (a.path !== b.path) {
    return a.path.localeCompare(b.path);
  }
  return a.line - b.line;
}

function escapeRe(name: string): string {
  return name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function wordReOf(name: string): RegExp {
  return new RegExp(`\\b${escapeRe(name)}\\b`);
}

function defResOf(name: string): RegExp[] {
  const n = escapeRe(name);
  const srcs = [
    `\\bclass\\s+${n}\\b`,
    `\\binterface\\s+${n}\\b`,
    `\\benum\\s+(?:class\\s+)?${n}\\b`,
    `\\btype\\s+${n}\\b`,
    `\\b(?:fun|func|fn|def)\\s+(?:\\([^)\\n]*\\)\\s*)?${n}\\b`,
    `\\b(?:const|let|var|val)\\s+${n}\\b`,
    `\\b(?:struct|trait|mod|object|typealias|namespace)\\s+${n}\\b`,
    `\\b(?:async\\s+)?function\\s+${n}\\b`,
    `#define\\s+${n}\\b`,
    `\\b(?:public|private|protected|internal)\\s+(?:static\\s+|final\\s+|open\\s+|override\\s+|abstract\\s+|suspend\\s+)*[\\w.<>,?\\[\\]]+\\s+${n}\\s*\\(`,
  ];
  return srcs.map((src) => new RegExp(src));
}
