const UNIT: Record<string, number> = {
  B: 1,
  KiB: 1024,
  MiB: 1024 ** 2,
  GiB: 1024 ** 3,
  TiB: 1024 ** 4,
};

/** 为什么: rclone 的汇总不是稳定 API. 解析不到就留空, 不能猜一个 0 当成没传文件. */
export type Stats = {
  bytes?: number;
  checks?: number;
  transfers?: number;
};

export function parseStats(stderr: string): Stats {
  const stats: Stats = {};
  const bytes = lastMatch(stderr, /^Transferred:\s+([0-9.]+)\s*([KMGTP]?i?B)\b/gm);
  const checks = lastMatch(stderr, /^Checks:\s+(\d+)\s+\//gm);
  const transfers = lastMatch(stderr, /^Transferred:\s+(\d+)\s+\//gm);
  if (bytes !== undefined) {
    const qty = Number(bytes[1]);
    const unit = UNIT[bytes[2] ?? ""];
    if (Number.isFinite(qty) && unit !== undefined) {
      stats.bytes = Math.round(qty * unit);
    }
  }
  if (checks !== undefined) {
    stats.checks = Number(checks[1]);
  }
  if (transfers !== undefined) {
    stats.transfers = Number(transfers[1]);
  }
  return stats;
}

/** 为什么: 运行记录和报错文本会落盘. 连接串里的口令即使已经 obscure, 也不能写进记录. */
export function redactText(text: string, secrets: string[]): string {
  let out = text;
  const sorted = [...secrets].filter((item) => item !== "").sort((a, b) => b.length - a.length);
  for (const secret of sorted) {
    out = out.replaceAll(secret, "***");
  }
  return out;
}

export function redactArgs(args: string[], label: string): string[] {
  return args.map((arg) => {
    if (arg.startsWith(":webdav") || arg.startsWith(":crypt")) {
      return label;
    }
    return arg;
  });
}

function lastMatch(text: string, re: RegExp): RegExpExecArray | undefined {
  let found: RegExpExecArray | undefined;
  re.lastIndex = 0;
  let match = re.exec(text);
  while (match !== null) {
    found = match;
    match = re.exec(text);
  }
  return found;
}
