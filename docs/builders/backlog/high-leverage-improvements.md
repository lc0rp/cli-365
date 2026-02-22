---
type: Reference
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../../README.md
read_when: Selecting highest-leverage maintenance work after feature delivery.
---

# High-Leverage Improvements

Prioritized scan output captured on 2026-02-22.

## P0 (do now)

- [x] Add a PR Go quality gate (`go test ./...`, `go vet`, `gofmt` check).
- [x] Align CI Node runtime to project policy (`>=22`).
- [x] Add root-level Go probe guardrails for release/test verification.
- [x] Refresh builder/docs hubs to reflect current (post-daemon-v1) reality.

## P1 (next)

- [x] Correct Go runtime requirement docs to match `go.mod`.
- [x] Mark MVP planning spec as legacy snapshot (avoid active-spec confusion).
- [x] Add concise tester/operator runbooks for day-2 operations and validation.

## P2 (structural)

- [ ] Split oversized files while preserving coverage:
  - `cmd/cli-365/main.go`
  - `internal/owa/calendar_directory.go`
  - `internal/owa/mail.go`
  - `internal/daemon/server.go`
- [ ] Add doc lint + link-check gate for docs drift prevention.

## Notes

- Local ad-hoc `tmp_*.go` probes at repo root will break `go test ./...`.
  Keep probes under `.scratch/` instead.
