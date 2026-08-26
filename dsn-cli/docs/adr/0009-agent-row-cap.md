# Agent result cap, never rewrite the Query

`agent` truncates after 1000 rows (override with `--limit`, `0` means no cap) and marks `truncated` in JSON. Human output is not capped.

The cap is applied while streaming Driver results. Injecting `LIMIT` into SQL or Redis `COUNT` was rejected: it changes query meaning and surprises humans. Agents need a cap so stdout cannot blow the context window.
