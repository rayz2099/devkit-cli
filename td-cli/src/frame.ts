import { isObj, asText, originOf } from "./util";

export type WsMsg =
  | { kind: "heartbeat" }
  | { kind: "progress"; id: string; progress: number; hint: string }
  | { kind: "done"; id: string; result: unknown }
  | { kind: "failed"; id: string; message: string }
  | { kind: "ignore" };

/** 为什么: 计算请求在 websocket 上是三元组, 不是 HTTP body. 前缀只用于请求号, 路由看 eventModel. */
export function buildFrame(id: string, body: Record<string, unknown>): string {
  const data = { requestId: id, ...body };
  return JSON.stringify(["data", data, { channel: "ta" }]);
}

/** 为什么: 取消计算沿用页面的 KILL 帧, 超时后要把这次查询停掉. */
export function killFrame(id: string): string {
  return JSON.stringify(["data", { requestId: id, queryStatus: "KILL" }]);
}

export function requestId(prefix: string): string {
  return `${prefix}@@${crypto.randomUUID()}`;
}

/** 为什么: TE 4.4 把 accessToken 拼在 websocket 路径上, bff 开关会改这条路径. */
export function wsUrl(host: string, token: string, bff: boolean): string {
  const origin = new URL(originOf(host));
  const proto = origin.protocol === "https:" ? "wss:" : "ws:";
  const base = `${proto}//${origin.host}`;
  const enc = encodeURIComponent(token);
  if (bff) {
    return `${base}/bff/v1/ta-websocket/query?token=${enc}`;
  }
  return `${base}/v1/ta-websocket/query/${enc}`;
}

/** 为什么: 服务端把进度、结果和心跳都塞进同一个文本帧, CLI 只认自己发出的 requestId. */
export function parseWs(raw: string): WsMsg {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch {
    return { kind: "ignore" };
  }
  if (!Array.isArray(parsed)) {
    return { kind: "ignore" };
  }
  const type = parsed[0];
  if (type === "heartbeat") {
    return { kind: "heartbeat" };
  }
  if (type !== "data" || !isObj(parsed[1])) {
    return { kind: "ignore" };
  }
  const payload = parsed[1];
  const id = typeof payload.requestId === "string" ? payload.requestId : "";
  if (payload.status === "failed") {
    const message = asText(payload.errorMsg);
    return { kind: "failed", id, message: message === "" ? "query failed" : message };
  }
  if (payload.result !== undefined && id !== "") {
    return { kind: "done", id, result: payload.result };
  }
  if (typeof payload.progress === "number") {
    return {
      kind: "progress",
      id,
      progress: payload.progress,
      hint: asText(payload.hintMsg),
    };
  }
  return { kind: "ignore" };
}
