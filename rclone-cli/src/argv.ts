import type { Plan } from "./plan";

/**
 * 为什么: 传输子命令只能是 copy 或 copyto. sync 会删除另一侧多出来的文件, 废片和旧密文都不能被工具清掉.
 * check 改走 lsjson, 不在这里拼.
 */
export function buildArgv(plan: Plan, cfgFile: string, remote: string, dest = plan.local): string[] {
  const ends = orderedEnds(plan, remote, dest);
  const kind = verb(plan);
  const args = ["--config", cfgFile, kind, ends.src, ends.dst];
  // 为什么: rclone copyto 不能带 filter. 单文件加上 --exclude 会直接 CRITICAL, 不是 not found.
  if (kind !== "copyto") {
    for (const pattern of plan.excludes) {
      args.push("--exclude", pattern);
    }
  }
  if (plan.target.skipLinks) {
    args.push("--skip-links");
  }
  if (plan.target.sizeOnly) {
    args.push("--size-only");
  }
  const budget = retryBudget(plan);
  args.push("--transfers", String(plan.transfers));
  args.push("--checkers", String(plan.target.checkers));
  args.push("--retries", String(budget.retries));
  args.push("--retries-sleep", budget.sleep);
  args.push("--low-level-retries", String(budget.low));
  args.push("--timeout", plan.timeout);
  args.push("--contimeout", plan.target.contimeout);
  args.push("--tpslimit", String(plan.tpslimit));
  if (plan.dryRun) {
    args.push("--dry-run");
  }
  if (plan.verbose) {
    args.push("-v");
  } else if (plan.action === "get" || plan.action === "download") {
    args.push("-q");
  }
  if (plan.progress) {
    args.push("--progress");
  }
  return args;
}

/**
 * 为什么: get/download 会自己换密钥对重试. rclone 再跑 10 次 15s 会把一次解错钥匙拖成几分钟.
 */
function retryBudget(plan: Plan): { retries: number; sleep: string; low: number } {
  if (plan.action === "get" || plan.action === "download") {
    return { retries: 1, sleep: "1s", low: 1 };
  }
  return {
    retries: plan.target.retries,
    sleep: plan.target.retriesSleep,
    low: plan.target.lowLevelRetries,
  };
}

export function verb(plan: Plan): "copy" | "copyto" {
  if (plan.action === "check") {
    throw new Error("check lists files; it does not spawn rclone check");
  }
  if (plan.action === "get" || plan.action === "download") {
    return "copyto";
  }
  return "copy";
}

/** 为什么: pull/get/download 只是把 copy 的两端对调, 不能改成 sync, 否则本地多出来的文件会被删掉. */
export function orderedEnds(plan: Plan, remote: string, dest = plan.local): { src: string; dst: string } {
  if (plan.action === "pull" || plan.action === "get" || plan.action === "download") {
    return { src: remote, dst: dest };
  }
  return { src: dest, dst: remote };
}
