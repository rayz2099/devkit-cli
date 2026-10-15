import { expect, test } from "bun:test";
import { unwrap } from "../src/http";
import { encryptPwd } from "../src/rsa";

test("return_code 非 0 是业务失败", () => {
  expect(unwrap(200, { return_code: 0, data: { n: 1 } })).toEqual({ n: 1 });
  expect(() => unwrap(200, { return_code: -1, return_message: "bad qp" })).toThrow("bad qp");
});

test("401 是未登录, 不把 Spring 错误体当成查询结果", () => {
  expect(() => unwrap(401, { status: 401, error: "Unauthorized" })).toThrow("unauthorized");
});

test("密码密文是 PKCS1 base64, 不是明文", () => {
  const enc = encryptPwd("secret");
  expect(enc).not.toContain("secret");
  expect(enc.length).toBeGreaterThan(20);
  expect(Buffer.from(enc, "base64").length).toBeGreaterThan(0);
});
