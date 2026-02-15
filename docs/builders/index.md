---
type: Reference
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-15
next_review_by: 2026-03-15
source_of_truth: ./specs/daemon-v1.md
read_when: Working in cli-365 code or planning daemon-mode implementation.
---

# Builders hub

## Current priority

- Daemon mode v1 implementation from `docs/builders/specs/daemon-v1.md` (spec complete, code not started).

## Builder docs

- `docs/builders/daemon-v1-implementation.md` (execution checklist for upcoming implementation)
- `docs/builders/specs/mvp-spec.md` (MVP architecture context)
- `docs/builders/status/mvp-status.md` (current capability status)
- `docs/builders/backlog/daemon-v1-todo.md` (daemon-v1 epics/tasks)
- `docs/builders/backlog/mvp-todo.md` (legacy/current MVP defects)

## Before coding daemon v1

1. Re-read `docs/builders/specs/daemon-v1.md` decision snapshot and non-goals.
2. Implement in phases (A-F) to keep PRs reviewable.
3. Add tests per phase before merging.
