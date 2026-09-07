# Merge defaults to squash

Codeup OpenAPI requires `mergeType` and documents `ff-only` first. Callers of this CLI omit flags; the house history shape is one squash commit per Change Request. Other types exist only as `--type`. Changing the omitted value later rewrites every agent call site.

## Considered Options

- Squash-only with no flag: rejected. Some protected branches demand ff-only or rebase.
- Require `--type` always: rejected. Agents would copy the vendor example and send ff-only.
