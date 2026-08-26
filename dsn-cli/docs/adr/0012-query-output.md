# Query Output vs Audience

Human `query` defaults to a table. `--output json|csv|plain` changes that; `json` is NDJSON for pipes. Agent `query` is always one JSON object `{ rows, truncated }` and ignores `--output`.

Audience chooses the consumer; Output is a human-only format switch. Letting `--output` change agent JSON would break callers. Console stdout is not ours to reformat.
