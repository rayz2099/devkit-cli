export type Audience = "human" | "agent";

export type Profile = {
  name: string;
  url: string;
  username: string;
  password: string;
  apiToken: string;
};

export type FileCfg = {
  defaultProfile: string;
  profiles: Profile[];
};

export type Runtime = {
  audience: Audience;
  profile: Profile;
  secret: string;
};

export type JobRef = {
  name: string;
  url: string;
  className: string;
};

export type JobView = {
  name: string;
  url: string;
  buildable: boolean;
  lastBuild: number | null;
  lastSuccessfulBuild: number | null;
  lastFailedBuild: number | null;
};

export type RunItem = {
  number: number;
  result: string;
  building: boolean;
  timestamp: number;
  durationMs: number;
};

export type RunStatus = {
  job: string;
  build: number;
  result: string;
  building: boolean;
  durationMs: number;
  timestamp: number;
  url: string;
  displayName?: string;
};

export type RunInfo = RunStatus & {
  estimatedDurationMs: number;
  description: string | null;
  artifacts: Array<{ name: string; path: string }>;
  causes: string[];
  params: Record<string, string>;
};

export type TriggerOut = {
  job: string;
  triggered: true;
  status: number;
  queueLocation: string | null;
};

export type QueueItem = {
  id: number;
  name: string;
  url: string;
  why: string;
  inQueueSince: number;
};

export type CancelOut = {
  jobPath: string;
  build: string;
  action: "stop";
  status: "requested";
};

export type CliCmd =
  | { kind: "help"; topic?: string }
  | { kind: "completion-fish" }
  | { kind: "complete"; tokens: string[]; current: string }
  | {
      kind: "job-ls";
      audience: Audience;
      profile?: string;
      folder: string;
    }
  | {
      kind: "job-view";
      audience: Audience;
      profile?: string;
      jobPath: string;
    }
  | {
      kind: "run-ls";
      audience: Audience;
      profile?: string;
      jobPath: string;
      limit: number;
    }
  | {
      kind: "run-view";
      audience: Audience;
      profile?: string;
      jobPath: string;
      buildNo: string;
      slim: boolean;
    }
  | {
      kind: "run-start";
      audience: Audience;
      profile?: string;
      jobPath: string;
    }
  | {
      kind: "run-cancel";
      audience: Audience;
      profile?: string;
      jobPath: string;
      buildNo: string;
    }
  | {
      kind: "run-rerun";
      audience: Audience;
      profile?: string;
      jobPath: string;
      buildNo: string;
    }
  | {
      kind: "log";
      audience: Audience;
      profile?: string;
      jobPath: string;
      buildNo: string;
      tail?: number;
    }
  | {
      kind: "queue-ls";
      audience: Audience;
      profile?: string;
    };

export class JenkinsErr extends Error {
  constructor(
    message: string,
    readonly code: number,
  ) {
    super(message);
  }
}
