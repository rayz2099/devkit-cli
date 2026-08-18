# Codeup CLI

Personal Codeup client for humans and LLM agents. Exists to list repositories, push git refs, open Change Requests, and inspect webhooks without wrapping a vendor MCP server.

## Language

**Profile**:
A named connection to one Codeup organization: `name`, `url`, `token`. A config file has `defaultProfile` plus a `profiles` list.
_Avoid_: context, AK/SK, accessKey, anonymous single-object config

**Organization**:
The Yunxiao enterprise id. Taken from the last path segment of `Profile.url`. Never hardcoded.
_Avoid_: tenant, company, org slug

**Token**:
A personal Yunxiao access token. The only secret in this context.
_Avoid_: password, secretKey, x-yunxiao-token as a domain noun

**Audience**:
Who consumes stdout. `human` is the default omitted mode. `agent` is an LLM caller and drops hints, tables, and progress chatter.
_Avoid_: output format, --json

**Repository**:
A Codeup git repository, addressed as `group/project`, numeric id, cwd `origin`, or a unique Index hit.
_Avoid_: project (unless it is the repo name), package

**Index**:
A local snapshot of repositories for one Profile. Only Init replaces it. Fish completion reads it and never calls OpenAPI.
_Avoid_: MRU, implicit refresh, live search

**Init**:
The command that rebuilds the Index from the live repository list.
_Avoid_: sync, update, refresh as the command name

**Change Request**:
The Yunxiao review request. This is the only write model.
_Avoid_: pull request, merge request, PR as the domain noun

**Push**:
A local `git push` of the current branch to the Codeup remote. Not a Codeup API write.
_Avoid_: upload, publish, open CR

**Webhook**:
A repository hook subscription (target url plus event flags). A read model only. `secretToken` is masked unless `--show-secrets`.
_Avoid_: hook as a verb, Jenkins trigger, CI
