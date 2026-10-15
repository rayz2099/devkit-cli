import { expect, test } from "bun:test";
import { parsePgCatalog, pgCatalogComplete, pgCatalogHead } from "../src/pg-stmt";
import { DsnErr } from "../src/types";

test("parsePgCatalog 接受 TABLES / COLUMNS / DDL / DESC", () => {
  expect(parsePgCatalog("tables")).toEqual({ head: "tables" });
  expect(parsePgCatalog("TABLES LIKE order%")).toEqual({ head: "tables", like: "order%" });
  expect(parsePgCatalog("tables like '%order%'")).toEqual({ head: "tables", like: "%order%" });
  expect(parsePgCatalog("columns orders")).toEqual({ head: "columns", rel: "orders" });
  expect(parsePgCatalog("desc public.orders")).toEqual({
    head: "columns",
    rel: "public.orders",
  });
  expect(parsePgCatalog("describe orders")).toEqual({ head: "columns", rel: "orders" });
  expect(parsePgCatalog("ddl orders")).toEqual({ head: "ddl", rel: "orders" });
  expect(pgCatalogHead("SELECT 1")).toBeUndefined();
  expect(pgCatalogHead("TABLE orders")).toBeUndefined();
});

test("parsePgCatalog 拒绝错误形状", () => {
  expect(() => parsePgCatalog("tables foo")).toThrow(DsnErr);
  expect(() => parsePgCatalog("tables like")).toThrow(DsnErr);
  expect(() => parsePgCatalog("SELECT 1")).toThrow(DsnErr);
  expect(() => parsePgCatalog("columns")).toThrow(DsnErr);
  expect(() => parsePgCatalog("ddl")).toThrow(DsnErr);
  expect(() => parsePgCatalog("columns orders extra")).toThrow(DsnErr);
  expect(() => parsePgCatalog("columns pay-order")).toThrow(DsnErr);
});

test("pgCatalogComplete 补目录头", () => {
  expect(pgCatalogComplete([])).toEqual(["tables", "columns", "ddl", "desc"]);
  expect(pgCatalogComplete(["tables"])).toEqual(["like"]);
  expect(pgCatalogComplete(["tables", "like"])).toEqual([]);
  expect(pgCatalogComplete(["columns"])).toEqual([]);
  expect(pgCatalogComplete(["ddl"])).toEqual([]);
});
