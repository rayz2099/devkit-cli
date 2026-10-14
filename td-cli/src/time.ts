/** 为什么: 分析页把 recentDay 展开成本地起止时间再发给计算接口, 只传代号服务端对不上页面结果. */
export function resolveRecent(
  recent: string,
  now: Date,
): { recentDay: string; startTime: string; endTime: string } {
  const parts = recent.split("-");
  if (parts.length !== 2) {
    throw new Error(`unsupported recentDay: ${recent}`);
  }
  const head = parts[0] ?? "";
  const tail = parts[1] ?? "";
  if (!/^\d+$/.test(head) || !/^\d+$/.test(tail)) {
    throw new Error(`unsupported recentDay: ${recent}`);
  }
  const older = head === "0" ? Number(tail) - 1 : Number(tail);
  const newer = Number(head);
  if (older < 0 || newer < 0) {
    throw new Error(`unsupported recentDay: ${recent}`);
  }
  const start = new Date(now);
  start.setHours(0, 0, 0, 0);
  start.setDate(start.getDate() - older);
  const end = new Date(now);
  end.setHours(23, 59, 59, 0);
  end.setDate(end.getDate() - newer);
  return {
    recentDay: recent,
    startTime: fmtLocal(start),
    endTime: fmtLocal(end),
  };
}

/** 为什么: 事件分析页默认过去 7 天, 属性分析没有时间条件. 两种口径不能合成一次默认. */
export function resolveSpan(
  recent: string | undefined,
  from: string | undefined,
  to: string | undefined,
  pageDefault: boolean,
  now: Date,
): { recentDay: string; startTime: string; endTime: string } | undefined {
  const hasFrom = from !== undefined;
  const hasTo = to !== undefined;
  if (hasFrom !== hasTo) {
    throw new Error("--from and --to are both required");
  }
  if ((hasFrom || hasTo) && recent !== undefined) {
    throw new Error("use either --recent or --from/--to");
  }
  if (from !== undefined && to !== undefined) {
    return { recentDay: "", startTime: from, endTime: to };
  }
  if (recent !== undefined) {
    return resolveRecent(recent, now);
  }
  if (pageDefault) {
    return resolveRecent("1-7", now);
  }
  return undefined;
}

export function fmtLocal(date: Date): string {
  const y = date.getFullYear();
  const m = pad(date.getMonth() + 1);
  const d = pad(date.getDate());
  const h = pad(date.getHours());
  const min = pad(date.getMinutes());
  const s = pad(date.getSeconds());
  return `${y}-${m}-${d} ${h}:${min}:${s}`;
}

function pad(value: number): string {
  return String(value).padStart(2, "0");
}
