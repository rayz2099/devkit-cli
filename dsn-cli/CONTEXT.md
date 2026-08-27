# DSN CLI

Named connections for humans and LLM agents. Humans may exec a vendor Console; agents only run Query through the Driver and Gate.

## Language

**Profile**:
A named connection: `name`, `kind`, `url`, `access`. `-p` is required; there is no default Profile.
_Avoid_: context, datasource, jdbc profile, defaultProfile

**Kind**:
The backend type on a Profile: `mysql`, `doris`, `redis`, `mongodb`, `elasticsearch`. Later `s3`, `kafka` join this same list.
_Avoid_: engine, database, datasource, dialect

**Url**:
The Profile's standard connection URL (`mysql://`, `redis://`, `mongodb://`, `https://`). Doris uses `mysql://host:9030/…`.
_Avoid_: jdbcUrl, host/port/user fields, DSN as a second field name, doris://

**Access**:
Whether the Gate runs. Default `read` (allowlist). `write` forwards Query to the Driver with no Gate.
_Avoid_: mode, role, readonly as the config key

**Gate**:
In-process enumerated allowlist of statement heads per Kind. Runs only on Query when Access is `read`. Unknown heads, Writes, multi-statement strings, and write-shaped tails are rejected before the Driver sends them. When Access is `write`, Query is forwarded as-is. Console never hits the Gate.
_Avoid_: 约定 as if advisory, denylist, driver connection readOnly as the domain noun, prefix glob, vendor parser as the Gate

**Statement Head**:
The leading token the Gate looks up: SQL keyword (`SELECT`), Redis command (`HGET`), Mongo command name (`find`), ES method (`GET`). Grain is the head, not a sql-manual page such as `SHOW BACKENDS`.
_Avoid_: dialect, verb, SHOW BACKENDS as its own allowlist row

**Write**:
A mutating or side-effecting request: SQL DML/DDL, `SELECT INTO OUTFILE`, `SELECT FOR UPDATE`, Redis SET/DEL, Mongo insert/update/delete, ES index/delete, later S3 put, Kafka produce.
_Avoid_: query, execute

**Query**:
The `query` subcommand: one native statement. SQL, a Redis command, a Mongo command document, or an Elasticsearch `GET`/`HEAD` line. Goes through the Gate when Access is `read`.
_Avoid_: exec, eval, -e, REPL, mongosh JS, unified QL

**Console**:
TTY exec of the Kind's vendor client (`mysql`, `redis-cli`, `mongosh`). No Gate. `agent` and non-TTY cannot enter it. Kinds without a vendor client have no Console.
_Avoid_: our REPL, mysql-cli, wrapping vendor stdin as Query, Shell as a dsn-cli noun

**Driver**:
Registered implementation for one Kind. Opens a connection and runs Query. `doris` reuses the MySQL-protocol Driver and keeps its own Gate allowlist. Console does not use the Driver.
_Avoid_: wrapper, vendor CLI as the Query runtime, JDBC as the runtime

**Audience**:
Who consumes stdout. Omitted means `human`. Prefix `agent` means JSON, never Console, and applies the result row cap on Query.
_Avoid_: --json, mixing with `--output`

**Output**:
Format of Query and Doctor stdout. Human default is table; `--output` may be `json` (NDJSON), `csv`, or `plain`. Agent is always one JSON object with `rows` and `truncated`, and ignores `--output`. Console is the vendor client's stdout.
_Avoid_: applying --output to Console or agent

**Doctor**:
A connectivity probe of Profiles. Default is all Profiles in parallel; `-p` limits to one. Uses a Driver ping, not Query. Gate does not run. One Profile's hang does not wait the others: each has a deadline.
_Avoid_: health as cluster semantics, sequential scan, fail-fast abort of remaining Profiles
