#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/release.sh [--dry-run]

Run semantic-release for this repository.

Notes:
- Version bump is commit-driven (Conventional Commits), not manual patch/minor/major input.
- dry-run mode previews the next release without writing tags/releases.

Examples:
  scripts/release.sh --dry-run
  scripts/release.sh
EOF
}

dry_run=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) dry_run=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "release error: unknown argument: $1" >&2; usage; exit 1 ;;
  esac
  shift
done

if ! command -v npm >/dev/null 2>&1; then
  echo "release error: npm is required" >&2
  exit 1
fi

if [[ ! -f package.json ]]; then
  echo "release error: package.json not found; run from repo root" >&2
  exit 1
fi

npm ci
if [[ "$dry_run" -eq 1 ]]; then
  npx semantic-release --dry-run --no-ci
else
  if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    echo "release error: GITHUB_TOKEN is required for non-dry-run release" >&2
    exit 1
  fi
  npx semantic-release
fi
