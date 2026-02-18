#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "set-version error: expected semantic version (X.Y.Z), got: ${version:-<empty>}" >&2
  exit 1
fi

printf '%s\n' "$version" > VERSION

tmp="$(mktemp)"
awk -v v="$version" '
  /^var version = ".*"$/ { print "var version = \"" v "\""; next }
  { print }
' cmd/cli-365/version.go > "$tmp"
mv "$tmp" cmd/cli-365/version.go

