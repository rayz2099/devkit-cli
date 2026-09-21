/**
 * Markdown 资源地址解析: 链接/图片必须相对源文件, 不能相对当前浏览器 URL.
 * 目录页内嵌 README 时 URL 常无尾斜杠, 浏览器会把 architecture.md 解析到上一级.
 */

export type MdHrefKind = "link" | "image";

/**
 * 把 Markdown href 收成站点路径.
 * 外链与页内锚点保持原样; 图片走 /raw, 才能在 article 里直接显示.
 */
export function resolveMdHref(
  srcPath: string,
  href: string,
  kind: MdHrefKind,
): string {
  if (href.length === 0 || href.startsWith("#") || isAbsMdHref(href)) {
    return href;
  }
  const split = splitMdHref(href);
  if (split.path.length === 0) {
    return href;
  }
  if (split.path.startsWith("/raw/") || split.path.startsWith("/api/")) {
    return split.path + split.suffix;
  }
  const joined = split.path.startsWith("/")
    ? split.path
    : joinSrcDir(srcPath, split.path);
  const encoded = encodeMdPath(normalizeMdRel(joined));
  if (kind === "image") {
    return "/raw/" + encoded + split.suffix;
  }
  return "/" + encoded + split.suffix;
}

function isAbsMdHref(href: string): boolean {
  return /^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(href) || href.startsWith("//");
}

function splitMdHref(href: string): { path: string; suffix: string } {
  const q = href.indexOf("?");
  const h = href.indexOf("#");
  let cut = -1;
  if (q >= 0 && (h < 0 || q < h)) {
    cut = q;
  } else if (h >= 0) {
    cut = h;
  }
  if (cut < 0) {
    return { path: href, suffix: "" };
  }
  return { path: href.slice(0, cut), suffix: href.slice(cut) };
}

function joinSrcDir(srcPath: string, rel: string): string {
  const src = srcPath.replace(/\\/g, "/").replace(/^\/+/, "");
  const slash = src.lastIndexOf("/");
  const dir = slash >= 0 ? src.slice(0, slash) : "";
  return dir.length === 0 ? rel : dir + "/" + rel;
}

function normalizeMdRel(path: string): string {
  const parts: string[] = [];
  for (const seg of path.replace(/\\/g, "/").split("/")) {
    if (seg.length === 0 || seg === ".") {
      continue;
    }
    if (seg === "..") {
      if (parts.length > 0) {
        parts.pop();
      }
      continue;
    }
    parts.push(seg);
  }
  return parts.join("/");
}

function encodeMdPath(path: string): string {
  if (path.length === 0) {
    return "";
  }
  return path.split("/").map(encodeURIComponent).join("/");
}

/**
 * 浏览器端同名实现, 嵌入 serve UI 脚本.
 * 与 resolveMdHref 保持同一规则, 避免目录页与文件页各走一套解析.
 */
export const CLIENT_RESOLVE_MD_HREF = `function resolveMdHref(srcPath, href, kind) {
      if (!href || href.charAt(0) === "#" || /^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(href) || href.indexOf("//") === 0) {
        return href;
      }
      var pathPart = href;
      var suffix = "";
      var q = href.indexOf("?");
      var h = href.indexOf("#");
      var cut = -1;
      if (q >= 0 && (h < 0 || q < h)) cut = q;
      else if (h >= 0) cut = h;
      if (cut >= 0) {
        pathPart = href.slice(0, cut);
        suffix = href.slice(cut);
      }
      if (!pathPart) return href;
      if (pathPart.indexOf("/raw/") === 0 || pathPart.indexOf("/api/") === 0) {
        return pathPart + suffix;
      }
      var src = String(srcPath || "").replace(/\\\\/g, "/");
      var slash = src.lastIndexOf("/");
      var dir = slash >= 0 ? src.slice(0, slash) : "";
      var raw = pathPart.charAt(0) === "/" ? pathPart : (dir ? dir + "/" + pathPart : pathPart);
      var segs = [];
      var pieces = raw.replace(/\\\\/g, "/").split("/");
      for (var i = 0; i < pieces.length; i++) {
        var seg = pieces[i];
        if (!seg || seg === ".") continue;
        if (seg === "..") {
          if (segs.length) segs.pop();
          continue;
        }
        segs.push(seg);
      }
      var encoded = segs.map(encodeURIComponent).join("/");
      if (kind === "image") return "/raw/" + encoded + suffix;
      return "/" + encoded + suffix;
    }`;
