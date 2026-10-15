/** 为什么: 用法错误和 rclone 自己的失败必须是不同退出码, 调用方才知道有没有真正发起传输. */
export class RcloneErr extends Error {
  readonly code: number;

  constructor(code: number, message: string) {
    super(message);
    this.name = "RcloneErr";
    this.code = code;
  }
}

/** 为什么: 给人看的进度和给 agent 的 JSON 不能混在同一种输出里. */
export type Audience = "human" | "agent";

/**
 * 为什么: 本地目录树是同步的身份, 不能靠当前工作目录隐式充当源.
 * path 在配置里可以写 $HOME/..., 解析后必须是绝对路径.
 */
export type Source = {
  name: string;
  path: string;
};

/** 为什么: rclone crypt 内容加密只有 NaCl secretbox, 配置里必须写出来, 不能假装还能换算法. */
export type CryptAlgo = "secretbox";

/**
 * 为什么: 网盘上的目录名必须和 source 目录同名, 不能再套一层 photograph-crypt.
 * 多对密钥用来轮换: 加密只用 default, 解密按对重试. rclone 连接串仍写成 password / password2.
 */
export type Crypt = {
  name: string;
  key: string;
  salt: string;
  algorithm: CryptAlgo;
  default: boolean;
};

/** 为什么: 账号、加密和传输参数都挂在目标上, 任务只被允许覆盖少数几项. */
export type Target = {
  name: string;
  url: string;
  vendor: string;
  user: string;
  pass: string;
  root: string;
  skipLinks: boolean;
  skipHidden: boolean;
  sizeOnly: boolean;
  transfers: number;
  checkers: number;
  retries: number;
  retriesSleep: string;
  lowLevelRetries: number;
  timeout: string;
  contimeout: string;
  tpslimit: number;
  pacerMinSleep: string;
  encryptFilenames: boolean;
  crypt?: Crypt[];
};

/** 为什么: 文件名加密和内容加密不是一回事. 关掉文件名时目录名也必须关, rclone 不允许只加密目录. */
export type NameEnc = "standard" | "off";

/**
 * 为什么: 关文件名加密时 rclone 默认给密文加 .bin.
 * 新文件必须同名无后缀; .bin 只留给已经在网盘上的旧对象.
 */
export type CryptSuffix = "none" | ".bin";

/** 为什么: 任务是唯一可执行的边; 没名字就不能传输, 也不存在默认任务. */
export type Task = {
  name: string;
  source: string;
  target: string;
  excludes: string[];
  transfers?: number;
  tpslimit?: number;
  timeout?: string;
  encryptFilenames?: boolean;
  skipHidden?: boolean;
};

/** 为什么: 配置只有这三层. 多出来的 defaultTask 会让缺省命令偷偷开始上传. */
export type FileCfg = {
  rcloneBin: string;
  sources: Source[];
  targets: Target[];
  tasks: Task[];
};

/** 为什么: pull 只恢复 source 树. download/get 是单文件 copyto, 默认落到 cwd, 不和 pull 抢语义. */
export type Action = "push" | "pull" | "check" | "get" | "download";

/**
 * 为什么: 每次传输都要留下本地记录, 而且记录里不能出现账号和口令.
 * 失败和 dry-run 也要记, 否则中断后无法判断上次有没有跑完.
 */
export type RunRec = {
  id: string;
  task: string;
  action: Action;
  startedAt: string;
  finishedAt: string;
  code: number;
  local: string;
  remote: string;
  dryRun: boolean;
  args: string[];
  bytes?: number;
  checks?: number;
  transfers?: number;
  error?: string;
};

/** 为什么: obscure 和 copy 都要能换成测试桩, 单测不能打到真实网盘. */
export type SpawnRet = {
  code: number;
  stdout: string;
  stderr: string;
};

/** 为什么: 人读模式要把 rclone 的进度原样送进 stderr, agent 模式则必须丢掉进度条. */
export type ErrSink = {
  writeErr: (chunk: string) => void;
};

export type SpawnFn = (bin: string, args: string[], sink: ErrSink) => Promise<SpawnRet>;

export type CliCmd =
  | { kind: "help" }
  | { kind: "completion-fish" }
  | { kind: "complete"; tokens: string[]; current: string }
  | { kind: "usage"; detail?: string }
  | { kind: "ls"; audience: Audience; task?: string; path?: string; limit?: number }
  | { kind: "runs"; audience: Audience }
  | {
      kind: "xfer";
      action: Action;
      audience: Audience;
      task: string;
      path?: string;
      dest?: string;
      dryRun: boolean;
      verbose: boolean;
    };

/** 为什么: 传输失败仍要往 stdout 打摘要, 不能跟用法错误一样只走 stderr. */
export type CmdOut = {
  text: string;
  code: number;
};
