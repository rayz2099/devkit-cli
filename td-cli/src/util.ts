/** 为什么: 服务端 JSON 层层包 data, 各命令要先确认是对象再取字段. */
export function isObj(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function asText(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return "";
}

/** 为什么: profile 的 url 可能带页面 hash, 接口只认 origin. */
export function originOf(host: string): string {
  const raw = host.includes("://") ? host : `http://${host}`;
  return new URL(raw).origin;
}

/** 为什么: 项目号和模型号必须是整数, 拼进查询体前就要拦住. */
export function asInt(raw: string, label: string): number {
  if (!/^-?\d+$/.test(raw)) {
    throw new Error(`${label} must be an integer: ${raw}`);
  }
  const value = Number(raw);
  if (!Number.isSafeInteger(value)) {
    throw new Error(`${label} must be an integer: ${raw}`);
  }
  return value;
}
