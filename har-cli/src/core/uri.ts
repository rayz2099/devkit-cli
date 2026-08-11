/**
 * 将完整 URL 归一化为 path/url/prefix 三类输出视图。
 */
export function buildUriView(rawUrl: string, prefixDepth: number): { url: string; path: string; prefix: string } {
  const parsed = new URL(rawUrl);
  const path = parsed.pathname || "/";
  const prefix = buildPrefix(path, prefixDepth);

  return {
    url: rawUrl,
    path,
    prefix,
  };
}

/**
 * 按路径前 N 段构造权限前缀。
 */
export function buildPrefix(path: string, depth: number): string {
  const segments = path.split("/").filter(Boolean);

  if (segments.length === 0) {
    return "/*";
  }

  const safeDepth = Math.max(1, depth);
  const picked = segments.slice(0, safeDepth).join("/");
  return `/${picked}/*`;
}
