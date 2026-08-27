# Doctor probes in parallel without `-p`

Doctor is a connectivity probe over the Profile set, not a Query against one backend. Omitting `-p` means all Profiles; `-p` limits to one. This is the exception to 0006.

Each Profile is probed with concurrency 4 and a hard deadline (`--connect-timeout` + `--timeout`, defaults 3s + 5s). A hung socket must not delay the others. Unlimited parallel was rejected: a connection storm makes mysql2/`ioredis` hit `connectTimeout` while a lone Query to the same Profile succeeds. 1s connectTimeout was rejected for the same false `ETIMEDOUT`. Failures are collected into one table; exit `3` if any fail. Sequential scan and fail-fast abort were rejected.

Doctor uses the Driver ping (`SELECT 1` / `PING` / `{ping:1}` / `GET /`), not Query, and does not run the Gate. Wrapping vendor `mysqladmin ping` was rejected: Console is TTY-only and elasticsearch has no client.
