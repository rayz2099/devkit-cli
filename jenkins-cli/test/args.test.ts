import { expect, test } from "bun:test";
import { parseArgs } from "../src/args";

test("默认 human, agent 是前缀", () => {
  const human = parseArgs(["run", "ls", "dt-vrec/feature/x"]);
  expect(human).toMatchObject({ kind: "run-ls", audience: "human", jobPath: "dt-vrec/feature/x" });
  const agent = parseArgs(["agent", "trigger", "dt-vrec/feature/x"]);
  expect(agent).toMatchObject({ kind: "run-start", audience: "agent", jobPath: "dt-vrec/feature/x" });
});

test("human 没有旧动词", () => {
  expect(() => parseArgs(["trigger", "dt-vrec/feature/x"])).toThrow("unknown command");
});

test("status 默认 lastBuild", () => {
  expect(parseArgs(["agent", "status", "dt-vrec/feature/x"])).toMatchObject({
    kind: "run-view",
    slim: true,
    buildNo: "lastBuild",
  });
});

test("-p 可插在前面", () => {
  expect(parseArgs(["-p", "prod", "job", "ls"])).toMatchObject({
    kind: "job-ls",
    profile: "prod",
    folder: "",
  });
});
