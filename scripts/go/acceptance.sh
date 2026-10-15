#!/usr/bin/env bash
# 临时目录验收。不读真实配置，不安装到真实的 ~/.local/bin。
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/bin" "$work/home/.config" "$work/home/.cache" "$work/home/.local/state"
before="$(cd "$work/home/.config" && find . | sort | cksum)"

(
  cd "$root"
  CGO_ENABLED=0 go build -o "$work/bin/" ./cmd/...
)

export HOME="$work/home"
export XDG_CONFIG_HOME="$work/home/.config"
export XDG_CACHE_HOME="$work/home/.cache"
export XDG_STATE_HOME="$work/home/.local/state"

for bin in "$work/bin"/*; do
  name="$(basename "$bin")"
  "$bin" --help >/dev/null
  if [[ "$name" == "devkit-cli" ]]; then
    continue
  fi
  "$bin" version >/dev/null
  set +e
  "$bin" doctor >"$work/doctor.out" 2>"$work/doctor.err"
  code=$?
  set -e
  if [[ "$code" -gt 128 ]]; then
    echo "doctor crashed: $name" >&2
    exit 1
  fi
done

after="$(cd "$work/home/.config" && find . | sort | cksum)"
if [[ "$before" != "$after" ]]; then
  echo "config dir changed during doctor" >&2
  exit 1
fi

if "$work/bin/devkit-cli" list | grep -qx alist-cli; then
  echo "alist-cli still listed" >&2
  exit 1
fi
if ! "$work/bin/devkit-cli" list | grep -qx tl; then
  echo "tl missing from list" >&2
  exit 1
fi

echo "acceptance.sh ok"
