import { join } from "node:path";
import type { Audience, FileCfg, Profile, Runtime } from "./types";

export const DEFAULT_CFG_REL = ".config/alist-cli/config.json";

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

/** 为什么: 连接三元组以 env 为准, 文件里允许空, 缺的字段留给 ALIST_*. */
export function emptyFileCfg(): FileCfg {
  return {
    defaultProfile: "",
    profiles: [],
  };
}

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
  return {
    name,
    url: stripSlash(readStr(raw.url)),
    username: readStr(raw.username),
    password: readStr(raw.password),
  };
}

/** 为什么: 每次执行读盘, fish 补全和真实命令看到同一份 XDG 配置. */
export async function loadFileCfg(path?: string): Promise<FileCfg> {
  const resolved = path ?? cfgPath();
  const file = Bun.file(resolved);
  if (!(await file.exists())) {
    if (path !== undefined) {
      throw new Error(`config file not found: ${resolved}`);
    }
    return emptyFileCfg();
  }
  return parseFileCfg(await file.text(), resolved);
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

/** 为什么: 旧 Go CLI 就是这 3 个 env, 优先级必须是 env > file, 否则 fish 里导出的 ALIST_* 会被 example 配置盖掉. */
export function resolveRuntime(
  fileCfg: FileCfg,
  audience: Audience,
  profileName?: string,
  env: NodeJS.ProcessEnv = process.env,
): Runtime {
  if (fileCfg.profiles.length === 0 && profileName !== undefined) {
    throw new Error(`profile not found: ${profileName}`);
  }
  const base = fileCfg.profiles.length === 0
    ? envProfile()
    : pickProfile(fileCfg, profileName);
  const profile: Profile = {
    name: base.name,
    url: stripSlash(readEnv(env, "ALIST_ADDRESS") ?? base.url),
    username: readEnv(env, "ALIST_USERNAME") ?? base.username,
    password: readEnv(env, "ALIST_PASSWORD") ?? base.password,
  };
  if (profile.url === "") {
    throw new Error("ALIST_ADDRESS is required");
  }
  if (profile.username === "") {
    throw new Error("ALIST_USERNAME is required");
  }
  if (profile.password === "") {
    throw new Error("ALIST_PASSWORD is required");
  }
  return { audience, profile };
}

export function profileNames(fileCfg: FileCfg): string[] {
  return fileCfg.profiles.map((item) => item.name);
}

function envProfile(): Profile {
  return {
    name: "env",
    url: "",
    username: "",
    password: "",
  };
}

function readEnv(env: NodeJS.ProcessEnv, key: string): string | undefined {
  return env[key];
}

function stripSlash(url: string): string {
  return url.replace(/\/+$/, "");
}

function readStr(value: unknown): string {
  return typeof value === "string" ? value : "";
}
