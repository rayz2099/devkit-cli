import { join } from "node:path";
import { API_BASE, parseOrgId } from "./org";
import type { Audience, FileCfg, Profile, Runtime } from "./types";

export const DEFAULT_CFG_REL = ".config/codeup-cli/config.json";

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

/** 为什么: 配置是唯一连接真相, 缺字段必须失败, 不能默默补一个 Organization. */
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
  if (typeof raw.defaultProfile !== "string" || raw.defaultProfile.trim() === "") {
    throw new Error(`invalid config file ${path}: defaultProfile is required`);
  }
  if (!Array.isArray(raw.profiles)) {
    throw new Error(`invalid config file ${path}: profiles must be an array`);
  }
  return {
    defaultProfile: raw.defaultProfile.trim(),
    profiles: raw.profiles.map((item, index) => parseProfile(item, path, index)),
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
  const url = readStr(raw.url).replace(/\/+$/, "");
  if (url === "") {
    throw new Error(`invalid config file ${path}: url is required for profile ${name}`);
  }
  const token = readStr(raw.token);
  if (token === "") {
    throw new Error(`invalid config file ${path}: token is required for profile ${name}`);
  }
  return { name, url, token };
}

/** 为什么: 每次执行读盘, fish 补全和真实命令看到同一份 XDG 配置. */
export async function loadFileCfg(path = cfgPath()): Promise<FileCfg> {
  const file = Bun.file(path);
  if (!(await file.exists())) {
    throw new Error(`config file not found: ${path}`);
  }
  return parseFileCfg(await file.text(), path);
}

export function pickProfile(fileCfg: FileCfg, name?: string): Profile {
  const wanted = (name ?? fileCfg.defaultProfile).trim();
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
  profileName?: string,
): Runtime {
  const profile = pickProfile(fileCfg, profileName);
  return {
    audience,
    profile,
    orgId: parseOrgId(profile.url),
    token: profile.token,
    apiBase: API_BASE,
  };
}

export function profileNames(fileCfg: FileCfg): string[] {
  return fileCfg.profiles.map((item) => item.name);
}

function readStr(value: unknown): string {
  return typeof value === "string" ? value : "";
}
