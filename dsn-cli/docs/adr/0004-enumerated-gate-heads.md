# Enumerated Gate heads

The Gate is a closed set of statement heads per Kind. It is not a string prefix and not an embedded vendor parser.

Unknown heads fail closed. New Doris `SHOW BACKENDS` still passes once `SHOW` is listed; `LOAD` / `EXPORT` / `INSERT` stay out until added.

v1 SQL read heads are `SELECT` / `WITH` / `SHOW` / `EXPLAIN` / `DESC` / `DESCRIBE` / `USE`. Doris also allows `SWITCH`. `SELECT` / `WITH` still reject `INTO OUTFILE` / `INTO DUMPFILE` / `FOR UPDATE` / `LOCK IN SHARE MODE` / `FOR SHARE`. `SET` stays out so `SET GLOBAL` cannot ride along.

When Access is `read`, one Query is one statement; multi-statement strings are rejected. Splitting on `;` would be a parser. When Access is `write`, the Gate does not run.
