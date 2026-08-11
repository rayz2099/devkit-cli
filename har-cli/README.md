# har-cli

`har-cli` is a Bun-based command-line tool for analyzing HAR files and extracting request URIs for permission whitelisting and regression loops.

## Features

- Scan one or more HAR files exported from Chrome DevTools
- Filter entries with an expression such as `body.status == 1 and http.status == 403`
- Render matched entries as raw HTTP request/response text by default
- Project selected fields such as `path`, `http.status`, or `response.headers`
- Deduplicate output by default
- Generate Fish shell completion
- Build and install with `just`

## Requirements

- Bun 1.3.x
- just 1.14+

## Build

```bash
just build
```

The compiled binary is written to `dist/har-cli`.

## Install

```bash
just install
```

This installs:

- `~/.local/bin/har-cli`
- `~/.config/fish/completions/har-cli.fish`

## Usage

```bash
har-cli sample.har
```

Short filter form:

```bash
har-cli sample.har -f 'body.status == 1 and http.status == 403'
```

Keep only path:

```bash
har-cli sample.har -f 'body.status == 1 and http.status == 403' -s path
```

Keep selected fields:

```bash
har-cli sample.har -s path,http.status
```

Output prefix aggregation:

```bash
har-cli sample.har -s prefix -p 2
```

Structured JSON output:

```bash
har-cli sample.har -o json -s path,http.status
```

## Expression Context

The filter expression can access these fields:

- `http.status`
- `http.method`
- `request.url`
- `request.path`
- `response.mimeType`
- `body`
- `uri.url`
- `uri.path`

Supported operators:

- `==`, `!=`, `>`, `>=`, `<`, `<=`
- `and`, `or`, `not`
- `&&`, `||`, `!`

If the response body is not valid JSON, `body` is `undefined`.

## Help and Completion

```bash
har-cli help
har-cli --help
har-cli completions fish
```

Example Fish setup:

```fish
har-cli completions fish > ~/.config/fish/completions/har-cli.fish
```

## Development

```bash
just clean
just build
just install
bun test
bun run typecheck
```

## License

This project is licensed under the GNU General Public License v3.0 only. See [LICENSE](./LICENSE).
