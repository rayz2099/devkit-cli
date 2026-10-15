# postgres is its own Kind

postgres joins the Kind registry as `postgres`, not a MySQL dialect and not a sibling CLI. Url is `postgres://` or `postgresql://` (libpq accepts both; refusing the IANA name only punishes pasted strings). TLS stays the postgres `sslmode=` query parameter. A `postgress://` / `postgres+ssl://` scheme was rejected so we do not invent a second family next to `rediss://` / `kafkas://`.

Query runs through the in-process `pg` Driver. Console is TTY `psql <url>` with the Profile Url as one argv, like mongosh, so sslmode and socket query parameters are not reimplemented as flags. `\d` and other psql meta-commands are Console-only. There is no table-name cache: SQL has no single argv slot the way kafka Peek has a Topic.

A `postgresql` Kind name and a `dialect` field were rejected. Cockroach / Greenplum stay out until their Gate surface is not postgres.
