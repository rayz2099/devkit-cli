# Kafka Record ceiling

Peek defaults to 50 Records and rejects a requested N above 500 (Gate exit 2). Listen stops at the first of `--timeout`, SIGINT, or 500 Records.

ADR 0009's "human output is not capped" does not apply here: an uncapped kafka read is a cluster incident, not a large SQL result. Rewriting the user's statement to insert a LIMIT was still rejected; this is a Kind ceiling, not a rewrite.
