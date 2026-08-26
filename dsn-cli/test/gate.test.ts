import { expect, test } from "bun:test";
import { gateQuery } from "../src/gate";
import { DsnErr } from "../src/types";

function blocked(kind: Parameters<typeof gateQuery>[0], stmt: string): string {
  try {
    gateQuery(kind, "read", stmt);
    throw new Error("expected reject");
  } catch (error) {
    expect(error).toBeInstanceOf(DsnErr);
    expect((error as DsnErr).code).toBe(2);
    return (error as DsnErr).message;
  }
}

test("SQL 只读头和注释剥离", () => {
  gateQuery("mysql", "read", "  /* c */ SELECT 1");
  gateQuery("mysql", "read", "-- hi\nSHOW TABLES");
  gateQuery("doris", "read", "SHOW BACKENDS");
  gateQuery("doris", "read", "SWITCH CATALOG hive");
});

test("SQL 未知头和写尾巴", () => {
  expect(blocked("mysql", "INSERT INTO t VALUES (1)")).toContain("INSERT");
  expect(blocked("mysql", "SELECT * FROM t FOR UPDATE")).toContain("FOR UPDATE");
  expect(blocked("mysql", "SELECT 1 INTO OUTFILE '/tmp/x'")).toContain("INTO OUTFILE");
  expect(blocked("mysql", "SELECT 1; DROP TABLE t")).toContain("multi-statement");
  expect(blocked("mysql", "SWITCH CATALOG hive")).toContain("SWITCH");
  expect(blocked("mysql", "SET NAMES utf8")).toContain("SET");
});

test("write 不跑 Gate", () => {
  gateQuery("mysql", "write", "INSERT INTO t VALUES (1)");
  gateQuery("redis", "write", "EVAL 'return 1' 0");
});

test("Redis / Mongo / ES 头", () => {
  gateQuery("redis", "read", "GET k");
  expect(blocked("redis", "SET k v")).toContain("SET");
  gateQuery("mongodb", "read", `{"find":"orders","filter":{}}`);
  expect(blocked("mongodb", `{"insert":"orders","documents":[]}`)).toContain("insert");
  gateQuery("elasticsearch", "read", `GET /orders/_search {"query":{"match_all":{}}}`);
  expect(blocked("elasticsearch", "POST /orders/_search")).toContain("POST");
  gateQuery("elasticsearch", "write", "POST /orders/_bulk {}");
});
