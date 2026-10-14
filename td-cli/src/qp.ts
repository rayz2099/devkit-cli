import { isObj } from "./util";

export type Span = {
  recentDay: string;
  startTime: string;
  endTime: string;
};

const EVENT_DESC: Record<string, string> = {
  A100: "总次数",
  A101: "触发用户数",
  A102: "人均次数",
};

/** 为什么: 分组项要带表类型, 计算接口靠 tableType 区分事件属性和用户属性. */
export function parseGroup(raw: string, table: "0" | "1"): Record<string, string> {
  let kind = table;
  let name = raw;
  const idx = raw.indexOf(":");
  if (idx > 0) {
    const head = raw.slice(0, idx);
    name = raw.slice(idx + 1);
    if (head === "event") {
      kind = "0";
    } else if (head === "user") {
      kind = "1";
    } else {
      throw new Error(`unknown group table: ${head}`);
    }
  }
  if (name.trim() === "") {
    throw new Error("group name is empty");
  }
  return {
    columnName: name,
    columnDesc: name,
    tableType: kind,
    propertyRange: "",
  };
}

/** 为什么: 属性分析页的 QP 不是 Open API 形状, 计算走的是页面里的 events/eventView. */
export function propertyQp(input: {
  analysis: string;
  quota: string;
  group?: string;
  span?: Span;
}): Record<string, unknown> {
  const quota = input.quota;
  const event: Record<string, unknown> = {
    analysis: input.analysis,
    analysisDesc: input.analysis === "A101" ? "用户数" : input.analysis,
    eventNameDisplay: "",
    filts: [],
    quota,
    quotaDesc: "",
    relation: 1,
    tableType: "1",
  };
  const view: Record<string, unknown> = {
    graphShape: "L1",
    groupBy: input.group === undefined ? [] : [parseGroup(input.group, "1")],
    uiCommonConfig: uiCfg(),
    userCrowds: [],
  };
  if (input.span !== undefined) {
    view.recentDay = input.span.recentDay;
    view.startTime = input.span.startTime;
    view.endTime = input.span.endTime;
  }
  return { events: [event], eventView: view, origin: 1 };
}

/** 为什么: 事件分析默认口径和页面一致, anyEvent + 总次数 + 过去 7 天, 时间粒度 T1. */
export function eventQp(input: {
  eventName: string;
  analysis: string;
  span: Span;
  particle: string;
  group?: string;
}): Record<string, unknown> {
  const desc = EVENT_DESC[input.analysis] ?? input.analysis;
  const event = {
    analysis: input.analysis,
    analysisDesc: desc,
    eventName: input.eventName,
    eventDesc: input.eventName === "anyEvent" ? "任意事件" : input.eventName,
    eventType: "event",
    eventNameDisplay: "",
    filts: [],
    quota: "",
    quotaDesc: "",
    relation: 1,
    type: 0,
  };
  const view: Record<string, unknown> = {
    startTime: input.span.startTime,
    endTime: input.span.endTime,
    recentDay: input.span.recentDay,
    timeParticleSize: input.particle,
    graphShape: "L0",
    groupBy: input.group === undefined ? [] : [parseGroup(input.group, "0")],
    filts: [],
    relation: 1,
    comparedByTime: false,
    rowSpanType: "fold",
    uiCommonConfig: uiCfg(),
  };
  return { events: [event], eventView: view, origin: 1 };
}

/** 为什么: SQL 模型能改数, 统计命令只放行页面上的只读分析模型. */
export function assertReadModel(model: number): void {
  const ok = model === 0 || model === 1 || model === 2 || model === 3
    || model === 4 || model === 7 || model === 12 || model === 14;
  if (!ok) {
    throw new Error(`model ${model} is not a read-only statistic`);
  }
}

export function modelPrefix(model: number): string {
  const map: Record<number, string> = {
    0: "WS_EVENT",
    1: "WS_RETENTION",
    2: "WS_FUNNEL",
    3: "WS_PROPERTY",
    4: "WS_TRACE",
    7: "WS_SCATTER",
    12: "WS_INTERVAL",
    14: "WS_ATTRIBUTION",
  };
  const prefix = map[model];
  if (prefix === undefined) {
    throw new Error(`model ${model} is not a read-only statistic`);
  }
  return prefix;
}

/** 为什么: 报表重跑必须用报表自己的模型和 QP, 不能另造一套条件. */
export function takeReport(data: unknown, modelFlag: number | undefined): { model: number; qp: unknown } {
  if (!isObj(data)) {
    throw new Error("report has no qp");
  }
  const rawModel = data.reportModel ?? data.eventModel;
  const fromReport = typeof rawModel === "number" ? rawModel : undefined;
  if (fromReport !== undefined && modelFlag !== undefined && fromReport !== modelFlag) {
    throw new Error(`report model ${fromReport} does not match --model ${modelFlag}`);
  }
  const model = fromReport ?? modelFlag;
  if (model === undefined) {
    throw new Error("report has no reportModel");
  }
  assertReadModel(model);
  return { model, qp: takeQp(data) };
}

function takeQp(data: Record<string, unknown>): unknown {
  if (typeof data.qp === "string") {
    return JSON.parse(data.qp) as unknown;
  }
  if (isObj(data.qp)) {
    return data.qp;
  }
  if (data.events !== undefined && data.eventView !== undefined) {
    return {
      events: data.events,
      eventView: data.eventView,
      origin: data.origin ?? 1,
    };
  }
  throw new Error("report has no qp");
}

function uiCfg(): string {
  return JSON.stringify({
    tableSorts: [],
    chartSort: "num-desc",
    showChartLabel: false,
    showChartPercent: true,
    topNOptions: 10,
  });
}
