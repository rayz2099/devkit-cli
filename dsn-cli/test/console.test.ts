import { expect, test } from "bun:test";
import { consoleSpec } from "../src/console";

test("mysql/doris 展开官方 mysql argv", () => {
  expect(consoleSpec("doris", "mysql://u:p@fe:9030/dw")).toEqual({
    bin: "mysql",
    args: ["-h", "fe", "-P", "9030", "-u", "u", "-pp", "dw"],
  });
});

test("postgres 把整段 url 交给 psql", () => {
  expect(
    consoleSpec("postgres", "postgres://u:p@127.0.0.1:5432/app?sslmode=require"),
  ).toEqual({
    bin: "psql",
    args: ["postgres://u:p@127.0.0.1:5432/app?sslmode=require"],
  });
});

test("redis / mongo 展开官方客户端", () => {
  expect(consoleSpec("redis", "redis://:s3cret@127.0.0.1:6379/2")).toEqual({
    bin: "redis-cli",
    args: ["-h", "127.0.0.1", "-p", "6379", "-a", "s3cret", "-n", "2"],
  });
  expect(consoleSpec("mongodb", "mongodb://127.0.0.1:27017/app")).toEqual({
    bin: "mongosh",
    args: ["mongodb://127.0.0.1:27017/app"],
  });
});

test("kafka / elasticsearch 没有 Console", () => {
  expect(() => consoleSpec("kafka", "kafka://127.0.0.1:9092")).toThrow("no Console");
  expect(() => consoleSpec("elasticsearch", "http://127.0.0.1:9200")).toThrow("no Console");
});

test("es 没有 Console", () => {
  expect(() => consoleSpec("elasticsearch", "https://127.0.0.1:9200")).toThrow("no Console");
});
