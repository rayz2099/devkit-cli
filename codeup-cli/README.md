# codeup-cli

Yunxiao Codeup CLI for humans and LLM agents.

## Install

From the repo root:

```bash
just build codeup-cli
just install codeup-cli
```

Config: `~/.config/codeup-cli/config.json`

```json
{
  "defaultProfile": "work",
  "profiles": [
    {
      "name": "work",
      "url": "https://codeup.aliyun.com/<organizationId>",
      "token": ""
    }
  ]
}
```

`url` must be the organization root. The last path segment is the Organization id. Do not put a repository path in `url`.

Repo index after `init`: `~/.config/codeup-cli/repos-<profile>.json`

## Usage

```bash
codeup-cli [-p profile] [agent|human] <command>

codeup-cli init
codeup-cli repos [--search S]
codeup-cli push [--remote origin] [branch]
codeup-cli cr list [group/project|name]
codeup-cli cr get <localId> [--repo group/project]
codeup-cli cr create --source <branch> --title <title> [--target <branch>]
codeup-cli webhook list [--repo group/project] [--show-secrets]
```

- Audience prefix: omit for `human`; `agent` prints stable JSON.
- `cr list` defaults to `--state opened`. Use `--state all` or `--state merged`.
- Short repo names resolve through the local index. Run `init` first.
- Webhook is read-only. `secretToken` is masked unless `--show-secrets`.
- Merge is not exposed.

## License

GPL-2.0-only.
