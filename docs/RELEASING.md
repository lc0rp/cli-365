---
type: How-to
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-17
next_review_by: 2026-03-17
source_of_truth: ../.releaserc.cjs
read_when: Preparing a release or validating semantic-release/commitlint automation.
---

# Releasing cli-365

Releases are fully semantic-release based. No manual `patch|minor|major` input.

## How version bumps are decided

- `fix:` and `perf:` commits => patch release
- `feat:` commits => minor release
- `BREAKING CHANGE:` footer or `!` marker => major release

Commit format is enforced by commitlint in CI.

## Default release flow

1. Merge Conventional Commit messages into `main`.
2. GitHub Actions `release.yml` runs semantic-release.
3. semantic-release:
   - computes next semver from commits
   - updates `CHANGELOG.md`
   - updates `VERSION` and `cmd/cli-365/version.go`
   - runs release verification (`go test ./...`, build, `--version` check)
   - creates commit `chore(release): X.Y.Z`
   - creates annotated tag `vX.Y.Z`
   - publishes GitHub Release

## Local preview

```bash
scripts/release.sh --dry-run
```

## Manual local run (rare)

```bash
scripts/release.sh
```

Requires `GITHUB_TOKEN` with repo write permissions.

## Files managed by release automation

- `CHANGELOG.md`
- `VERSION`
- `cmd/cli-365/version.go`

