import { splitArgs } from "./split";
import { DsnErr } from "./types";

export type PgCatalog =
  | { head: "tables"; like?: string }
  | { head: "columns"; rel: string }
  | { head: "ddl"; rel: string };

const CATALOG_HEADS = new Set(["TABLES", "COLUMNS", "DDL", "DESC", "DESCRIBE"]);
const REL_HEADS = new Set(["COLUMNS", "DDL", "DESC", "DESCRIBE"]);
const COMPLETE_HEADS = ["tables", "columns", "ddl", "desc"];

/** 为什么: 目录命令不是 SQL, Gate 和 Driver 必须认同一套形状. */
export function pgCatalogHead(stmt: string): string | undefined {
  const head = splitArgs(stmt)[0]?.toUpperCase();
  if (head === undefined || !CATALOG_HEADS.has(head)) {
    return undefined;
  }
  return head;
}

export function parsePgCatalog(stmt: string): PgCatalog {
  const args = splitArgs(stmt);
  const raw = args[0];
  if (raw === undefined) {
    throw new DsnErr("empty statement", 2);
  }
  const head = raw.toUpperCase();
  if (head === "TABLES") {
    return parseTables(args);
  }
  if (REL_HEADS.has(head)) {
    const rel = parseRel(args);
    if (head === "DDL") {
      return { head: "ddl", rel };
    }
    return { head: "columns", rel };
  }
  throw new DsnErr(`blocked head: ${raw}`, 2);
}

function parseTables(args: string[]): Extract<PgCatalog, { head: "tables" }> {
  if (args.length === 1) {
    return { head: "tables" };
  }
  const likeKw = args[1];
  const pattern = args[2];
  if (args.length === 3 && likeKw !== undefined && likeKw.toUpperCase() === "LIKE") {
    if (pattern === undefined || pattern === "") {
      throw new DsnErr("tables LIKE requires a pattern", 2);
    }
    return { head: "tables", like: pattern };
  }
  throw new DsnErr("tables query must be TABLES or TABLES LIKE <pattern>", 2);
}

function parseRel(args: string[]): string {
  const cmd = (args[0] ?? "columns").toUpperCase();
  if (args.length !== 2) {
    throw new DsnErr(`${cmd} query must be ${cmd} <table>`, 2);
  }
  const rel = args[1];
  if (rel === undefined || rel === "") {
    throw new DsnErr(`${cmd} requires a table`, 2);
  }
  if (!/^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$/.test(rel)) {
    throw new DsnErr("invalid table name", 2);
  }
  return rel;
}

export function pgCatalogComplete(rest: string[]): string[] {
  if (rest.length === 0) {
    return [...COMPLETE_HEADS];
  }
  const head = rest[0]?.toLowerCase();
  if (head === "tables") {
    return rest.length === 1 ? ["like"] : [];
  }
  if (head === "columns" || head === "ddl" || head === "desc" || head === "describe") {
    return [];
  }
  return [...COMPLETE_HEADS];
}
