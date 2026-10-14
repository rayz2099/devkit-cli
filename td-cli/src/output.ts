import type { Audience } from "./types";
import { asText, isObj } from "./util";

/** 为什么: agent 要稳定 JSON, human 才用对齐文本扫读. */
export function emit(audience: Audience, data: unknown, text: string): string {
  if (audience === "agent") {
    return `${JSON.stringify({ ok: true, data }, null, 2)}\n`;
  }
  if (text === "" || text.endsWith("\n")) {
    return text;
  }
  return `${text}\n`;
}

export function errBody(message: string): string {
  return `${JSON.stringify({ ok: false, error: { type: errType(message), message } }, null, 2)}\n`;
}

export function renderTable(headers: string[], rows: string[][]): string {
  if (rows.length === 0) {
    return "";
  }
  const table = [headers, ...rows];
  const widths = headers.map((_, col) => Math.max(...table.map((row) => (row[col] ?? "").length)));
  return `${table
    .map((row) => row.map((cell, col) => (cell ?? "").padEnd(widths[col] ?? 0)).join("  "))
    .join("\n")}\n`;
}

/** 为什么: 目录接口把事件嵌在多层 catalog 里, human 只需要名字和说明. */
export function eventRows(data: unknown): string[][] {
  const seen = new Set<string>();
  const rows: string[][] = [];
  walk(data, (obj) => {
    const name = asText(obj.eventName);
    if (name === "" || seen.has(name)) {
      return;
    }
    seen.add(name);
    rows.push([name, asText(obj.eventDesc)]);
  });
  return rows;
}

/** 为什么: 属性接口字段不统一, quot 和 columnName 都表示属性名. */
export function propRows(data: unknown): string[][] {
  const rows: string[][] = [];
  walk(data, (obj) => {
    const name = asText(obj.quot) || asText(obj.columnName);
    if (name === "") {
      return;
    }
    const desc = asText(obj.quotDesc) || asText(obj.columnDesc);
    const table = asText(obj.tableType);
    rows.push([name, desc, table]);
  });
  return rows;
}

export function projectRows(data: unknown): string[][] {
  const rows: string[][] = [];
  const list = firstArray(data);
  for (const item of list) {
    if (!isObj(item)) {
      continue;
    }
    const id = asText(item.projectId) || asText(item.id);
    const name = asText(item.projectName) || asText(item.name);
    if (id === "" && name === "") {
      continue;
    }
    rows.push([id, name]);
  }
  return rows;
}

export function reportRows(data: unknown): string[][] {
  const rows: string[][] = [];
  const list = firstArray(data);
  for (const item of list) {
    if (!isObj(item)) {
      continue;
    }
    const id = asText(item.reportId) || asText(item.id);
    const name = asText(item.reportName) || asText(item.name);
    const model = asText(item.reportModel);
    if (id === "" && name === "") {
      continue;
    }
    rows.push([id, name, model]);
  }
  return rows;
}

/** 为什么: 计算结果的 x/y 是页面表格, 没有这层结构时保留原始 JSON. */
export function queryText(result: unknown): string {
  const table = resultTable(result);
  if (table !== "") {
    return table;
  }
  return `${JSON.stringify(result, null, 2)}\n`;
}

export function jsonText(data: unknown): string {
  return `${JSON.stringify(data, null, 2)}\n`;
}

function resultTable(result: unknown): string {
  const root = isObj(result) && isObj(result.data) ? result.data : result;
  if (!isObj(root) || !Array.isArray(root.x) || !Array.isArray(root.y)) {
    return "";
  }
  const headers = ["NAME", ...root.x.map((item) => asText(item))];
  const rows: string[][] = [];
  for (const item of root.y) {
    if (!isObj(item) || !Array.isArray(item.data)) {
      continue;
    }
    const name = asText(item.eventNameDisplay) || asText(item.eventName) || asText(item.name);
    const nums = item.data.map((cell) => asText(cell));
    rows.push([name, ...nums]);
  }
  if (rows.length === 0) {
    return "";
  }
  return renderTable(headers, rows);
}

function firstArray(data: unknown): unknown[] {
  if (Array.isArray(data)) {
    return data;
  }
  if (!isObj(data)) {
    return [];
  }
  for (const key of ["list", "data", "reports", "projects"]) {
    const value = data[key];
    if (Array.isArray(value)) {
      return value;
    }
  }
  return [];
}

function walk(data: unknown, visit: (obj: Record<string, unknown>) => void): void {
  if (Array.isArray(data)) {
    for (const item of data) {
      walk(item, visit);
    }
    return;
  }
  if (!isObj(data)) {
    return;
  }
  visit(data);
  for (const value of Object.values(data)) {
    walk(value, visit);
  }
}

function errType(message: string): string {
  if (message === "unauthorized" || message.includes("未登录")) {
    return "auth";
  }
  return "error";
}
