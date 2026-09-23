import { RcloneErr } from "./types";
import type { CryptSuffix, NameEnc, Target } from "./types";

/**
 * 为什么: obscure 之后的值要原样放进连接串.
 * pass 是 WebDAV 口令. crypt 里的 password / password2 对应配置的 key / salt.
 */
export type CryptSecret = {
  name: string;
  password: string;
  password2: string;
};

export type Secrets = {
  pass: string;
  crypt?: CryptSecret[];
};

/** 为什么: 加密永远走 default 那一对, 解密才按配置顺序重试其它对. */
export function cryptOrder(target: Target): string[] {
  if (target.crypt === undefined) {
    return [];
  }
  const head = target.crypt.filter((item) => item.default).map((item) => item.name);
  const rest = target.crypt.filter((item) => !item.default).map((item) => item.name);
  return [...head, ...rest];
}

export type ParsedConn = {
  name: string;
  config: Record<string, string>;
  path: string;
};

/**
 * 为什么: 未加引号的值遇到 `:` 或 `,` 会被 rclone 截断.
 * 用户提供的字段一律加单引号, 引号本身按 rclone 的规则加倍.
 */
export function encVal(raw: string): string {
  return `'${raw.replaceAll("'", "''")}'`;
}

/**
 * 为什么: 网盘上看到的目录就是 source 的目录名, 也就是 root.
 * 加密时这个名字放在 webdav 路径上, rel 才是里面的子路径. 再套 crypt.remote 会变成 photograph-crypt.
 */
export function buildRemote(
  target: Target,
  secrets: Secrets,
  rel: string,
  nameEnc: NameEnc,
  suffix: CryptSuffix = "none",
  pair?: string,
): string {
  const folder = trimSlash(target.root);
  const sub = trimSlash(rel);
  if (target.crypt === undefined) {
    return `${webdavHead(target, secrets.pass)}${joinDest(folder, sub)}`;
  }
  const names = cryptOrder(target);
  const picked = pair ?? names[0];
  if (picked === undefined) {
    throw new RcloneErr(1, `targets.${target.name}.crypt requires a default key pair`);
  }
  const wire = secrets.crypt?.find((item) => item.name === picked);
  if (wire === undefined || wire.password === "" || wire.password2 === "") {
    throw new RcloneErr(1, `targets.${target.name}.crypt.${picked} requires key and salt`);
  }
  const web = `${webdavHead(target, secrets.pass)}${folder}`;
  const head = [
    ":crypt",
    `remote=${encVal(web)}`,
    ...nameFlags(nameEnc, suffix),
    `password=${encVal(wire.password)}`,
    `password2=${encVal(wire.password2)}`,
  ].join(",");
  if (sub === "") {
    return `${head}:`;
  }
  return `${head}:${sub}`;
}

/** 为什么: 密文在网盘上就是普通对象. 先用 WebDAV 拉一次, 换钥匙只在本地解, 避免同一文件下两遍. */
export function buildPlainRemote(target: Target, pass: string, rel: string): string {
  return `${webdavHead(target, pass)}${joinDest(trimSlash(target.root), trimSlash(rel))}`;
}

/**
 * 为什么: 本地临时目录里的文件名是我们起的, 必须关文件名加密, 否则 crypt 会去找密文文件名.
 */
export function buildLocalCrypt(folder: string, wire: CryptSecret, file: string): string {
  const head = [
    ":crypt",
    `remote=${encVal(folder)}`,
    "filename_encryption=off",
    "directory_name_encryption=false",
    `suffix=${encVal("none")}`,
    `password=${encVal(wire.password)}`,
    `password2=${encVal(wire.password2)}`,
  ].join(",");
  return `${head}:${file}`;
}

/**
 * 为什么: 关文件名加密时 rclone 默认 suffix=.bin, 网盘上会变成 filename.bin.
 * 新对象必须 suffix=none 才能和本地同名; .bin 只在 pull/get 读旧对象时打开.
 */
function nameFlags(nameEnc: NameEnc, suffix: CryptSuffix): string[] {
  if (nameEnc === "off") {
    return [
      "filename_encryption=off",
      "directory_name_encryption=false",
      `suffix=${encVal(suffix)}`,
    ];
  }
  return ["filename_encryption=standard", "directory_name_encryption=true"];
}

/** 为什么: 单测要证明特殊字符进了 argv 之后还能被 rclone 的连接串语法原样解出来. */
export function parseConn(input: string): ParsedConn {
  if (input === "") {
    throw new Error("connection string is empty");
  }
  let state = "name";
  let prev = 0;
  let param = "";
  let quote = "";
  let doubled = false;
  let name = "";
  const config: Record<string, string> = {};
  let index = 0;
  for (; index < input.length; index += 1) {
    const ch = input[index] ?? "";
    if (state === "name") {
      const step = takeName(input, index, ch);
      if (step.kind === "continue") {
        continue;
      }
      name = step.name;
      prev = index + 1;
      if (step.kind === "done") {
        state = "done";
        break;
      }
      state = "param";
      continue;
    }
    if (state === "param") {
      const step = takeParam(input, index, ch, prev);
      if (step.kind === "continue") {
        continue;
      }
      param = step.param;
      prev = index + 1;
      if (step.kind === "value") {
        state = "value";
        continue;
      }
      config[param] = "true";
      if (step.kind === "done") {
        state = "done";
        break;
      }
      state = "param";
      continue;
    }
    if (state === "value") {
      const step = takeValue(input, index, ch, prev, param, config);
      if (step.kind === "quoted") {
        quote = ch;
        state = "quoted";
        prev = index + 1;
        doubled = false;
        continue;
      }
      if (step.kind === "continue") {
        continue;
      }
      prev = index + 1;
      if (step.kind === "done") {
        state = "done";
        break;
      }
      state = "param";
      continue;
    }
    if (state === "quoted") {
      if (ch === quote) {
        state = "after";
      }
      continue;
    }
    const step = takeAfter(input, index, ch, prev, quote, doubled, param, config);
    if (step.kind === "quote") {
      state = "quoted";
      doubled = true;
      continue;
    }
    prev = index + 1;
    if (step.kind === "done") {
      state = "done";
      break;
    }
    state = "param";
  }
  if (state !== "done") {
    throw new Error(`connection string: truncated in ${state}`);
  }
  return {
    name,
    config,
    path: input.slice(index + 1),
  };
}

function webdavHead(target: Target, pass: string): string {
  const parts = [
    ":webdav",
    `url=${encVal(target.url)}`,
    `vendor=${encVal(target.vendor)}`,
    `user=${encVal(target.user)}`,
    `pass=${encVal(pass)}`,
    `pacer_min_sleep=${encVal(target.pacerMinSleep)}`,
  ];
  return `${parts.join(",")}:`;
}

function joinDest(root: string, rel: string): string {
  const base = trimSlash(root);
  const sub = trimSlash(rel);
  if (sub === "") {
    return base;
  }
  if (base === "") {
    return sub;
  }
  return `${base}/${sub}`;
}

function trimSlash(value: string): string {
  return value.replace(/^\/+|\/+$/g, "");
}

type NameStep =
  | { kind: "continue" }
  | { kind: "param"; name: string }
  | { kind: "done"; name: string };

function takeName(input: string, index: number, ch: string): NameStep {
  if (index === 0 && ch === ":") {
    return { kind: "continue" };
  }
  if (ch === "/" || ch === "\\") {
    throw new Error("connection string: invalid name");
  }
  if (ch !== ":" && ch !== ",") {
    return { kind: "continue" };
  }
  const name = input.slice(0, index);
  if (!/^:?[\w.+@-]+$/.test(name)) {
    throw new Error("connection string: invalid name");
  }
  if (ch === ":") {
    return { kind: "done", name };
  }
  return { kind: "param", name };
}

type ParamStep =
  | { kind: "continue" }
  | { kind: "value"; param: string }
  | { kind: "flag"; param: string }
  | { kind: "done"; param: string };

function takeParam(input: string, index: number, ch: string, prev: number): ParamStep {
  if (ch !== ":" && ch !== "," && ch !== "=") {
    if (!/[A-Za-z0-9_.]/.test(ch)) {
      throw new Error("connection string: bad param");
    }
    return { kind: "continue" };
  }
  const param = input.slice(prev, index);
  if (param === "" || !/^[A-Za-z0-9_.]+$/.test(param)) {
    throw new Error("connection string: bad param");
  }
  if (ch === "=") {
    return { kind: "value", param };
  }
  if (ch === ":") {
    return { kind: "done", param };
  }
  return { kind: "flag", param };
}

type ValueStep = { kind: "continue" } | { kind: "quoted" } | { kind: "param" } | { kind: "done" };

function takeValue(
  input: string,
  index: number,
  ch: string,
  prev: number,
  param: string,
  config: Record<string, string>,
): ValueStep {
  if ((ch === "'" || ch === "\"") && index === prev) {
    return { kind: "quoted" };
  }
  if (ch !== ":" && ch !== ",") {
    return { kind: "continue" };
  }
  config[param] = input.slice(prev, index);
  if (ch === ":") {
    return { kind: "done" };
  }
  return { kind: "param" };
}

function takeAfter(
  input: string,
  index: number,
  ch: string,
  prev: number,
  quote: string,
  doubled: boolean,
  param: string,
  config: Record<string, string>,
): { kind: "quote" } | { kind: "param" } | { kind: "done" } {
  if (ch === quote) {
    return { kind: "quote" };
  }
  if (ch !== ":" && ch !== ",") {
    throw new Error("connection string: trailing after quote");
  }
  let value = input.slice(prev, index - 1);
  if (doubled) {
    value = value.replaceAll(quote + quote, quote);
  }
  config[param] = value;
  if (ch === ":") {
    return { kind: "done" };
  }
  return { kind: "param" };
}
