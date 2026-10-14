import { expect, test } from "bun:test";
import { buildFrame, killFrame, parseWs, wsUrl } from "../src/frame";

test("计算帧带 channel, 取消帧不带", () => {
  const frame = JSON.parse(buildFrame("WS_PROPERTY@@1", { projectId: 3, eventModel: 3 })) as unknown[];
  expect(frame[0]).toBe("data");
  expect(frame[1]).toMatchObject({ requestId: "WS_PROPERTY@@1", projectId: 3, eventModel: 3 });
  expect(frame[2]).toEqual({ channel: "ta" });
  const kill = JSON.parse(killFrame("WS_PROPERTY@@1")) as unknown[];
  expect(kill).toHaveLength(2);
  expect(kill[1]).toEqual({ requestId: "WS_PROPERTY@@1", queryStatus: "KILL" });
});

test("结果帧和失败帧按 requestId 解析", () => {
  const done = parseWs(JSON.stringify(["data", { requestId: "a", progress: 100, result: { x: [] } }, { channel: "ta" }]));
  expect(done).toEqual({ kind: "done", id: "a", result: { x: [] } });
  const failed = parseWs(JSON.stringify(["data", { status: "failed", errorMsg: "未登录" }]));
  expect(failed).toEqual({ kind: "failed", id: "", message: "未登录" });
  expect(parseWs("[\"heartbeat\", \"\"]")).toEqual({ kind: "heartbeat" });
});

test("websocket 地址跟 bff 开关走", () => {
  expect(wsUrl("http://10.0.0.1:8993", "tok", false)).toBe("ws://10.0.0.1:8993/v1/ta-websocket/query/tok");
  expect(wsUrl("https://te.example", "a b", true)).toBe("wss://te.example/bff/v1/ta-websocket/query?token=a%20b");
});
