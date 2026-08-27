/** 为什么: Redis 和 kafka 语句都按空白+引号切 token, 头永远是第一个词. */
export function splitArgs(stmt: string): string[] {
  const args: string[] = [];
  let index = 0;
  while (index < stmt.length) {
    while (index < stmt.length && /\s/.test(stmt[index] ?? "")) {
      index += 1;
    }
    if (index >= stmt.length) {
      break;
    }
    const ch = stmt[index] ?? "";
    if (ch === "'" || ch === '"') {
      const start = index + 1;
      index += 1;
      while (index < stmt.length && stmt[index] !== ch) {
        if (stmt[index] === "\\") {
          index += 2;
          continue;
        }
        index += 1;
      }
      args.push(stmt.slice(start, index));
      index += 1;
      continue;
    }
    const start = index;
    while (index < stmt.length && !/\s/.test(stmt[index] ?? "")) {
      index += 1;
    }
    args.push(stmt.slice(start, index));
  }
  return args;
}
