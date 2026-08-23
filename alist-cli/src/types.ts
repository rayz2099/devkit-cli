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

export type Runtime = {
  audience: Audience;
  profile: Profile;
};

export type FsEntry = {
  name: string;
  size: number;
  isDir: boolean;
  modified: string;
  sign: string;
  thumb: string;
  type: number;
};

export type PutTask = {
  id: string;
  name: string;
  progress: number;
  state: number;
  status: string;
  error: string;
};

export type PutOut = {
  src: string;
  dst: string;
  task?: PutTask;
};

export type SyncOut = {
  dst: string;
  count: number;
  files: PutOut[];
};

export type ListQuery = {
  path: string;
  password: string;
  page: number;
  size: number;
  refresh: boolean;
};

export type CliCmd =
  | { kind: "help"; topic?: string }
  | { kind: "completion-fish" }
  | { kind: "complete"; tokens: string[]; current: string }
  | {
      kind: "ls";
      audience: Audience;
      profile?: string;
      path: string;
      password: string;
      page: number;
      size: number;
      refresh: boolean;
    }
  | {
      kind: "put";
      audience: Audience;
      profile?: string;
      src: string;
      dst: string;
      asTask: boolean;
      password: string;
    }
  | {
      kind: "mkdir";
      audience: Audience;
      profile?: string;
      path: string;
      password: string;
    }
  | {
      kind: "sync";
      audience: Audience;
      profile?: string;
      src: string;
      dst: string;
      asTask: boolean;
      password: string;
    };

/** 为什么: 进程码要和 jenkins-cli 对齐, 调用方靠数字区分用法错误和 API 错误. */
export class AlistErr extends Error {
  constructor(
    message: string,
    readonly code: number,
  ) {
    super(message);
  }
}
