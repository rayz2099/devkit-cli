import { isMainline, jobUrl, runResult } from "./path";
import { JenkinsErr } from "./types";
import type {
  JobRef,
  JobView,
  QueueItem,
  RunInfo,
  RunItem,
  RunStatus,
  Runtime,
  TriggerOut,
} from "./types";

export type HttpFn = (input: URL | RequestInfo, init?: RequestInit) => Promise<Response>;

type Crumb = {
  key: string;
  value: string;
};

type RawBuild = {
  number?: number;
  result?: string | null;
  building?: boolean;
  duration?: number;
  estimatedDuration?: number;
  timestamp?: number;
  url?: string;
  displayName?: string;
  fullDisplayName?: string;
  description?: string | null;
  artifacts?: Array<{ fileName?: string; relativePath?: string }>;
  actions?: Array<Record<string, unknown>>;
};

/** 为什么: crumb / 404 / Basic 必须收在一处, 命令层只谈 JobPath 和 Run. */
export class JenkinsClient {
  private crumb: Crumb | undefined;
  private crumbTried = false;

  constructor(
    private readonly runtime: Runtime,
    private readonly http: HttpFn = fetch,
  ) {}

  async listJobs(folder: string): Promise<JobRef[]> {
    const path = folder.trim() === "" ? "/api/json" : `${jobUrl(folder)}/api/json`;
    const data = await this.readJson(path, "tree=jobs[name,url,_class]");
    const jobs = asArr(data.jobs);
    return jobs.map((item) => {
      const raw = asRec(item);
      return {
        name: asStr(raw.name),
        url: asStr(raw.url),
        className: asStr(raw._class),
      };
    });
  }

  async viewJob(jobPath: string): Promise<JobView> {
    const data = await this.readJson(
      `${jobUrl(jobPath)}/api/json`,
      "tree=name,url,buildable,lastBuild[number],lastSuccessfulBuild[number],lastFailedBuild[number]",
    );
    return {
      name: asStr(data.name),
      url: asStr(data.url),
      buildable: data.buildable === true,
      lastBuild: asNum(asRec(data.lastBuild).number),
      lastSuccessfulBuild: asNum(asRec(data.lastSuccessfulBuild).number),
      lastFailedBuild: asNum(asRec(data.lastFailedBuild).number),
    };
  }

  async listRuns(jobPath: string, limit: number): Promise<RunItem[]> {
    const tree = `builds[number,result,building,timestamp,duration]{0,${limit}}`;
    const data = await this.readJson(`${jobUrl(jobPath)}/api/json`, `tree=${tree}`);
    return asArr(data.builds).map((item) => {
      const raw = item as RawBuild;
      return {
        number: raw.number ?? 0,
        result: runResult(raw.building === true, raw.result ?? null),
        building: raw.building === true,
        timestamp: raw.timestamp ?? 0,
        durationMs: raw.duration ?? 0,
      };
    });
  }

  async viewRun(jobPath: string, buildNo: string): Promise<RunInfo> {
    const raw = (await this.readJson(
      `${jobUrl(jobPath)}/${encodeURIComponent(buildNo)}/api/json`,
    )) as RawBuild;
    return toRunInfo(jobPath, raw);
  }

  async viewStatus(jobPath: string, buildNo: string): Promise<RunStatus> {
    const info = await this.viewRun(jobPath, buildNo);
    return {
      job: info.job,
      build: info.build,
      result: info.result,
      building: info.building,
      durationMs: info.durationMs,
      timestamp: info.timestamp,
      url: info.url,
      displayName: info.displayName,
    };
  }

  async readLog(jobPath: string, buildNo: string, tail?: number): Promise<string> {
    const text = await this.readText(
      `${jobUrl(jobPath)}/${encodeURIComponent(buildNo)}/consoleText`,
    );
    if (tail === undefined) {
      return text;
    }
    const lines = text.split("\n");
    const sliced = lines.slice(-tail).join("\n");
    return text.endsWith("\n") ? `${sliced}\n` : sliced;
  }

  async startRun(jobPath: string): Promise<TriggerOut> {
    rejectMainline(jobPath, "start");
    return this.postBuild(jobPath, "/build");
  }

  async rerun(jobPath: string, buildNo: string): Promise<TriggerOut> {
    rejectMainline(jobPath, "rerun");
    const info = await this.viewRun(jobPath, buildNo);
    if (Object.keys(info.params).length === 0) {
      return this.postBuild(jobPath, "/build");
    }
    return this.postBuild(jobPath, "/buildWithParameters", info.params);
  }

  async cancelRun(jobPath: string, buildNo: string): Promise<void> {
    const resp = await this.request(
      "POST",
      `${jobUrl(jobPath)}/${encodeURIComponent(buildNo)}/stop`,
      { withCrumb: true },
    );
    if (resp.status === 404) {
      throw new JenkinsErr(`Build not found: ${jobPath} #${buildNo}`, 2);
    }
    if (!resp.ok && resp.status !== 302) {
      throw new JenkinsErr(`cancel failed: ${resp.status} ${resp.statusText}`, 3);
    }
  }

  async listQueue(): Promise<QueueItem[]> {
    const data = await this.readJson(
      "/queue/api/json",
      "tree=items[id,task[name,url],why,inQueueSince]",
    );
    return asArr(data.items).map((item) => {
      const raw = asRec(item);
      const task = asRec(raw.task);
      return {
        id: typeof raw.id === "number" ? raw.id : 0,
        name: asStr(task.name),
        url: asStr(task.url),
        why: asStr(raw.why),
        inQueueSince: typeof raw.inQueueSince === "number" ? raw.inQueueSince : 0,
      };
    });
  }

  private async postBuild(
    jobPath: string,
    suffix: string,
    params?: Record<string, string>,
  ): Promise<TriggerOut> {
    const body = params === undefined ? undefined : new URLSearchParams(params);
    const resp = await this.request("POST", `${jobUrl(jobPath)}${suffix}`, {
      withCrumb: true,
      body,
    });
    if (resp.status === 404) {
      throw new JenkinsErr(`Job not found: ${jobPath}`, 2);
    }
    if (resp.status !== 201 && resp.status !== 200 && resp.status !== 302) {
      const text = await resp.text().catch(() => "");
      throw new JenkinsErr(
        `Trigger failed: ${resp.status} ${resp.statusText}${text === "" ? "" : `\n${text.slice(0, 500)}`}`,
        3,
      );
    }
    return {
      job: jobPath,
      triggered: true,
      status: resp.status,
      queueLocation: resp.headers.get("Location"),
    };
  }

  private async readJson(path: string, tree?: string): Promise<Record<string, unknown>> {
    const qs = tree === undefined ? "" : `?${tree.startsWith("tree=") ? tree : `tree=${tree}`}`;
    const resp = await this.request("GET", `${path}${qs}`);
    this.throwHttp(resp, path);
    return (await resp.json()) as Record<string, unknown>;
  }

  private async readText(path: string): Promise<string> {
    const resp = await this.request("GET", path);
    this.throwHttp(resp, path);
    return await resp.text();
  }

  private throwHttp(resp: Response, path: string): void {
    if (resp.status === 404) {
      throw new JenkinsErr(`not found: ${path}`, 2);
    }
    if (!resp.ok) {
      throw new JenkinsErr(`Jenkins API error: ${resp.status} ${resp.statusText}`, 3);
    }
  }

  private async request(
    method: string,
    path: string,
    opts: { withCrumb?: boolean; body?: URLSearchParams } = {},
  ): Promise<Response> {
    const headers = new Headers();
    const token = Buffer.from(
      `${this.runtime.profile.username}:${this.runtime.secret}`,
    ).toString("base64");
    headers.set("Authorization", `Basic ${token}`);
    if (opts.body !== undefined) {
      headers.set("Content-Type", "application/x-www-form-urlencoded");
    }
    if (opts.withCrumb === true) {
      const crumb = await this.loadCrumb();
      if (crumb !== undefined) {
        headers.set(crumb.key, crumb.value);
      }
    }
    return this.http(`${this.runtime.profile.url}${path}`, {
      method,
      headers,
      body: opts.body,
      redirect: "manual",
    });
  }

  private async loadCrumb(): Promise<Crumb | undefined> {
    if (this.crumbTried) {
      return this.crumb;
    }
    this.crumbTried = true;
    const resp = await this.request("GET", "/crumbIssuer/api/json");
    if (resp.status === 404) {
      return undefined;
    }
    if (!resp.ok) {
      throw new JenkinsErr(`crumb failed: ${resp.status} ${resp.statusText}`, 3);
    }
    const data = (await resp.json()) as { crumb?: string; crumbRequestField?: string };
    if (data.crumb && data.crumbRequestField) {
      this.crumb = { key: data.crumbRequestField, value: data.crumb };
    }
    return this.crumb;
  }
}

function rejectMainline(jobPath: string, verb: string): void {
  if (isMainline(jobPath)) {
    throw new JenkinsErr(`refusing to ${verb} mainline job: ${jobPath}`, 1);
  }
}

function toRunInfo(jobPath: string, raw: RawBuild): RunInfo {
  const causes: string[] = [];
  for (const action of raw.actions ?? []) {
    const list = action.causes;
    if (!Array.isArray(list)) {
      continue;
    }
    for (const item of list) {
      const rec = asRec(item);
      if (typeof rec.shortDescription === "string") {
        causes.push(rec.shortDescription);
      }
    }
  }
  return {
    job: jobPath,
    build: raw.number ?? 0,
    result: runResult(raw.building === true, raw.result ?? null),
    building: raw.building === true,
    durationMs: raw.duration ?? 0,
    timestamp: raw.timestamp ?? 0,
    url: raw.url ?? "",
    displayName: raw.displayName ?? raw.fullDisplayName,
    estimatedDurationMs: raw.estimatedDuration ?? 0,
    description: raw.description ?? null,
    artifacts: (raw.artifacts ?? []).map((item) => ({
      name: item.fileName ?? "",
      path: item.relativePath ?? "",
    })),
    causes,
    params: collectParams(raw.actions ?? []),
  };
}

function collectParams(actions: Array<Record<string, unknown>>): Record<string, string> {
  const params: Record<string, string> = {};
  for (const action of actions) {
    const list = action.parameters;
    if (!Array.isArray(list)) {
      continue;
    }
    for (const item of list) {
      const rec = asRec(item);
      const name = asStr(rec.name).trim();
      if (name === "" || rec.value === undefined || rec.value === null) {
        continue;
      }
      params[name] = String(rec.value);
    }
  }
  return params;
}

function asRec(value: unknown): Record<string, unknown> {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

function asArr(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function asStr(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function asNum(value: unknown): number | null {
  return typeof value === "number" ? value : null;
}
