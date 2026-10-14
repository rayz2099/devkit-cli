import { describe, expect, test } from "bun:test";
import { CLIENT_CODE } from "./serve-ui-code";
import { CLIENT_PREVIEW } from "./serve-ui-preview";

describe("preview parsers", () => {
  test("CSV 引号和 JSONL 列并集", () => {
    const api = new Function(`${CLIENT_PREVIEW}; return { parseDelimited, parseJsonl };`)() as {
      parseDelimited: (text: string, sep: string) => { header: string[]; body: { cells: string[] }[] };
      parseJsonl: (text: string) => { header: string[]; body: { cells: string[] }[] };
    };
    const table = api.parseDelimited("name,qty\n\"a,b\",2\n", ",");
    expect(table.header).toEqual(["name", "qty"]);
    expect(table.body[0]?.cells).toEqual(["a,b", "2"]);

    const rows = api.parseJsonl('{"a":1}\n{"b":2}\nbad\n');
    expect(rows.header).toEqual(["a", "b", "_raw"]);
    expect(rows.body[2]?.cells).toEqual(["", "", "bad"]);
  });
});

describe("code highlight split", () => {
  test("跨行 span 在每一行闭合", () => {
    const api = new Function(`${CLIENT_CODE}; return { splitHlLines };`)() as {
      splitHlLines: (html: string) => string[];
    };
    const lines = api.splitHlLines('<span class="hljs-comment">/*\n still</span>');
    expect(lines[0]?.endsWith("</span>")).toBe(true);
    expect(lines[1]?.startsWith('<span class="hljs-comment">')).toBe(true);
  });
});
