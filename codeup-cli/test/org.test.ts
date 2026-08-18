import { expect, test } from "bun:test";
import { encodeRepoId, parseGitRemote, parseOrgId, repoFromRemotePath } from "../src/org";

test("parseOrgId 只接受 Organization 根", () => {
  expect(parseOrgId("https://codeup.aliyun.com/org-example")).toBe("org-example");
  expect(() => parseOrgId("https://codeup.aliyun.com/org-example/group/repo")).toThrow(
    "organization url must be",
  );
  expect(() => parseOrgId("https://example.com/org-example")).toThrow("host must be");
  expect(() => parseOrgId("https://codeup.aliyun.com/<organizationId>")).toThrow("missing");
});

test("encodeRepoId 补 org 并编码斜杠", () => {
  expect(encodeRepoId("12", "org-example")).toBe("12");
  expect(encodeRepoId("group/demo", "org-example")).toBe("org-example%2Fgroup%2Fdemo");
  expect(encodeRepoId("org-example/group/demo", "org-example")).toBe("org-example%2Fgroup%2Fdemo");
  expect(encodeRepoId("demo", "org-example")).toBe("org-example%2Fdemo");
});

test("repoFromRemotePath 去掉 org 前缀", () => {
  expect(repoFromRemotePath("org-example/group/demo", "org-example")).toBe("group/demo");
  expect(repoFromRemotePath("group/demo", "org-example")).toBe("group/demo");
  const ssh = parseGitRemote("git@codeup.aliyun.com:org-example/group/demo.git");
  expect(ssh.host).toBe("codeup.aliyun.com");
  expect(ssh.path).toBe("org-example/group/demo");
});
