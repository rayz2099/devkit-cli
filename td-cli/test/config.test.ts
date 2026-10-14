import { expect, test } from "bun:test";
import { parseFileCfg } from "../src/config";

const raw = `{
  "defaultProfile": "local",
  "profiles": [
    {
      "name": "local",
      "url": "http://10.0.0.1:8993",
      "username": "ray",
      "password": "secret",
    }
  ]
}`;

test("配置是 profiles, 三项是 url username password", () => {
  const file = parseFileCfg(raw, "config.json");
  expect(file.profiles[0]).toEqual({
    name: "local",
    url: "http://10.0.0.1:8993",
    username: "ray",
    password: "secret",
  });
});

test("缺 url 或 username 直接失败", () => {
  expect(() => parseFileCfg(raw.replace("\"url\": \"http://10.0.0.1:8993\",", ""), "config.json"))
    .toThrow("url is required");
  expect(() => parseFileCfg(raw.replace("\"username\": \"ray\",", ""), "config.json"))
    .toThrow("username is required");
});
