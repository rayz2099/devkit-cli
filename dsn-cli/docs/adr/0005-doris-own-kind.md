# Doris is its own Kind

Doris speaks the MySQL protocol on FE `:9030`, but its write set (`LOAD`, `EXPORT`, `INSERT OVERWRITE`, `SELECT INTO OUTFILE`) is not MySQL's.

`kind: doris` reuses the MySQL-protocol Driver and keeps a separate Gate allowlist. Url stays `mysql://host:9030/db`. A `dialect` field next to `kind` was rejected; merging Doris into the MySQL allowlist was rejected because the reject rules would leak across Kinds.
