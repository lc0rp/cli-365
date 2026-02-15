---
type: How-to
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-15
next_review_by: 2026-02-22
source_of_truth: ../status/mvp-status.md
read_when: Working MVP defects/regression checks outside daemon-v1 epics.
---

# MVP TODO

## Legacy MVP issues (from root `TODO.md`, 2026-01-29)

- [ ] `mail view` returns 500 for messages fetched from search/index.
- [ ] `mail thread get` returns 500 using conversation IDs from search.
- [ ] `mail attachments list` fails because `mail view`/`GetMessage` returns 500.
- [ ] `mail draft create` returns 500.
- [ ] `mail send` previously returned 500 (fixed at time of note); keep regression check.
- [ ] `mail search --limit` appears to ignore limit (returned > limit).

## Current known MVP issues (carry-forward)

- [ ] `mail draft send` fails with 404 on some accounts.
- [ ] `mail draft delete` fails with 500 on some accounts.
- [ ] `mail attachments download` fails with 500 on some accounts.
- [ ] Re-verify `mail thread get` reliability from cached conversation IDs.
