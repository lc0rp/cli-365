---
type: Reference
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ./status/mvp-status.md
read_when: Working in cli-365 code and selecting the next highest-leverage work.
---

# Builders hub

## Current priority

- Stabilization and maintainability hardening after daemon-v1 rollout.
- Keep CI/test/docs truth in sync with runtime behavior.

## Builder docs

- `docs/builders/backlog/high-leverage-improvements.md`
  (prioritized improvements from latest scan)
- `docs/builders/daemon-v1-implementation.md`
  (implementation checklist and phase history)
- `docs/RELEASING.md` (semantic-release + commitlint workflow)
- `docs/builders/specs/mvp-spec.md` (legacy MVP snapshot; historical context only)
- `docs/builders/status/mvp-status.md` (current capability status)
- `docs/builders/backlog/daemon-v1-todo.md` (daemon-v1 epics/tasks)
- `docs/builders/backlog/mvp-todo.md` (legacy/current MVP defects)

## Next engineering priorities

1. Land CI/runtime alignment tasks from `docs/builders/backlog/high-leverage-improvements.md`.
2. Keep docs hubs and status pages aligned with shipped behavior.
3. Split oversized files only after coverage remains stable.
