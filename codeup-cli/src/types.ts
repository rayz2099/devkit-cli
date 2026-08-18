export type Audience = "human" | "agent";

export type Profile = {
  name: string;
  url: string;
  token: string;
};

export type FileCfg = {
  defaultProfile: string;
  profiles: Profile[];
};

export type Runtime = {
  audience: Audience;
  profile: Profile;
  orgId: string;
  token: string;
  apiBase: string;
};

export type Repo = {
  id: string;
  name: string;
  path: string;
  pathNs: string;
  webUrl: string;
  defBranch: string;
};

export type ChangeRequest = {
  localId: string;
  title: string;
  state: string;
  srcBranch: string;
  tgtBranch: string;
  author: string;
  createdAt: string;
  updatedAt: string;
  webUrl: string;
  crUrl: string;
  description: string;
};

export type Webhook = {
  id: string;
  url: string;
  secretToken: string;
  pushEvents: boolean;
  mergeRequestsEvents: boolean;
  tagPushEvents: boolean;
  noteEvents: boolean;
  createdAt: string;
  updatedAt: string;
};

export type PushOut = {
  remote: string;
  branch: string;
  pushed: true;
};

export type CliCmd =
  | { kind: "help"; topic?: string }
  | { kind: "completion-fish" }
  | { kind: "complete"; tokens: string[]; current: string }
  | {
      kind: "init";
      audience: Audience;
      profile?: string;
    }
  | {
      kind: "repos";
      audience: Audience;
      profile?: string;
      search?: string;
      page: number;
      perPage: number;
    }
  | {
      kind: "push";
      audience: Audience;
      profile?: string;
      remote: string;
      branch?: string;
    }
  | {
      kind: "cr-list";
      audience: Audience;
      profile?: string;
      repo?: string;
      state?: string;
      source?: string;
      target?: string;
      search?: string;
      page: number;
      perPage: number;
    }
  | {
      kind: "cr-get";
      audience: Audience;
      profile?: string;
      repo?: string;
      localId: string;
    }
  | {
      kind: "cr-create";
      audience: Audience;
      profile?: string;
      repo?: string;
      source: string;
      target?: string;
      title: string;
      body?: string;
      bodyFile?: string;
    }
  | {
      kind: "webhook-list";
      audience: Audience;
      profile?: string;
      repo?: string;
      showSecrets: boolean;
      page: number;
      perPage: number;
    };

/** 为什么: 进程码要和 jenkins-cli 对齐, 调用方靠数字区分用法错误和 API 错误. */
export class CodeupErr extends Error {
  constructor(
    message: string,
    readonly code: number,
  ) {
    super(message);
  }
}
