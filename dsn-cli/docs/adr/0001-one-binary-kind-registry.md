# One binary, Kind registry

`dsn-cli` is a single CLI. `Profile.kind` selects a registered Driver. Later kinds (`s3`, `kafka`) join this binary instead of sibling CLIs.

Splitting later rewrites every Profile, Fish completion, and agent call site. Sibling CLIs were rejected so humans keep one `-p` path across backends.
