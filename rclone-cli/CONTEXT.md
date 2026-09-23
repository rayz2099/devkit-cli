# rclone-cli

WebDAV 同步 CLI（123 网盘）。底层只拼 rclone argv，不读、不写、不依赖 `rclone.conf`。本文件是实现合同；实现放在 `devkit-cli/rclone-cli/`，对齐 `alist-cli` 的 bun + just。项目在 `devkit-cli/rclone-cli/`；fish 补全只放 `my-script/fish/completions/rclone-cli.fish`。

## Language

**Source**:
Named local tree: `name`, `path`. Path is an absolute directory, or `$HOME/...` / `${HOME}/...` which expands against HOME. Other `$` variables are not expanded.
_Avoid_: profile, cwd as implicit source, embedding the local username

**Target**:
Named pipe: WebDAV `url` / `user` / `pass` / `root` required. `vendor` and transport knobs optional with defaults. Optional Crypt. Secrets live here. File mode 600.
_Avoid_: rclone remote name (`pan123:`), rclone.conf, kind/s3 enum

**Task**:
Named edge: `source` + `target` plus optional overrides. The only executable noun. Verbs:

- `ls` with no task: see the graph (sources / targets / tasks). No network.
- `ls <task> [path] [--limit n]`: list both sides, print at most n rows (default 10). Remote names stay as stored, so leftover `filename.bin` is visible.
- `check <task> [path]`: the same list, then size verdict. Same-name wins; `.bin` counts as present when the same-name object is absent. Remote extras do not fail.
- `push` / `pull`: move a tree. `pull` restores into Source.path and rejects a file path.
- `get`: one file `copyto` into the source tree, optional dest.
- `download`: one file `copyto` into the current directory (or dest). This is how a single file leaves the library.

_Avoid_: defaultTask, profile, dest as the CLI noun, `rclone check` as the status engine

**Root**:
Target field. The directory name on the remote, and it must equal the source directory name (`basename(Source.path)`). Subdirs append RelPath under that folder. Crypt does not add another directory.
_Avoid_: a second bucket name such as `photograph-crypt`, `pan123:photograph` as a remote id

**RelPath**:
Path of the CLI `[path]` relative to Source.path. Dest = `Root/RelPath`. `[path]` outside Source.path is an error.
_Avoid_: using basename as a new remote root

**Crypt**:
Optional array on Target. Each pair has `name`, `key`, `salt`; `algorithm` and `default` are optional. Omitted `algorithm` is `secretbox` (rclone's NaCl XSalsa20-Poly1305). Omitted `default` is false; if no pair is marked, the first pair is the encrypt pair. Two `default: true` → config error. Push encrypts with the default pair. Decrypt (`pull` / `get` / `download`) retries the other pairs after the default. Array absent → plaintext. Empty array, a lone object, two defaults, or the old `password` / `password2` keys → config error, no degrade. The WebDAV directory is still Root, same as the source folder name. With filename encryption off, that folder and the files inside it keep their real names; file contents stay encrypted. rclone's default crypt suffix is `.bin`; this tool sets `suffix=none` so new objects keep the same name. `pull` / `get` still try `.bin` after a same-name miss, because the existing remote already has `filename.bin`.
_Avoid_: `--encrypt` flag, a second crypt remote in rclone.conf, empty crypt array meaning plaintext, a ciphertext directory name different from the source folder, writing new objects as `filename.bin`, keeping `password` / `password2` in config.json, implicit algorithm

**Override**:
Task may set only `excludes` (append), `transfers`, `tpslimit`, `timeout`, `encryptFilenames`, `skipHidden`. Omitted fields use the Target values. `encryptFilenames: true` requires Crypt, and turns on rclone `filename_encryption=standard` plus directory name encryption. `false` keeps names plain (`filename_encryption=off`, `suffix=none`). Task wins. Cannot override `url`/`user`/`pass`/`root`/`crypt` secrets.
_Avoid_: task copying the whole Target

**Builtin Excludes**:
Always applied: `.git/**`, `.grok/**`, `.DS_Store`. When `skipHidden` is true (default true), also `.*` and `**/.*` so dotfiles and hidden directories are not copied. Symlinks skipped (`--skip-links`). `_work/**` is uploaded. Task `excludes` append.
_Avoid_: putting the builtin list in every Target

**Run**:
One local execution record after a Task command. Stored under XDG data (`~/.local/share/rclone-cli/runs/`). Not a config object. Args stored with secrets redacted.
_Avoid_: writing runs into config.json, rclone.conf log

**Audience**:
Omitted = human (progress on stderr). Prefix `agent` = JSON summary, no progress bar.
_Avoid_: --json as a separate dialect

## CLI

```
rclone-cli push <task> [path] [--dry-run]
rclone-cli pull <task> [path] [--dry-run]
rclone-cli check <task> [path]
rclone-cli get <task> <path> [dest] [--dry-run]
rclone-cli download <task> <path> [dest] [--dry-run]
rclone-cli ls
rclone-cli ls <task> [path] [--limit n]
rclone-cli runs
rclone-cli completion fish
```

No `<task>` → exit 2, print usage, list Task names, do not transfer. There is no default Task.

- `push` / `pull`: `rclone copy` (never `rclone sync`; never delete extras on the other side).
- `ls` with no task: print sources / targets / tasks from config. No network.
- `ls <task>`: same two listings as check, then print at most `--limit` rows (default 10). Counts stay full. `-n` is an alias of `--limit`.
- `check`: same two listings, then join by relative path. A local file is backed up if a same-name or `.bin` object has the same size. Missing / size mismatch → exit 1. Remote extras are listed, not a failure. Never `rclone check`.
- `get`: `rclone copyto` one file into the source tree. Optional `dest` may sit outside Source.path.
- `download`: `rclone copyto` one file into the current directory (basename), or into dest. `pull` of a file path is rejected with this command.
- `runs`: print recent Run records.

`just install` bootstraps `config.example.json` → `~/.config/rclone-cli/config.json` only when missing. It does not write fish completions; those live in my-script.

## Config

JSONC (comments + trailing commas), same as `dsn-cli`. Path: `~/.config/rclone-cli/config.json`.

```jsonc
{
  "rcloneBin": "rclone",
  "sources": [
    { "name": "photograph", "path": "$HOME/workspace/photograph" }
  ],
  "targets": [
    {
      "name": "123",
      "url": "https://webdav.123pan.cn/webdav",
      "vendor": "other",
      "user": "",
      "pass": "",
      "root": "photograph",
      "skipLinks": true,
      "skipHidden": true,
      "sizeOnly": true,
      "transfers": 1,
      "checkers": 4,
      "retries": 10,
      "retriesSleep": "15s",
      "lowLevelRetries": 20,
      "timeout": "1h",
      "contimeout": "60s",
      "tpslimit": 2,
      "pacerMinSleep": "200ms",
      "encryptFilenames": false
    }
  ],
  "tasks": [
    { "name": "photograph-123", "source": "photograph", "target": "123" }
  ]
}
```

Unknown Task / Source / Target names are errors. `rcloneBin` missing → `rclone` on PATH.

## rclone argv

Every invocation:

1. `--config` points at a tool-owned empty file (create once under `~/.config/rclone-cli/empty.conf`). Never read `~/.config/rclone/rclone.conf`.
2. Build a WebDAV on-the-fly remote: `:webdav,url='…',vendor=other,user='…',pass='<obscured>':<path>`
3. `pass`, plus every Crypt pair `key` / `salt`, go through `rclone obscure`. Config stores plaintext; the connection string stores the obscured form as `pass` / `password` / `password2`.
4. Plain path is `Root/RelPath` on WebDAV. If Crypt is on, the inner WebDAV path is `Root` (the source folder name) and the outer crypt path is only `RelPath`: `:crypt,remote=':webdav,…:<root>',…,password='<obscured>',password2='<obscured>':<rel>`. Push uses the default pair. Decrypt retries remaining pairs. Name encryption follows `encryptFilenames` (task then target): `true` → `filename_encryption=standard,directory_name_encryption=true`; `false` → `filename_encryption=off,directory_name_encryption=false,suffix='none'`. `pull` / `get` retry the same remote with `suffix='.bin'` after a same-name miss. `pull` of a tree then copies leftover `.bin` with `--ignore-existing` so a same-name file already written is not overwritten by the old object.
5. Push: `rclone copy <local> <remote> …flags… --progress`
6. Pull: swap the two paths. Get / download: `rclone copyto` with the same swap. Single-file copyto uses `--retries 1` so a wrong crypt pair fails in one shot and the next pair can run. Probe lists the parent with each pair until a name or `name.bin` is found.
7. `ls <task>` / `check`: `rclone lsjson --recursive --files-only` on local and on the `suffix=none` remote, then compute. Not `rclone check`.

Transport flags come from Target then Override. Omitted knobs use defaults: `skipLinks` / `skipHidden` / `sizeOnly` true, `encryptFilenames` false, `transfers` 1, `checkers` 4, `retries` 10, `retriesSleep` 15s, `lowLevelRetries` 20, `timeout` 1h, `contimeout` 60s, `tpslimit` 2, `pacerMinSleep` 200ms, `vendor` other. `--skip-links` / `--size-only` follow those booleans. `skipHidden` adds `--exclude .*` and `--exclude **/.*`. `pacerMinSleep` always lands on the WebDAV connection string.

Quoting: connection strings contain `:` and `,`. Follow rclone connection-string quoting; add a unit test that the argv round-trips a password with special characters.

## Run record

After each push/pull/check (including dry-run and failures), append one JSON object:

- `id`, `task`, `action`, `startedAt`, `finishedAt`, `code`
- `local`, `remote` (no user/pass)
- `dryRun`
- `bytes`, `checks`, `transfers` if parseable from rclone output
- `error` if non-zero

Directory: `~/.local/share/rclone-cli/runs/` as JSONL (`runs.jsonl`) or one file per id. `runs` command reads newest first.

## Layout

Match `alist-cli`: `src/cli.ts` entry, `src/*.ts` split by concern, `test/*.test.ts`, `package.json` bun compile, `justfile` (`test` / `typecheck` / `build` / `install`), `config.example.json`. Root `just install rclone-cli` discovers this module via its justfile. Fish completion is not installed here.

Chinese `why` comments on exported functions/types. No fallback after a missing field: throw. No `rclone.conf` helpers except the empty `--config` file.

## Out of scope

Thumbnails, GUI, rclone rcd, `rclone sync` prune, S3/Huawei target shape, reading existing `pan123` / `pan123-crypt` remotes, a default Task.

## Build sequence

1. Schema + JSONC load + empty `--config` file. Done: invalid config fails with field path; example file loads.
2. Connection-string builder + `rclone obscure`. Done: unit tests, no live network.
3. `push`/`pull`/`check` with RelPath + Builtin Excludes + Override merge. Done: `--dry-run` against a fake rclone stub or captured argv.
4. Run records + `ls` + `runs`. Done: a failed push still writes a Run.
5. `just install`, `Audience`. Done: `rclone-cli push` with no args exits 2 and lists Task names. Fish stays in my-script.

Do not ship until step 3 argv never includes a user rclone.conf path and never contains the word `sync` as a subcommand.
