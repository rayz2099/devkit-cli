import { probeStmt, runDriver } from "./drivers";
import { DsnErr, type Kind, type Profile, type QueryOut, type Timeouts } from "./types";

export type DoctorRow = {
  name: string;
  kind: Kind;
  status: "ok" | "fail";
  ms: number;
  error: string;
};

export type ProbeFn = (kind: Kind, url: string, timeouts: Timeouts) => Promise<void>;

/** 为什么: doctor 走 Driver ping, 复用 Query 连接栈, 但不经过 Gate. */
export async function driverProbe(kind: Kind, url: string, timeouts: Timeouts): Promise<void> {
  const stmt = probeStmt(kind);
  await runDriver(kind, url, stmt, timeouts);
}

/** 为什么: 全开并行会把握手挤过 connectTimeout, 单独 query 能通的库 doctor 误报 ETIMEDOUT. */
export const DOCTOR_CONC = 4;

/** 为什么: 一个 Profile 挂死不能拖住其余探测, 限并发且每条有硬超时. */
export async function checkProfiles(
  profiles: Profile[],
  timeouts: Timeouts,
  probe: ProbeFn,
): Promise<DoctorRow[]> {
  const deadlineMs = timeouts.connectMs + timeouts.execMs;
  async function probeRow(profile: Profile): Promise<DoctorRow> {
    return checkOne(profile, timeouts, deadlineMs, probe);
  }
  return mapPool(profiles, DOCTOR_CONC, probeRow);
}

/** 为什么: 连接风暴来自 Promise.all 全开, 必须用固定 worker 池限制在途握手. */
async function mapPool<T, R>(items: T[], conc: number, fn: (item: T) => Promise<R>): Promise<R[]> {
  const out: R[] = new Array(items.length);
  let cursor = 0;
  async function worker(): Promise<void> {
    while (cursor < items.length) {
      const index = cursor;
      cursor += 1;
      const item = items[index];
      if (item === undefined) {
        return;
      }
      out[index] = await fn(item);
    }
  }
  const n = Math.min(conc, items.length);
  const workers = Array.from({ length: n }, () => worker());
  await Promise.all(workers);
  return out;
}

async function checkOne(
  profile: Profile,
  timeouts: Timeouts,
  deadlineMs: number,
  probe: ProbeFn,
): Promise<DoctorRow> {
  const started = Date.now();
  const work = probe(profile.kind, profile.url, timeouts);
  try {
    await withDeadline(work, deadlineMs);
    return {
      name: profile.name,
      kind: profile.kind,
      status: "ok",
      ms: Date.now() - started,
      error: "",
    };
  } catch (error) {
    return {
      name: profile.name,
      kind: profile.kind,
      status: "fail",
      ms: Date.now() - started,
      error: errText(error),
    };
  }
}

async function withDeadline<T>(work: Promise<T>, ms: number): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = setTimeout(() => {
      reject(new DsnErr(`timeout ${ms}ms`, 3));
    }, ms);
  });
  try {
    return await Promise.race([work, timeout]);
  } finally {
    if (timer !== undefined) {
      clearTimeout(timer);
    }
  }
}

export function doctorOut(rows: DoctorRow[]): QueryOut {
  return {
    columns: ["name", "kind", "status", "ms", "error"],
    rows: rows.map((row) => ({
      name: row.name,
      kind: row.kind,
      status: row.status,
      ms: row.ms,
      error: row.error,
    })),
  };
}

function errText(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}
