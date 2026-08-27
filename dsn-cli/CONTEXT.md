# DSN CLI

Named connections for humans and LLM agents. Humans may exec a vendor Console; agents only run Query through the Driver and Gate.

## Language

**Profile**:
A named connection: `name`, `kind`, `url`, `access`. `-p` is required; there is no default Profile.
_Avoid_: context, datasource, jdbc profile, defaultProfile

**Kind**:
The backend type on a Profile: `mysql`, `doris`, `postgres`, `redis`, `mongodb`, `elasticsearch`, `kafka`. Later `s3` joins this same list.
_Avoid_: engine, database, datasource, dialect, kf, postgresql, pg

**Url**:
The Profile's standard connection URL (`mysql://`, `postgres://`, `redis://`, `mongodb://`, `https://`, `kafka://`). Doris uses `mysql://host:9030/…`. postgres also accepts `postgresql://`. kafka uses one bootstrap (`kafka://host:9092`); TLS is `kafkas://`. postgres TLS is the `sslmode=` query parameter, not a second scheme.
_Avoid_: jdbcUrl, host/port/user fields, DSN as a second field name, doris://, jdbc:postgresql, postgress://, comma-separated brokers as the URL host

**Access**:
Whether the Gate runs. Default `read` (allowlist). `write` forwards Query to the Driver with no Gate.
_Avoid_: mode, role, readonly as the config key

**Gate**:
In-process enumerated allowlist of statement heads per Kind. Runs only on Query when Access is `read`. Unknown heads, Writes, multi-statement strings, and write-shaped tails are rejected before the Driver sends them. When Access is `write`, Query is forwarded as-is. Console never hits the Gate.
_Avoid_: 约定 as if advisory, denylist, driver connection readOnly as the domain noun, prefix glob, vendor parser as the Gate

**Statement Head**:
The leading token the Gate looks up: SQL keyword (`SELECT`), Redis command (`HGET`), Mongo command name (`find`), ES method (`GET`), kafka command (`topics` / `peek` / `listen`). Grain is the head, not a sql-manual page such as `SHOW BACKENDS`. kafka heads are lowercase. postgres read heads include `TABLE` / `VALUES` / `TABLES` / `COLUMNS` / `DDL`; `DESC` / `DESCRIBE` alias `COLUMNS`. postgres `SHOW` is a GUC read, not `SHOW TABLES` / `SHOW CREATE`. `TABLES LIKE` is an argument to the `TABLES` head.
_Avoid_: dialect, verb, SHOW BACKENDS as its own allowlist row, SHOW TABLES as a SHOW row, SHOW CREATE as a SHOW row, PEEK as a kafka head, psql backslash as a Query head

**Write**:
A mutating or side-effecting request: SQL DML/DDL, `SELECT INTO OUTFILE`, `SELECT FOR UPDATE`, postgres `SELECT INTO` / `COPY` / `EXPLAIN ANALYZE`, Redis SET/DEL, Mongo insert/update/delete, ES index/delete, later S3 put, Kafka produce.
_Avoid_: query, execute

**Query**:
The `query` subcommand: one native statement, as one argv or several argv tokens joined by space. SQL, a Redis command, a Mongo command document, an Elasticsearch `GET`/`HEAD` line, or a kafka command. Goes through the Gate when Access is `read`. postgres Query is SQL plus catalog commands (`TABLES`, `COLUMNS`, `DDL`), not psql meta-commands (`\d`).
_Avoid_: exec, eval, -e, REPL, mongosh JS, unified QL, --topic as a second dialect, \dt as Query, SHOW TABLES as Query, SHOW CREATE as Query

**Console**:
TTY exec of the Kind's vendor client (`mysql`, `psql`, `redis-cli`, `mongosh`). No Gate. `agent` and non-TTY cannot enter it. Kinds without a vendor client have no Console.
_Avoid_: our REPL, mysql-cli, wrapping vendor stdin as Query, Shell as a dsn-cli noun

**Driver**:
Registered implementation for one Kind. Opens a connection and runs Query. `doris` reuses the MySQL-protocol Driver and keeps its own Gate allowlist. postgres has its own protocol Driver. Console does not use the Driver.
_Avoid_: wrapper, vendor CLI as the Query runtime, JDBC as the runtime

**Audience**:
Who consumes stdout. Omitted means `human`. Prefix `agent` means JSON, never Console, and applies the result row cap on Query.
_Avoid_: --json, mixing with `--output`

**Output**:
Format of Query and Doctor stdout. Human default is table; `--output` may be `json` (NDJSON), `csv`, or `plain`. `--pretty` indents human `--output json` as one JSON array. Agent is always one JSON object with `rows` and `truncated`, and ignores `--output` / `--pretty`. Console is the vendor client's stdout.
_Avoid_: applying --output to Console or agent

**Doctor**:
A connectivity probe of Profiles. Default is all Profiles in parallel; `-p` limits to one. Uses a Driver ping, not Query. Gate does not run. One Profile's hang does not wait the others: each has a deadline.
_Avoid_: health as cluster semantics, sequential scan, fail-fast abort of remaining Profiles

## Kafka

**Topic**:
A named log on a kafka Profile. Completion uses cached names; a list-topics Query is what refreshes that cache.
_Avoid_: table, queue, channel, subject

**Record**:
One kafka log entry: partition, offset, timestamp, key, value, headers.
_Avoid_: message, event, row as the kafka unit

**Peek**:
A snapshot read of Records from near the high watermark. Assigns partitions, does not join a consumer group, and does not commit offsets.
_Avoid_: consume, subscribe, group consume

**Listen**:
A streaming read of new Records starting at the current high watermark. Same assignment rules as Peek. Stops on timeout, SIGINT, or the kafka Record ceiling.
_Avoid_: consume, tail, follow as the domain noun

**Topic Cache**:
A local list of Topic names for one Profile. Completion reads it and never contacts the cluster. Only a list-topics Query refreshes it.
_Avoid_: live metadata on TAB, a global cache shared across Profiles
