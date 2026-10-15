/** 为什么: 远端是 POSIX 路径, 不能用本机 path.join, Windows 会塞反斜杠. */
export function joinRemote(...parts: string[]): string {
  const segs: string[] = [];
  for (const part of parts) {
    for (const seg of part.split("/")) {
      if (seg === "" || seg === ".") {
        continue;
      }
      if (seg === "..") {
        throw new Error(`invalid remote path: ${parts.join("/")}`);
      }
      segs.push(seg);
    }
  }
  return `/${segs.join("/")}`;
}

/** 为什么: File-Path 要整段 PathEscape, 空格必须是 %20, 不能走 query 的 +. */
export function encodePath(path: string): string {
  return encodeURIComponent(path);
}
