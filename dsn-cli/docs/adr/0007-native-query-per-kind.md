# Native Query per Kind

There is no unified QL. The `query` subcommand takes one native statement for that Kind. `-e` was rejected so Console (`dsn-cli -p buy`) and Query (`… query '…'`) stay distinct argv shapes.

Mongo is one `runCommand` document; the Gate reads the first key (`find` / `aggregate` / `count` vs `insert` / `update` / `delete`). mongosh JS was rejected because eval bypasses the Gate.

Elasticsearch is one REST line `GET|HEAD /path` with an optional JSON body on that same line. Read Access allows `GET` / `HEAD` only. `POST /_search` vs `POST /_bulk` share a method, so method-only write gating was rejected.

Redis uses command heads. v1 read allowlist is the usual getters/scanners (`GET`, `HGET`, `LRANGE`, `SCAN`, `INFO`, …).
