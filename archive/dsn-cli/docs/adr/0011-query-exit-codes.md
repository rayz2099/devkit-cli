# Query exit codes

`query` uses `0` success (empty result included), `1` usage/config, `2` Gate reject, `3` server/driver error.

A single `0`/`1` cannot tell an agent whether the allowlist fired. Jenkins `2` = not found does not apply: an empty SELECT is success.
