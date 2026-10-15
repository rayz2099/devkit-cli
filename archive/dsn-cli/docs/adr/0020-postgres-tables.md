# postgres TABLES catalog command

Listing relations with `information_schema` is valid SQL and remains allowed. It is also the wrong shape for a CLI: kafka already has `topics`, mysql has `SHOW TABLES LIKE`, and `\dt` is Console-only.

postgres therefore gets a catalog head `TABLES` / `TABLES LIKE <pattern>`. LIKE uses SQL `%` / `_` against `table_name` only, bound as a parameter. The result is base tables outside `pg_catalog` / `information_schema`. Views stay out; `information_schema` Query still lists them.

`SHOW TABLES` is not a SHOW variant. Grain is the head (ADR 0004); special-casing `SHOW TABLES` would be the Doris `SHOW BACKENDS` row we rejected. The Gate rejects `SHOW TABLES` with a pointer to `TABLES`. This is not a table-name cache and does not complete table names.
