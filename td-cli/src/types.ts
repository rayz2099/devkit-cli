export type Audience = "human" | "agent";

export type Profile = {
  name: string;
  url: string;
  username: string;
  password: string;
};

export type FileCfg = {
  defaultProfile: string;
  profiles: Profile[];
};

export type Scope = {
  audience: Audience;
  profile?: string;
  config?: string;
};

export type CliCmd =
  | { kind: "help"; audience: Audience; topic?: string; config?: string }
  | { kind: "completion-fish" }
  | { kind: "complete"; tokens: string[]; current: string }
  | ({ kind: "auth-login" } & Scope)
  | ({ kind: "auth-status" } & Scope)
  | ({ kind: "project-ls" } & Scope)
  | ({ kind: "meta-events"; project: string } & Scope)
  | ({ kind: "meta-props"; project: string; event?: string } & Scope)
  | ({ kind: "report-ls"; project: string } & Scope)
  | ({ kind: "report-get"; project: string; reportId: string } & Scope)
  | ({
      kind: "query-property";
      project: string;
      analysis: string;
      quota: string;
      group?: string;
      recent?: string;
      from?: string;
      to?: string;
      qp?: string;
      timeoutMs: number;
    } & Scope)
  | ({
      kind: "query-event";
      project: string;
      eventName: string;
      analysis: string;
      recent?: string;
      from?: string;
      to?: string;
      particle: string;
      group?: string;
      qp?: string;
      timeoutMs: number;
    } & Scope)
  | ({
      kind: "query-run";
      project: string;
      model: number;
      qp: string;
      timeoutMs: number;
    } & Scope)
  | ({
      kind: "query-report";
      project: string;
      reportId: string;
      model?: number;
      timeoutMs: number;
    } & Scope);
