---
type: How-to
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-17
next_review_by: 2026-03-17
source_of_truth: ../scripts/release.sh
read_when: Preparing a patch/minor/major release and git tag.
---

# Releasing cli-365

Use the release script to reduce manual error:

```bash
scripts/release.sh patch
scripts/release.sh minor
scripts/release.sh major
```

Optional flags:

```bash
scripts/release.sh patch --dry-run
scripts/release.sh patch --push
```

## What the script does

1. Validates clean git state on `main`.
2. Fetches tags from `origin` and verifies branch is not behind upstream.
3. Bumps `VERSION` (semver).
4. Updates `cmd/cli-365/version.go`.
5. Generates a new release section in `CHANGELOG.md`.
6. Runs full test gate: `go test ./...`.
7. Builds release binary with stamped version and verifies `--version`.
8. Commits release files and creates annotated tag `vX.Y.Z`.
9. Optionally pushes commit and tag when `--push` is set.

## Files touched per release

- `VERSION`
- `CHANGELOG.md`
- `cmd/cli-365/version.go`

## After release

If you did not use `--push`, publish manually:

```bash
git push origin HEAD
git push origin vX.Y.Z
```

Then create a GitHub Release from the pushed tag.
