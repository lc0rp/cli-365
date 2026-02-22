---
type: Reference
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../README.md
read_when: Start docs work and route to the right audience/intention page.
---

# cli-365 docs hub

Audience-first routing for repository docs. Keep pages intent-pure:

- Tutorial: guided first success.
- How-to: one concrete task.
- Reference: lookup facts/contracts/flags.
- Concept: explain model/tradeoffs.

## Read when

- Planning daemon implementation: `docs/builders/specs/daemon-v1.md`,
  `docs/builders/daemon-v1-implementation.md`,
  `docs/builders/backlog/daemon-v1-todo.md`.
- Verifying current CLI behavior: `README.md`,
  `docs/builders/status/mvp-status.md`, `docs/builders/specs/mvp-spec.md`,
  `docs/builders/backlog/mvp-todo.md`.
- Cutting a release/tag: `docs/RELEASING.md`.

## Audience hubs

- Builders: `docs/builders/index.md`
- Release: `docs/RELEASING.md`
- Testers: pending (add once daemon v1 tests land)
- Operators: pending (add once daemon runbook exists)
- Users: `docs/users/index.md`

## Maintenance rules

- One primary audience per page.
- One owner/team per page.
- Keep `last_verified` and `next_review_by` current.
- Add `read_when` on cross-cutting pages.
