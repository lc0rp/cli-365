---
type: Reference
primary_audience: Testers
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../README.md
read_when: Running validation gates, smoke tests, and regression checks.
---

# Testers Hub

## Read when

You need deterministic local/CI validation coverage before merge or release.

## Core gates

```bash
go test ./cmd/... ./internal/... -count=1
go test ./internal/daemon -count=1
go test ./cmd/cli-365 -count=1 -run 'TestDaemonAutoStartAndReuseIntegration|TestDaemonInProcessDispatchParity|TestDaemonInProcessDispatchParityAuthStatusWithCachedTokens'
```

## CI workflows

- Commit format gate: `.github/workflows/commitlint.yml`
- Go quality gate: `.github/workflows/go-ci.yml`
- Daemon smoke gate (Linux + macOS): `.github/workflows/daemon-e2e-smoke.yml`

## Known pitfalls

- Root-level `tmp_*.go` probe files will break `go test ./...`.
  Keep ad-hoc probes under `.scratch/`.

## Related docs

- `docs/builders/status/mvp-status.md`
- `docs/builders/backlog/mvp-todo.md`
- `docs/builders/daemon-v1-validation.md`
