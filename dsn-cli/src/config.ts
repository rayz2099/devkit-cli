import { join } from "node:path";
import { KINDS, type Access, type FileCfg, type Kind, type Profile, type Runtime } from "./types";
import type { Audience } from "./types";

export const DEFAULT_CFG_REL = ".config/dsn-cli/config.json";

/** 为什么: 本机已有 trailing comma 的手写文件, 不能逼用户先学严格 JSON. */
export function stripJsonc(text: string): string {
  let out = "";
  let index = 0;
  let inStr = false;
  let escape = false;
  while (index < text.length) {
    const ch = text[index] ?? "";
    const next = text[index + 1] ?? "";
    if (inStr) {
      out += ch;
      if (escape) {
        escape = false;
      } else if (ch === "\\") {
        escape = true;
      } else if (ch === "\"") {
        inStr = false;
      }
      index += 1;
      continue;
    }
    if (ch === "\"") {
      inStr = true;
      out += ch;
      index += 1;
      continue;
    }
    if (ch === "/" && next === "/") {
      index += 2;
      while (index < text.length && text[index] !== "\n") {
        index += 1;
      }
      continue;
    }
    if (ch === "/" && next === "*") {
      index += 2;
      while (index < text.length && !(text[index] === "*" && text[index + 1] === "/")) {
        index += 1;
      }
      index += 2;
      continue;
    }
    if (ch === ",") {
      let look = index + 1;
      while (look < text.length && /\s/.test(text[look] ?? "")) {
        look += 1;
      }
      const end = text[look] ?? "";
      if (end === "}" || end === "]") {
        index += 1;
        continue;
      }
    }
    out += ch;
    index += 1;
  }
  return out;
}

export function cfgPath(home = process.env.HOME): string {
  if (home === undefined || home === "") {
    throw new Error("HOME is required");
  }
  return join(home, DEFAULT_CFG_REL);
}

/** 为什么: 配置是唯一连接真相, 缺 kind/url 必须失败, 不能默默补一个 mysql. */
export function parseFileCfg(content: string, path: string): FileCfg {
  let parsed: unknown;
  try {
    parsed = JSON.parse(stripJsonc(content));
  } catch {
    throw new Error(`invalid config file ${path}`);
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(`invalid config file ${path}`);
  }
  const raw = parsed as Record<string, unknown>;
  if (!Array.isArray(raw.profiles)) {
    throw new Error(`invalid config file ${path}: profiles must be an array`);
  }
  return {
    // 为什么: 未知 kind 或单条坏配置不能挡其它 Profile, 否则 kafka 用不了.
    profiles: raw.profiles.flatMap((item, index) => {
      try {
        return [parseProfile(item, path, index)];
      } catch {
        return [];
      }
    }),
  };
}

function parseProfile(item: unknown, path: string, index: number): Profile {
  if (!item || typeof item !== "object" || Array.isArray(item)) {
    throw new Error(`invalid config file ${path}: profiles[${index}] must be an object`);
  }
  const raw = item as Record<string, unknown>;
  const name = readStr(raw.name);
  if (name === "") {
    throw new Error(`invalid config file ${path}: profiles[${index}].name is required`);
  }
  const kind = readKind(raw.kind, path, name);
  const url = readStr(raw.url);
  if (url === "") {
    throw new Error(`invalid config file ${path}: url is required for profile ${name}`);
  }
  assertUrl(kind, url, name);
  return {
    name,
    kind,
    url,
    access: readAccess(raw.access, path, name),
  };
}

function readKind(value: unknown, path: string, name: string): Kind {
  if (typeof value !== "string") {
    throw new Error(`invalid config file ${path}: kind is required for profile ${name}`);
  }
  if (!(KINDS as string[]).includes(value)) {
    throw new Error(`invalid config file ${path}: unknown kind ${value} for profile ${name}`);
  }
  return value as Kind;
}

function readAccess(value: unknown, path: string, name: string): Access {
  if (value === undefined) {
    return "read";
  }
  if (value === "read" || value === "write") {
    return value;
  }
  throw new Error(`invalid config file ${path}: access must be read or write for profile ${name}`);
}

/** 为什么: kind 和 URL scheme 必须对上, 否则 Driver 会连错协议还当配置合法. */
export function assertUrl(kind: Kind, url: string, name: string): void {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    throw new Error(`invalid url for profile ${name}`);
  }
  const proto = parsed.protocol;
  if (kind === "mysql" || kind === "doris") {
    if (proto !== "mysql:") {
      throw new Error(`profile ${name}: ${kind} url must use mysql://`);
    }
    return;
  }
  if (kind === "redis") {
    if (proto !== "redis:" && proto !== "rediss:") {
      throw new Error(`profile ${name}: redis url must use redis:// or rediss://`);
    }
    return;
  }
  if (kind === "mongodb") {
    if (proto !== "mongodb:" && proto !== "mongodb+srv:") {
      throw new Error(`profile ${name}: mongodb url must use mongodb://`);
    }
    return;
  }
  if (kind === "elasticsearch") {
    if (proto !== "http:" && proto !== "https:") {
      throw new Error(`profile ${name}: elasticsearch url must use http:// or https://`);
    }
    return;
  }
  if (kind === "postgres") {
    if (proto !== "postgres:" && proto !== "postgresql:") {
      throw new Error(`profile ${name}: postgres url must use postgres:// or postgresql://`);
    }
    return;
  }
  if (kind === "kafka") {
    if (proto !== "kafka:" && proto !== "kafkas:") {
      throw new Error(`profile ${name}: kafka url must use kafka:// or kafkas://`);
    }
    return;
  }
  const _never: never = kind;
  void _never;
}

/** 为什么: 每次执行读盘, fish 补全和真实命令看到同一份 XDG 配置. */
export async function loadFileCfg(path?: string): Promise<FileCfg> {
  const resolved = path ?? cfgPath();
  const file = Bun.file(resolved);
  if (!(await file.exists())) {
    throw new Error(`config file not found: ${resolved}`);
  }
  return parseFileCfg(await file.text(), resolved);
}

export function pickProfile(fileCfg: FileCfg, name: string): Profile {
  const wanted = name.trim();
  const profile = fileCfg.profiles.find((item) => item.name === wanted);
  if (profile === undefined) {
    const names = fileCfg.profiles.map((item) => item.name).join(", ");
    throw new Error(
      `profile not found: ${wanted}${names === "" ? "" : `. available: ${names}`}`,
    );
  }
  return profile;
}

export function resolveRuntime(
  fileCfg: FileCfg,
  audience: Audience,
  profileName: string,
): Runtime {
  return {
    audience,
    profile: pickProfile(fileCfg, profileName),
  };
}

export function profileNames(fileCfg: FileCfg): string[] {
  return fileCfg.profiles.map((item) => item.name);
}

function readStr(value: unknown): string {
  return typeof value === "string" ? value : "";
}
