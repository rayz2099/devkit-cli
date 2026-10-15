# Vendor Console, Gate only on Query

TTY `dsn-cli -p <profile>` execs the native client on PATH: `mysql` (also doris), `psql`, `redis-cli`, `mongosh`. dsn-cli does not implement a REPL. This Console may be removed later; it is not the long-term surface.

`agent`, a pipe, or a Kind with no vendor client (`elasticsearch`, `kafka`, later `s3`) cannot enter Console: they error and must use `query`. Exit status of Console is the vendor client's, untranslated.

Gate, Access, `--output`, and `--limit` apply only to `query`. The deprecated `mysql-cli` is not spawned.
