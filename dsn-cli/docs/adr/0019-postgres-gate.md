# postgres Gate heads and write-shaped tails

postgres read heads are `SELECT` / `WITH` / `TABLE` / `VALUES` / `SHOW` / `EXPLAIN`. They are not mysql's list: `USE` / `DESC` / `DESCRIBE` are not postgres SQL, and copying them would only bounce at the server. `SET` stays out so `SET ROLE` / `SET SESSION AUTHORIZATION` cannot ride a `search_path` exception; grain is still the head, not a prefix rule. Schema-qualify instead, or use Access `write`.

`SELECT` / `WITH` reject `INTO` (postgres `SELECT INTO` creates a table) and the lock tails `FOR UPDATE` / `FOR SHARE` / `FOR NO KEY UPDATE` / `FOR KEY SHARE`. `TABLE` / `VALUES` reject the same lock tails. `COPY` / `DO` / `CALL` / `LISTEN` / `ANALYZE` (stats) are unknown heads.

`EXPLAIN` is a read of a plan. `EXPLAIN ANALYZE` executes the inner statement, which may be a Write. The Gate rejects `ANALYZE` when it appears before the inner statement head (`EXPLAIN ANALYZE SELECT`, `EXPLAIN (ANALYZE) INSERT`). `EXPLAIN SELECT * FROM analyze` is allowed: `ANALYZE` is a later identifier, not the option. This is token-sequence matching, not a vendor parser.

Dollar-quoted strings (`$$…$$`, `$tag$…$tag$`) are strings so a `;` inside them is not a second statement. postgres `'…'` strings do not treat `\` as escape; mysql-style backslash escaping would hide `E'\'; DROP …`. `#` is not a comment in postgres.
