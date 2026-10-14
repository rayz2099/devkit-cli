import { readFileSync } from "node:fs";
import { join } from "node:path";
import type { FileCfg, Profile } from "./types";
import { isObj } from "./util";

export const DEFAULT_CFG_REL = ".config/td-cli/config.json";

/** 为什么: 手写配置常带注释和尾逗号, 严格 JSON.parse 会把能用的文件打成无效. */
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

/** 为什么: 连接只认 profile 里的 url、username、password, 配置里不留令牌. */
export function parseFileCfg(content: string, path: string): FileCfg {
  let parsed: unknown;
  try {
    parsed = JSON.parse(stripJsonc(content));
  } catch {
    throw new Error(`invalid config file ${path}`);
  }
  if (!isObj(parsed)) {
    throw new Error(`invalid config file ${path}`);
  }
  if (typeof parsed.defaultProfile !== "string" || parsed.defaultProfile.trim() === "") {
    throw new Error(`invalid config file ${path}: defaultProfile is required`);
  }
  if (!Array.isArray(parsed.profiles)) {
    throw new Error(`invalid config file ${path}: profiles must be an array`);
  }
  return {
    defaultProfile: parsed.defaultProfile.trim(),
    profiles: parsed.profiles.map((item, index) => parseProfile(item, path, index)),
  };
}

export function loadFileCfg(path: string): FileCfg {
  let text: string;
  try {
    text = readFileSync(path, "utf8");
  } catch {
    throw new Error(`config not found: ${path}`);
  }
  return parseFileCfg(text, path);
}

export function pickProfile(file: FileCfg, name: string | undefined): Profile {
  const want = name === undefined || name === "" ? file.defaultProfile : name;
  const found = file.profiles.find((item) => item.name === want);
  if (found === undefined) {
    throw new Error(`profile not found: ${want}`);
  }
  return found;
}

export function profileNames(file: FileCfg): string[] {
  return file.profiles.map((item) => item.name);
}

function parseProfile(item: unknown, path: string, index: number): Profile {
  if (!isObj(item)) {
    throw new Error(`invalid config file ${path}: profiles[${index}] must be an object`);
  }
  const name = needStr(item.name, path, `profiles[${index}].name`);
  const url = needStr(item.url, path, `profiles[${index}].url`);
  const username = readStr(item.username, path, `profiles[${index}].username`);
  const password = readStr(item.password, path, `profiles[${index}].password`);
  return { name, url, username, password };
}

function needStr(value: unknown, path: string, field: string): string {
  if (typeof value !== "string" || value.trim() === "") {
    throw new Error(`invalid config file ${path}: ${field} is required`);
  }
  return value.trim();
}

function readStr(value: unknown, path: string, field: string): string {
  if (typeof value !== "string") {
    throw new Error(`invalid config file ${path}: ${field} is required`);
  }
  return value;
}
