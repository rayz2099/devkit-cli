import { constants, publicEncrypt } from "node:crypto";

// 为什么: 这是 TE 4.4 前端写死的 RSA-1024 公钥, 登录接口只认这把钥匙加密的密码.
const PUB_BODY =
  "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCT+JeQB4oij4007Kslmma0sfFodahq1olOpWJoZR+aimyNuYUSTmbK1/bArGkIb0CxIPUGQIrARpY6apQNncORutdmBHonFkn7r7XpktL7bxZhDUKeLjIdjaEB1lh1tFxLl3cP0siHwuc1aAaEOWAjuCu2f9+FgY7WCEJla9Qa/QIDAQAB";

const PUB = `-----BEGIN PUBLIC KEY-----\n${PUB_BODY}\n-----END PUBLIC KEY-----\n`;

/** 为什么: 页面用 JSEncrypt 的 PKCS1 密文, Node 必须用同一填充, 否则 loginForToken 解不开. */
export function encryptPwd(plain: string): string {
  const buf = publicEncrypt(
    {
      key: PUB,
      padding: constants.RSA_PKCS1_PADDING,
    },
    Buffer.from(plain, "utf8"),
  );
  return buf.toString("base64");
}
