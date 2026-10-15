#!/usr/bin/env bash
# 本机检查。脚本用 bash，调用方式兼容 fish。
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

if [[ -n "$(gofmt -l .)" ]]; then
  echo "gofmt dirty:" >&2
  gofmt -l . >&2
  exit 1
fi

go vet ./...
go test ./...
CGO_ENABLED=0 go build -o bin/ ./cmd/...
echo "check.sh ok"
