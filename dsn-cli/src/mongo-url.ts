/** 为什么: Mongo 副本集 seed list 是合法 DSN, WHATWG URL 不认逗号主机, 不能用 new URL 拆. */
const MONGO_URL =
  /^(mongodb(?:\+srv)?):\/\/(?:(?<username>[^:@]*)(?::(?<password>[^@]*))?@)?(?<hosts>(?!:)[^/?@]+)(?<rest>.*)$/;

type MongoParts = {
  hosts: string[];
  db: string;
};

function mongoParts(url: string): MongoParts | undefined {
  const matched = url.match(MONGO_URL);
  const groups = matched?.groups;
  if (groups === undefined) {
    return undefined;
  }
  const hostList = groups.hosts;
  if (hostList === undefined) {
    return undefined;
  }
  const hosts = hostList.split(",");
  if (hosts.length === 0 || hosts.some((item) => item === "")) {
    return undefined;
  }
  const rest = groups.rest ?? "";
  const noHash = rest.split("#")[0] ?? "";
  const qMark = noHash.indexOf("?");
  const path = qMark === -1 ? noHash : noHash.slice(0, qMark);
  return {
    hosts,
    db: decodeURIComponent(path.replace(/^\//, "")),
  };
}

export function mongoUrlOk(url: string): boolean {
  return mongoParts(url) !== undefined;
}

export function mongoDbName(url: string): string {
  const parts = mongoParts(url);
  if (parts === undefined) {
    throw new Error("invalid mongodb url");
  }
  return parts.db;
}
