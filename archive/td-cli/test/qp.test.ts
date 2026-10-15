import { expect, test } from "bun:test";
import { assertReadModel, eventQp, propertyQp, takeReport } from "../src/qp";

test("属性分析默认是用户数", () => {
  const qp = propertyQp({ analysis: "A101", quota: "" });
  const events = qp.events as Array<Record<string, unknown>>;
  expect(events[0]?.analysis).toBe("A101");
  expect(events[0]?.tableType).toBe("1");
  expect(events[0]?.quota).toBe("");
});

test("事件分析 QP 带上展开后的时间", () => {
  const qp = eventQp({
    eventName: "login",
    analysis: "A101",
    particle: "T1",
    span: { recentDay: "1-1", startTime: "2026-10-13 00:00:00", endTime: "2026-10-13 23:59:59" },
  });
  const view = qp.eventView as Record<string, unknown>;
  expect(view.startTime).toBe("2026-10-13 00:00:00");
  expect(view.timeParticleSize).toBe("T1");
  const events = qp.events as Array<Record<string, unknown>>;
  expect(events[0]?.eventName).toBe("login");
  expect(events[0]?.analysisDesc).toBe("触发用户数");
});

test("SQL 模型不是只读统计", () => {
  expect(() => assertReadModel(10)).toThrow("not a read-only statistic");
  expect(() => assertReadModel(0)).not.toThrow();
});

test("报表重跑使用报表自己的模型和 QP", () => {
  const got = takeReport({
    reportModel: 3,
    events: [{ analysis: "A101" }],
    eventView: { graphShape: "L1" },
  }, undefined);
  expect(got.model).toBe(3);
  expect(got.qp).toMatchObject({ events: [{ analysis: "A101" }] });
});

test("报表模型冲突时失败", () => {
  expect(() => takeReport({ reportModel: 0, qp: { events: [] } }, 3)).toThrow("does not match");
});
