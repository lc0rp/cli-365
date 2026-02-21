---
type: Reference
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-21
next_review_by: 2026-03-01
source_of_truth: ./mvp-status.md
read_when: Need current feature health before selecting next implementation work.
---

# MVP Status

## Recent hardening (2026-02-20)

- Daemon startup now rejects deleting directory/symlink socket paths and only removes safe stale socket file types.
- OWA startup-data canary extraction now supports string payloads and header/JSON-like canary tokens.
- Daemon in-process config flag detection now honors `-c=<path>` without injecting a duplicate `--config`.
- Recipient write-rate tracking now prunes stale recipient buckets to prevent unbounded map growth.
- Folder ID resolution now writes through to `tokens.Folders` cache after first lookup.
- Path home-directory resolution now has deterministic absolute fallbacks when OS home lookup fails.

## Recent hardening (2026-02-21)

- Directory calendar add now uses OWA in-page CreateCalendar module path with GraphQL fallback handling.
- Directory calendar add now warms required calendar modules before mutation when missing.
- Directory calendar add now normalizes OWA results that return `calendarId` without `FolderId`.
- Directory calendar add now deduplicates by existing exact name/email match and returns existing IDs when already added.
- `calendar add-from-directory` / `add-directory` now bypass daemon by default unless `--daemon` is explicitly set.
- `calendar calendars` now includes registry-backed directory calendars (name/email/folder_id/calendar_id) even when absent from live folder list.
- `calendar list` now supports `--calendar` selector (exact name/email/calendar_id) as shortcut to folder filtering.

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
| Add directory calendar | ✅ | Live verified on 2026-02-21 with `calendar add-from-directory user@example.com`; returns stable folder/calendar IDs and reports `already added` on repeat |
