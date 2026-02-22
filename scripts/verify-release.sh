#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "verify-release error: expected semantic version (X.Y.Z), got: ${version:-<empty>}" >&2
  exit 1
fi

root_go_files="$(find . -maxdepth 1 -type f -name '*.go' -printf '%P\n' | sort)"
if [[ -n "$root_go_files" ]]; then
  echo "verify-release error: unexpected Go files at repo root:" >&2
  echo "$root_go_files" >&2
  echo "move ad-hoc probes into .scratch/ before running release verification" >&2
  exit 1
fi

go test ./...
go build -ldflags "-X main.version=${version}" -o /tmp/cli-365-release ./cmd/cli-365
actual="$(/tmp/cli-365-release --version)"
if [[ "$actual" != *"$version"* ]]; then
  echo "verify-release error: version output mismatch: $actual" >&2
  exit 1
fi
