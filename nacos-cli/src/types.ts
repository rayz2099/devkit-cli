export type OutputFmt = "text" | "json";

export type FileConfig = {
  serverAddr: string;
  username: string;
  password: string;
  namespace: string;
  namespaces: string[];
  output: string;
};

export type Runtime = {
  serverAddr: string;
  username: string;
  password: string;
  namespace: string;
  output: OutputFmt;
  dev: boolean;
};

export type GlobalFlags = {
  serverAddr?: string;
  username?: string;
  password?: string;
  namespace?: string;
  output?: OutputFmt;
  dev: boolean;
};

export type ServerEp = {
  scheme: "http" | "https";
  host: string;
  port: number;
};

export type ConfigItem = {
  dataId: string;
  group: string;
  content?: string;
};

export type ConfigPage = {
  totalCount: number;
  pageNumber: number;
  pagesAvailable: number;
  pageItems: ConfigItem[];
};

export type NamingInst = {
  ip: string;
  port: number;
  weight?: number;
  healthy?: boolean;
  enabled?: boolean;
  ephemeral?: boolean;
  clusterName?: string;
  serviceName?: string;
  metadata?: Record<string, string>;
};

export type SearchMode = "accurate" | "blur";

export type CliCmd =
  | { kind: "help"; topic?: string }
  | { kind: "completion-fish" }
  | { kind: "fish-namespaces"; prefix: string }
  | { kind: "fish-data-ids"; prefix: string; global: GlobalFlags }
  | { kind: "fish-groups"; dataId: string; prefix: string; global: GlobalFlags }
  | {
      kind: "complete";
      tokens: string[];
      current: string;
      global: GlobalFlags;
    }
  | {
      kind: "config-get";
      dataId: string;
      group: string;
      global: GlobalFlags;
    }
  | {
      kind: "config-put";
      dataId: string;
      group: string;
      content: string;
      global: GlobalFlags;
    }
  | {
      kind: "config-delete";
      dataId: string;
      group: string;
      global: GlobalFlags;
    }
  | {
      kind: "config-list";
      search: SearchMode;
      dataId: string;
      group: string;
      pageNo: number;
      pageSize: number;
      global: GlobalFlags;
    }
  | {
      kind: "naming-register";
      service: string;
      ip: string;
      port: number;
      group: string;
      cluster: string;
      weight: number;
      ephemeral: boolean;
      global: GlobalFlags;
    }
  | {
      kind: "naming-deregister";
      service: string;
      ip: string;
      port: number;
      group: string;
      cluster: string;
      ephemeral: boolean;
      global: GlobalFlags;
    }
  | {
      kind: "naming-instances";
      service: string;
      ip?: never;
      group: string;
      clusters: string[];
      healthyOnly: boolean;
      global: GlobalFlags;
    };
