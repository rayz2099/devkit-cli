# postgres COLUMNS and DDL catalog commands

Table metadata and the CREATE statement are basic catalog reads, like `TABLES`. Postgres has no `DESC` / `SHOW CREATE TABLE`; `\d` is psql-only. Reconstructing from `pg_catalog` in the Driver keeps Query off `psql` / `pg_dump`.

Heads are `COLUMNS <table>` (aliases `DESC` / `DESCRIBE`, same argv shape as mysql) and `DDL <table>`. `DESC` is not sent to the server. `SHOW CREATE` / `SHOW COLUMNS` stay rejected as SHOW variants (ADR 0004); the Gate points at `DDL` / `COLUMNS`.

`DDL` emits `CREATE TABLE` / `CREATE VIEW` from `format_type`, identity/generated, `pg_get_constraintdef`, extra `pg_get_indexdef`, and comments. It is a reconstruction, not `pg_dump`. Table names are `ident` or `schema.ident` and bound through `to_regclass($1)`. No table-name completion cache.
