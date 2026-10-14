import { isObj, asText, originOf } from "./util";

export type HttpOpt = {
  host: string;
  token: string;
};

/** 为什么: 页面把 return_code 非 0 当业务失败, HTTP 401 则是令牌无效, 两者不能混成空结果. */
export function unwrap(status: number, body: unknown): unknown {
  if (status === 401 || status === 403) {
    throw new Error("unauthorized");
  }
  if (status < 200 || status >= 300) {
    throw new Error(httpMsg(status, body));
  }
  if (isObj(body) && "return_code" in body) {
    const code = body.return_code;
    if (code !== 0 && code !== "0") {
      const message = asText(body.return_message);
      throw new Error(message === "" ? `return_code ${String(code)}` : message);
    }
    return "data" in body ? body.data : body;
  }
  return body;
}

/** 为什么: 登录接口吃表单, 查询接口吃 JSON, 两种 Content-Type 不能混用. */
export async function apiSend(input: {
  url: string;
  method: "GET" | "POST";
  token: string;
  json?: unknown;
  form?: URLSearchParams;
}): Promise<unknown> {
  const headers = new Headers();
  headers.set("Accept", "application/json");
  headers.set("X-Requested-With", "XMLHttpRequest");
  headers.set("Accept-Language", "zh-CN");
  if (input.token !== "") {
    headers.set("Authorization", `bearer ${input.token}`);
  }
  let body: string | undefined;
  if (input.form !== undefined) {
    headers.set("Content-Type", "application/x-www-form-urlencoded");
    body = input.form.toString();
  } else if (input.json !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(input.json);
  }
  const res = await fetch(input.url, { method: input.method, headers, body });
  const text = await res.text();
  const parsed = parseBody(text);
  return unwrap(res.status, parsed);
}

export function taRoot(host: string, bff: boolean): string {
  const origin = originOf(host);
  return bff ? `${origin}/bff/v1/ta` : `${origin}/v1/ta`;
}

export function withQuery(url: string, query: Record<string, string>, stamp: boolean): string {
  const u = new URL(url);
  for (const [key, value] of Object.entries(query)) {
    u.searchParams.set(key, value);
  }
  if (stamp) {
    u.searchParams.set("@t", String(Date.now()));
  }
  return u.toString();
}

function parseBody(text: string): unknown {
  if (text.trim() === "") {
    return {};
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return { return_message: text };
  }
}

function httpMsg(status: number, body: unknown): string {
  if (isObj(body)) {
    const message = asText(body.return_message) || asText(body.error) || asText(body.message);
    if (message !== "") {
      return message;
    }
  }
  return `http ${status}`;
}
