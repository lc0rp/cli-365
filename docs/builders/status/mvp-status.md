---
type: Reference
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-15
next_review_by: 2026-02-22
source_of_truth: ./mvp-status.md
read_when: Need current feature health before selecting next implementation work.
---

# MVP Status

## Mail

Legend: ✅ working | ⚠️ partial | ❌ failing | ⏳ not tested

| Capability | Status | Notes |
| --- | --- | --- |
| Search (basic query) | ✅ | `mail search "test"` works |
| Search (filters: from/since/etc) | ✅ | `--from`, `--since` works |
| Search limit | ✅ | `--limit` respected even after query |
| View message | ✅ | `mail view --index` works |
| Thread get | ⏳ | Cache now stores conversation IDs; rerun search and verify |
| Reply | ✅ | `mail reply` works (Send via CreateItem) |
| Send (direct) | ✅ | `mail send` works after payload update |
| Draft create | ✅ | `mail draft create` works |
| Draft update | ✅ | `mail draft update` works |
| Draft send | ❌ | `mail draft send` returns 404 |
| Draft delete | ❌ | `mail draft delete` returns 500 |
| Attachments list | ✅ | `mail attachments list` works |
| Attachments download | ❌ | `mail attachments download` returns 500 |

## Calendar

Legend: ✅ working | ⚠️ partial | ❌ failing | ⏳ not tested

| Capability | Status | Notes |
| --- | --- | --- |
| List events | ⏳ | Needs verification |
| Get event | ⏳ | Needs verification |
| Create event | ⏳ | Needs verification |
| Update event | ⏳ | Needs verification |
| Delete event | ⏳ | Needs verification |
