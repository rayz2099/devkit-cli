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

test("postgres 只读头和写形尾巴", () => {
  gateQuery("postgres", "read", "tables");
  gateQuery("postgres", "read", "TABLES LIKE order%");
  gateQuery("postgres", "read", "tables like '%order%'");
  gateQuery("postgres", "read", "columns orders");
  gateQuery("postgres", "read", "desc public.orders");
  gateQuery("postgres", "read", "ddl orders");
  expect(blocked("postgres", "SHOW TABLES")).toContain("TABLES LIKE");
  expect(blocked("postgres", "SHOW TABLES LIKE 'order%'")).toContain("TABLES LIKE");
  expect(blocked("postgres", "SHOW CREATE TABLE orders")).toContain("DDL");
  expect(blocked("postgres", "SHOW COLUMNS FROM orders")).toContain("COLUMNS");
  expect(blocked("postgres", "tables foo")).toContain("TABLES or TABLES LIKE");
  expect(blocked("postgres", "columns")).toContain("COLUMNS <table>");
  expect(blocked("postgres", "ddl")).toContain("DDL <table>");
  expect(blocked("postgres", "tables like")).toContain("TABLES or TABLES LIKE");
  gateQuery("postgres", "read", "SELECT 1");
  gateQuery("postgres", "read", "WITH x AS (SELECT 1) SELECT * FROM x");
  gateQuery("postgres", "read", "TABLE orders");
  gateQuery("postgres", "read", "VALUES (1), (2)");
  gateQuery("postgres", "read", "SHOW search_path");
  gateQuery("postgres", "read", "EXPLAIN SELECT 1");
  gateQuery("postgres", "read", "EXPLAIN (FORMAT JSON) SELECT 1");
  gateQuery("postgres", "read", "EXPLAIN SELECT * FROM analyze");
  gateQuery("postgres", "read", "SELECT $$; not a second stmt $$");
  expect(blocked("postgres", "INSERT INTO t VALUES (1)")).toContain("INSERT");
  expect(blocked("postgres", "SET search_path TO public")).toContain("SET");
  expect(blocked("postgres", "USE app")).toContain("USE");
  expect(blocked("postgres", "COPY t TO STDOUT")).toContain("COPY");
  expect(blocked("postgres", "SELECT * FROM t FOR UPDATE")).toContain("FOR UPDATE");
  expect(blocked("postgres", "SELECT * FROM t FOR NO KEY UPDATE")).toContain("FOR NO KEY UPDATE");
  expect(blocked("postgres", "TABLE t FOR SHARE")).toContain("FOR SHARE");
  expect(blocked("postgres", "SELECT a, b INTO tmp FROM t")).toContain("INTO");
  expect(blocked("postgres", "EXPLAIN ANALYZE SELECT 1")).toContain("ANALYZE");
  expect(blocked("postgres", "EXPLAIN (ANALYZE, BUFFERS) SELECT 1")).toContain("ANALYZE");
  expect(blocked("postgres", "EXPLAIN ANALYZE INSERT INTO t SELECT 1")).toContain("ANALYZE");
  expect(blocked("postgres", "SELECT 1; DROP TABLE t")).toContain("multi-statement");
});

test("write 不跑 Gate", () => {
  gateQuery("mysql", "write", "INSERT INTO t VALUES (1)");
  gateQuery("postgres", "write", "INSERT INTO t VALUES (1)");
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

test("kafka 只读头是小写, peek n 有硬顶", () => {
  gateQuery("kafka", "read", "topics");
  gateQuery("kafka", "read", "peek album_audit_log");
  gateQuery("kafka", "read", "peek album_audit_log 50 partition 0");
  gateQuery("kafka", "read", "listen album_audit_log");
  expect(blocked("kafka", "PEEK album_audit_log")).toContain("PEEK");
  expect(blocked("kafka", "produce album_audit_log")).toContain("produce");
  expect(blocked("kafka", "peek album_audit_log 501")).toContain("1..500");
});
