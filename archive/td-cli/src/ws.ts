import { buildFrame, killFrame, parseWs, requestId, wsUrl } from "./frame";

export type QueryIn = {
  host: string;
  token: string;
  bff: boolean;
  prefix: string;
  projectId: number;
  eventModel: number;
  qp: unknown;
  timeoutMs: number;
  onProgress?: (line: string) => void;
};

/** 为什么: 事件分析和属性分析的计算不在 REST 上返回, 页面把 QP 交给 websocket 再等进度到结果. */
export async function runQuery(input: QueryIn): Promise<unknown> {
  const id = requestId(input.prefix);
  const body = {
    projectId: input.projectId,
    eventModel: input.eventModel,
    qp: JSON.stringify(input.qp),
    searchSource: "model_search",
    querySource: "module",
  };
  const frame = buildFrame(id, body);
  const kill = killFrame(id);
  const url = wsUrl(input.host, input.token, input.bff);
  return await waitResult({
    url,
    frame,
    kill,
    id,
    timeoutMs: input.timeoutMs,
    onProgress: input.onProgress,
  });
}

type WaitIn = {
  url: string;
  frame: string;
  kill: string;
  id: string;
  timeoutMs: number;
  onProgress?: (line: string) => void;
};

function waitResult(input: WaitIn): Promise<unknown> {
  return new Promise((resolve, reject) => {
    let settled = false;
    const ws = new WebSocket(input.url);
    const timer = setTimeout(() => {
      sendKill();
      fail(new Error(`query timeout ${input.timeoutMs}ms`));
    }, input.timeoutMs);

    const finish = (value: unknown): void => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      ws.close();
      resolve(value);
    };
    const fail = (error: Error): void => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      ws.close();
      reject(error);
    };
    const sendKill = (): void => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(input.kill);
      }
    };

    ws.addEventListener("open", () => {
      ws.send(input.frame);
    });
    ws.addEventListener("message", (ev) => {
      const text = typeof ev.data === "string" ? ev.data : "";
      const msg = parseWs(text);
      if (msg.kind === "heartbeat") {
        ws.send("[\"heartbeat\", \"\"]");
        return;
      }
      if (msg.kind === "failed" && (msg.id === "" || msg.id === input.id)) {
        fail(new Error(msg.message));
        return;
      }
      if (msg.kind === "progress" && msg.id === input.id) {
        const line = `${msg.progress} ${msg.hint}`.trim();
        input.onProgress?.(line);
        return;
      }
      if (msg.kind === "done" && msg.id === input.id) {
        finish(msg.result);
      }
    });
    ws.addEventListener("error", () => {
      fail(new Error("websocket error"));
    });
    ws.addEventListener("close", () => {
      fail(new Error("websocket closed"));
    });
  });
}
