# Jenkins CLI

Personal Jenkins controller client for humans and LLM agents. Exists so `$dt-deploy` / code-ws CI can list, trigger, and inspect builds without the archived Go `jk` surface.

## Language

**Profile**:
A named connection to one Jenkins controller: `name`, `url`, `username`, `password`, `apiToken`.
_Avoid_: Context, JK_CONTEXT, controller alias

**Audience**:
Who consumes stdout. `human` is the default omitted mode. `agent` is an LLM caller and drops hints, tables, and progress chatter.
_Avoid_: output format, --json, Jenkins node

**Job**:
A Jenkins item that can be built, addressed by `JobPath`.
_Avoid_: pipeline, project (unless the Job name happens to be a repo name)

**JobPath**:
Slash-separated Jenkins folder path, such as `dt-vshop/master`, mapped to `/job/dt-vshop/job/master`.
_Avoid_: URL, folder URI

**Run**:
One numbered build of a Job (`#123`, `lastBuild`).
_Avoid_: build (as a domain noun), execution

**Secret**:
The Basic-auth password sent to Jenkins REST. A non-empty `apiToken` is the Secret; only when `apiToken` is empty does `password` become the Secret. Both empty is an error.
_Avoid_: guessing, mixing the two fields, keychain
