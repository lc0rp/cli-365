#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/release.sh [patch|minor|major] [--push] [--dry-run]

Automated release workflow:
1. Validates clean git state on main and fetches tags.
2. Bumps VERSION (semantic version).
3. Updates cmd/cli-365/version.go default version.
4. Generates CHANGELOG.md release entry from git commits.
5. Runs go test ./...
6. Builds release binary and verifies --version output.
7. Commits release files and creates annotated tag vX.Y.Z.
8. Optionally pushes commit + tag with --push.
EOF
}

die() {
  printf 'release error: %s\n' "$1" >&2
  exit 1
}

ensure_repo_root() {
  local root
  root="$(git rev-parse --show-toplevel 2>/dev/null)" || die "not inside a git repository"
  cd "$root"
}

validate_clean_tree() {
  if [[ -n "$(git status --porcelain)" ]]; then
    die "working tree is not clean"
  fi
}

validate_branch() {
  local branch
  branch="$(git rev-parse --abbrev-ref HEAD)"
  [[ "$branch" == "main" ]] || die "releases must run from main (current: $branch)"
}

fetch_remote_tags() {
  git fetch --quiet --tags origin
}

validate_not_behind_upstream() {
  local upstream behind
  upstream="$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)"
  [[ -n "$upstream" ]] || return 0
  behind="$(git rev-list --count HEAD.."$upstream")"
  [[ "$behind" -eq 0 ]] || die "branch is behind $upstream by $behind commit(s)"
}

validate_semver() {
  local v="$1"
  [[ "$v" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "invalid semver: $v"
}

bump_version() {
  local current="$1" bump="$2"
  local major minor patch
  IFS='.' read -r major minor patch <<<"$current"
  case "$bump" in
    patch) patch=$((patch + 1)) ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    major) major=$((major + 1)); minor=0; patch=0 ;;
    *) die "unknown bump type: $bump" ;;
  esac
  printf '%s.%s.%s\n' "$major" "$minor" "$patch"
}

update_version_go() {
  local new_version="$1"
  local file="cmd/cli-365/version.go"
  [[ -f "$file" ]] || die "$file not found"

  local tmp
  tmp="$(mktemp)"
  awk -v new_version="$new_version" '
    /^var version = ".*"$/ { print "var version = \"" new_version "\""; next }
    { print }
  ' "$file" > "$tmp"
  mv "$tmp" "$file"
}

ensure_changelog_file() {
  if [[ ! -f CHANGELOG.md ]]; then
    cat > CHANGELOG.md <<'EOF'
# Changelog

All notable changes to this project are documented in this file.

The format follows Keep a Changelog and semantic versioning (`vMAJOR.MINOR.PATCH` tags).

## Unreleased
EOF
  fi
}

collect_release_notes() {
  local last_tag="$1"
  if [[ -n "$last_tag" ]]; then
    git log --pretty=format:'- %s (%h)' "${last_tag}..HEAD"
  else
    git log --pretty=format:'- %s (%h)'
  fi
}

update_changelog() {
  local new_version="$1" release_date="$2" notes="$3"
  ensure_changelog_file

  local entry_file tmp
  entry_file="$(mktemp)"
  {
    printf '## [%s] - %s\n\n' "$new_version" "$release_date"
    printf '### Commits\n'
    if [[ -n "$notes" ]]; then
      printf '%s\n' "$notes"
    else
      printf -- '- No user-visible commit messages found.\n'
    fi
  } > "$entry_file"

  tmp="$(mktemp)"
  local inserted=0
  while IFS= read -r line || [[ -n "$line" ]]; do
    printf '%s\n' "$line" >> "$tmp"
    if [[ "$inserted" -eq 0 && "$line" == "## Unreleased" ]]; then
      printf '\n' >> "$tmp"
      cat "$entry_file" >> "$tmp"
      printf '\n' >> "$tmp"
      inserted=1
    fi
  done < CHANGELOG.md

  if [[ "$inserted" -eq 0 ]]; then
    printf '\n' >> "$tmp"
    cat "$entry_file" >> "$tmp"
    printf '\n' >> "$tmp"
  fi

  mv "$tmp" CHANGELOG.md
  rm -f "$entry_file"
}

run_release_gate() {
  local new_version="$1"
  go test ./...
  go build -ldflags "-X main.version=${new_version}" -o /tmp/cli-365-release ./cmd/cli-365
  local output
  output="$(/tmp/cli-365-release --version)"
  [[ "$output" == *"$new_version"* ]] || die "version check failed: expected $new_version, got: $output"
}

commit_and_tag() {
  local new_version="$1"
  local tag="v${new_version}"

  git add VERSION CHANGELOG.md cmd/cli-365/version.go
  git commit -m "chore(release): ${tag}" -- VERSION CHANGELOG.md cmd/cli-365/version.go
  git tag -a "$tag" -m "Release ${tag}"
}

push_release() {
  local new_version="$1"
  local tag="v${new_version}"
  git push origin HEAD
  git push origin "$tag"
}

main() {
  [[ $# -ge 1 ]] || { usage; exit 1; }
  if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    usage
    exit 0
  fi

  local bump_type="$1"
  shift
  local push_release_flag=0
  local dry_run=0

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --push) push_release_flag=1 ;;
      --dry-run) dry_run=1 ;;
      -h|--help) usage; exit 0 ;;
      *) die "unknown argument: $1" ;;
    esac
    shift
  done

  case "$bump_type" in
    patch|minor|major) ;;
    *) die "bump type must be one of: patch, minor, major" ;;
  esac

  ensure_repo_root
  validate_clean_tree
  validate_branch
  fetch_remote_tags
  validate_not_behind_upstream

  [[ -f VERSION ]] || die "VERSION file not found"
  local current_version new_version tag release_date last_tag notes
  current_version="$(tr -d '[:space:]' < VERSION)"
  validate_semver "$current_version"
  new_version="$(bump_version "$current_version" "$bump_type")"
  validate_semver "$new_version"
  tag="v${new_version}"

  if git rev-parse "$tag" >/dev/null 2>&1; then
    die "tag already exists: $tag"
  fi

  last_tag="$(git describe --tags --abbrev=0 2>/dev/null || true)"
  notes="$(collect_release_notes "$last_tag")"
  release_date="$(date -u +%Y-%m-%d)"

  printf 'Current version: %s\n' "$current_version"
  printf 'Next version:    %s\n' "$new_version"
  printf 'Last tag:        %s\n' "${last_tag:-<none>}"

  if [[ "$dry_run" -eq 1 ]]; then
    printf '\n[Dry run] Release notes preview:\n%s\n' "${notes:-<none>}"
    exit 0
  fi

  printf '%s\n' "$new_version" > VERSION
  update_version_go "$new_version"
  update_changelog "$new_version" "$release_date" "$notes"

  run_release_gate "$new_version"
  commit_and_tag "$new_version"

  if [[ "$push_release_flag" -eq 1 ]]; then
    push_release "$new_version"
    printf 'Release pushed: %s\n' "$tag"
  else
    printf 'Release committed and tagged locally: %s\n' "$tag"
    printf 'Push with:\n  git push origin HEAD\n  git push origin %s\n' "$tag"
  fi
}

main "$@"
