/**
 * Cmd+K 路径排序: 模糊匹配 + 扩展名过滤 + 最近访问加权.
 * 与前端 palette 共用同一套规则, 避免 "搜得到/搜不到" 口径分裂.
 */

export type SearchQuery = {
  /** 空白分隔后的模糊词, 全部命中才保留. */
  terms: string[];
  /** 规范化扩展名, 如 .md /.kt; 空表示不限类型. */
  exts: string[];
};

export type RankedPath = {
  path: string;
  score: number;
};

const TYPE_ALIASES: Record<string, string[]> = {
  md: [".md", ".markdown"],
  markdown: [".md", ".markdown"],
  kt: [".kt", ".kts"],
  kotlin: [".kt", ".kts"],
  ts: [".ts", ".tsx"],
  js: [".js", ".jsx", ".mjs", ".cjs"],
  py: [".py"],
  go: [".go"],
  rs: [".rs", ".rust"],
  java: [".java"],
  json: [".json"],
  yml: [".yml", ".yaml"],
  yaml: [".yml", ".yaml"],
};

/**
 * 解析人类输入: `readme .md` / `ext:kt oss` / `type:markdown api`.
 */
export function parseSearchQuery(raw: string): SearchQuery {
  const terms: string[] = [];
  const exts: string[] = [];
  const tokens = raw.trim().split(/\s+/).filter(Boolean);

  for (const token of tokens) {
    const lower = token.toLowerCase();
    if (lower.startsWith("ext:") || lower.startsWith("type:")) {
      const val = lower.slice(lower.indexOf(":") + 1).replace(/^\./, "");
      if (val.length === 0) {
        continue;
      }
      const mapped = TYPE_ALIASES[val];
      if (mapped !== undefined) {
        for (const ext of mapped) {
          pushUnique(exts, ext);
        }
      } else {
        pushUnique(exts, `.${val}`);
      }
      continue;
    }
    if (lower.startsWith(".") && lower.length > 1 && !lower.slice(1).includes("/")) {
      pushUnique(exts, lower);
      continue;
    }
    terms.push(token);
  }

  return { terms, exts };
}

function pushUnique(arr: string[], value: string): void {
  if (!arr.includes(value)) {
    arr.push(value);
  }
}

function basenameOf(path: string): string {
  const idx = path.lastIndexOf("/");
  return idx >= 0 ? path.slice(idx + 1) : path;
}

function extOf(path: string): string {
  const base = basenameOf(path).toLowerCase();
  const dot = base.lastIndexOf(".");
  if (dot <= 0) {
    return "";
  }
  return base.slice(dot);
}

/**
 * fzf 风格子序列打分: 连续命中/边界命中加权, 过散则淘汰, 压掉 readme→dt-metadata 误伤.
 */
export function fuzzyScore(text: string, query: string): number {
  if (query.length === 0) {
    return 0;
  }
  const hay = text.toLowerCase();
  const needle = query.toLowerCase();

  if (hay === needle) {
    return 10_000;
  }

  const exact = hay.indexOf(needle);
  if (exact >= 0) {
    let score = 5_000 - exact * 2 - Math.max(0, hay.length - needle.length);
    if (exact === 0) {
      score += 800;
    } else {
      const prev = hay[exact - 1];
      if (prev === "/" || prev === "-" || prev === "_" || prev === ".") {
        score += 400;
      }
    }
    return score;
  }

  let qi = 0;
  let score = 0;
  let prevMatch = -2;
  let gaps = 0;
  for (let i = 0; i < hay.length && qi < needle.length; i += 1) {
    if (hay[i] !== needle[qi]) {
      continue;
    }
    let gain = 12;
    if (i === prevMatch + 1) {
      gain += 48;
    } else if (prevMatch >= 0) {
      gaps += i - prevMatch - 1;
      gain -= Math.min(30, (i - prevMatch - 1) * 4);
    }
    if (i === 0) {
      gain += 24;
    } else {
      const prev = hay[i - 1];
      if (prev === "/" || prev === "-" || prev === "_" || prev === ".") {
        gain += 20;
      }
    }
    score += gain;
    prevMatch = i;
    qi += 1;
  }

  if (qi < needle.length) {
    return -1;
  }
  // 间隙过大视为误匹配 (readme.md vs tools-cdn-replace/.../dt-metadata.md).
  if (gaps > Math.max(4, needle.length)) {
    return -1;
  }
  score -= gaps * 6;
  score -= Math.max(0, hay.length - needle.length);
  return score > 0 ? score : -1;
}

/**
 * 单文件相对 query 的综合分; 文件名权重大于路径.
 */
export function scorePath(path: string, query: SearchQuery): number {
  if (query.exts.length > 0) {
    const ext = extOf(path);
    if (!query.exts.includes(ext)) {
      return -1;
    }
  }

  if (query.terms.length === 0) {
    return query.exts.length > 0 ? 1 : 0;
  }

  const base = basenameOf(path);
  let total = 0;
  for (const term of query.terms) {
    const baseScore = fuzzyScore(base, term);
    const pathScore = fuzzyScore(path, term);
    // 带点的查询更像文件名, 必须命中 basename, 避免路径字符拼出假阳性.
    const fileLike = term.includes(".");
    if (fileLike && baseScore < 0) {
      return -1;
    }
    const best = Math.max(
      baseScore >= 0 ? baseScore + 1_200 : -1,
      fileLike ? -1 : pathScore,
    );
    if (best < 0) {
      return -1;
    }
    total += best;
  }
  // 短路径略优先, monorepo 里同名 README 更靠前的叶子更贴近当前意图.
  total -= Math.min(80, path.split("/").length * 4);
  return total;
}

/**
 * 排序检索结果; recent 仅在空 query 时作为主列表, 有 query 时做轻量加权.
 */
export function rankPaths(
  files: string[],
  rawQuery: string,
  recent: string[] = [],
  limit = 50,
): RankedPath[] {
  const query = parseSearchQuery(rawQuery);
  const empty = query.terms.length === 0 && query.exts.length === 0;
  if (empty) {
    const seen = new Set<string>();
    const out: RankedPath[] = [];
    for (const path of recent) {
      if (!files.includes(path) || seen.has(path)) {
        continue;
      }
      seen.add(path);
      out.push({ path, score: 1_000_000 - out.length });
      if (out.length >= limit) {
        return out;
      }
    }
    for (const path of files) {
      if (seen.has(path)) {
        continue;
      }
      out.push({ path, score: 0 });
      if (out.length >= limit) {
        break;
      }
    }
    return out;
  }

  const recentBoost = new Map<string, number>();
  recent.forEach((path, idx) => {
    recentBoost.set(path, Math.max(0, 200 - idx * 8));
  });

  const ranked: RankedPath[] = [];
  for (const path of files) {
    const base = scorePath(path, query);
    if (base < 0) {
      continue;
    }
    ranked.push({
      path,
      score: base + (recentBoost.get(path) ?? 0),
    });
  }
  ranked.sort((a, b) => b.score - a.score || a.path.localeCompare(b.path));
  return ranked.slice(0, limit);
}
