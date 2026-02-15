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
| List events | ✅ | `calendar list` works; server appears to ignore `CalendarView` so client filters by range + applies `--limit` (slower than ideal) |
| Get event | ✅ | `calendar get <ItemId>` works |
| Create event | ✅ | `calendar create` works (returns `ItemId` + `ChangeKey`); `--location` currently not persisted/returned (TBD) |
| Update event | ❌ | `calendar update` fails with `ErrorSendMeetingInvitationsOrCancellationsRequired` (UpdateItem) |
| Delete event | ✅ | `calendar delete <ItemId>` works (MoveToDeletedItems). Note: `calendar get` may still succeed post-delete since the item can be fetched by id in Deleted Items |
