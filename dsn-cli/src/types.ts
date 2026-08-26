export type Audience = "human" | "agent";

export type Kind = "mysql" | "doris" | "redis" | "mongodb" | "elasticsearch";

export type Access = "read" | "write";

export type OutputFmt = "table" | "json" | "csv" | "plain";

export type Profile = {
  name: string;
  kind: Kind;
  url: string;
  access: Access;
};

export type FileCfg = {
  profiles: Profile[];
};

export type Runtime = {
  audience: Audience;
  profile: Profile;
};

/** 为什么: 没索引的查询会拖死 agent, 连接和执行必须分开掐秒, 不能共用一个超时. */
export type Timeouts = {
  connectMs: number;
  execMs: number;
};


export type QueryOut = {
  columns: string[];
  rows: Array<Record<string, unknown>>;
};

export type CliCmd =
  | { kind: "help"; topic?: string }
  | { kind: "completion-fish" }
  | { kind: "complete"; tokens: string[]; current: string }
  | {
      kind: "console";
      audience: Audience;
      profile: string;
    }
  | {
      kind: "query";
      audience: Audience;
      profile: string;
      stmt: string;
      output: OutputFmt;
      limit?: number;
      connectSec: number;
      execSec: number;
    };

export type RunOut =
  | { type: "stdout"; body: string }
  | { type: "exit"; code: number };

/** 为什么: agent 要靠退出码区分用法错误、门禁拒绝和驱动失败, 不能全挤进 1. */
export class DsnErr extends Error {
  constructor(
    message: string,
    readonly code: number,
  ) {
    super(message);
  }
}

export const KINDS: Kind[] = [
  "mysql",
  "doris",
  "redis",
  "mongodb",
  "elasticsearch",
];
