import { describe, expect, test } from "bun:test";
import {
  CLIENT_RESOLVE_MD_HREF,
  resolveMdHref,
} from "./serve-md";

const src = "docs/postgres/README.md";

describe("resolveMdHref", () => {
  test("目录页 README 相对链接绑到源文件目录", () => {
    expect(resolveMdHref(src, "architecture.md", "link")).toBe(
      "/docs/postgres/architecture.md",
    );
    expect(resolveMdHref(src, "./architecture.md", "link")).toBe(
      "/docs/postgres/architecture.md",
    );
    expect(resolveMdHref(src, "architecture.md#overview", "link")).toBe(
      "/docs/postgres/architecture.md#overview",
    );
  });

  test("相对图片走 raw, 避免被 SPA 当成文档页", () => {
    expect(resolveMdHref(src, "img/er.png", "image")).toBe(
      "/raw/docs/postgres/img/er.png",
    );
    expect(resolveMdHref(src, "../assets/x.png", "image")).toBe(
      "/raw/docs/assets/x.png",
    );
  });

  test("外链、锚点和已是站点路径的地址不改语义", () => {
    expect(resolveMdHref(src, "#install", "link")).toBe("#install");
    expect(resolveMdHref(src, "https://example.com/a.md", "link")).toBe(
      "https://example.com/a.md",
    );
    expect(resolveMdHref(src, "/docs/other.md", "link")).toBe("/docs/other.md");
    expect(resolveMdHref(src, "/raw/docs/x.png", "image")).toBe(
      "/raw/docs/x.png",
    );
  });

  test("越出仓库根的 .. 被夹住", () => {
    expect(resolveMdHref("README.md", "../../secret.md", "link")).toBe(
      "/secret.md",
    );
  });
});

describe("CLIENT_RESOLVE_MD_HREF", () => {
  test("浏览器脚本与服务端同一套规则", () => {
    const fn = new Function(`${CLIENT_RESOLVE_MD_HREF}; return resolveMdHref;`)() as (
      srcPath: string,
      href: string,
      kind: string,
    ) => string;
    expect(fn(src, "architecture.md", "link")).toBe(
      "/docs/postgres/architecture.md",
    );
    expect(fn(src, "img/er.png", "image")).toBe("/raw/docs/postgres/img/er.png");
    expect(fn(src, "#install", "link")).toBe("#install");
  });
});
