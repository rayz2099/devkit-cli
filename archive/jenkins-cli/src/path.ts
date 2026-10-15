/** 为什么: JobPath 是领域地址, URL 编码必须集中, 避免 /job 段漏编或编两次. */
export function jobUrl(jobPath: string): string {
  const trimmed = jobPath.replace(/^\/+|\/+$/g, "");
  if (trimmed === "") {
    throw new Error("job path is required");
  }
  return trimmed
    .split("/")
    .map((seg) => `/job/${encodeURIComponent(seg)}`)
    .join("");
}

/** 为什么: 用户明确禁止打主分支包, 用路径最后一段识别, 不靠 job 描述猜测. */
export function isMainline(jobPath: string): boolean {
  const segs = jobPath.split("/").filter((seg) => seg !== "");
  const last = segs[segs.length - 1];
  return last === "master" || last === "main";
}

export function runResult(building: boolean, result: string | null): string {
  if (building) {
    return "IN_PROGRESS";
  }
  return result ?? "UNKNOWN";
}
