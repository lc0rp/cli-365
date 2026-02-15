---
type: How-to
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-15
next_review_by: 2026-03-01
source_of_truth: ./specs/daemon-v1.md
read_when: Validating daemon v1 readiness across Linux and macOS.
---

# Daemon v1 validation runbook

## Goal

Close final readiness item in `docs/builders/backlog/daemon-v1-todo.md`:

- End-to-end `--daemon` works on Linux and macOS.

## Local deterministic checks

Run full local gate first:

```bash
GOCACHE=/tmp/gocache go test ./...
```

Run cross-build matrix to prove daemon code compiles for both target OS/arch sets:

```bash
GOCACHE=/tmp/gocache GOOS=linux GOARCH=amd64 go build ./cmd/cli-365
GOCACHE=/tmp/gocache GOOS=linux GOARCH=arm64 go build ./cmd/cli-365
GOCACHE=/tmp/gocache GOOS=darwin GOARCH=amd64 go build ./cmd/cli-365
GOCACHE=/tmp/gocache GOOS=darwin GOARCH=arm64 go build ./cmd/cli-365
```

## CI deterministic smoke checks

Workflow: `.github/workflows/daemon-e2e-smoke.yml`

It runs on Linux + macOS and executes:

- `go test ./internal/daemon -count=1`
- `go test ./cmd/cli-365 -count=1 -run 'TestDaemonAutoStartAndReuseIntegration|TestDaemonInProcessDispatchParity|TestDaemonInProcessDispatchParityAuthStatusWithCachedTokens'`

Workflow observability:

- Publishes a job summary per OS with package-level pass lines.
- Uploads raw logs as artifact: `daemon-smoke-logs-<os>`.

## Close criteria

Mark backlog item done when all are true:

1. Local deterministic checks are green.
2. Cross-build matrix is green.
3. `daemon-e2e-smoke` workflow is green on both `ubuntu-latest` and `macos-latest` for the target commit.

If macOS fails, keep backlog item open and capture failure summary in `docs/builders/backlog/daemon-v1-todo.md`.
