# MVP Status

## Mail

Legend: ✅ working | ⚠️ partial | ❌ failing | ⏳ not tested

| Capability | Status | Notes |
| --- | --- | --- |
| Search (basic query) | ✅ | `mail search "test"` works |
| Search (filters: from/since/etc) | ✅ | `--from`, `--since` works |
| Search limit | ⚠️ | `--limit` seems ignored (returned > limit) |
| View message | ❌ | `mail view` returns 500 |
| Thread get | ❌ | `mail thread get` returns 500 |
| Reply | ✅ | `mail reply` works (Send via CreateItem) |
| Send (direct) | ✅ | `mail send` works after payload update |
| Draft create | ❌ | `mail draft create` returns 500 |
| Draft update | ⏳ | not tested (blocked by create) |
| Draft send | ⏳ | not tested (blocked by create) |
| Draft delete | ⏳ | not tested (blocked by create) |
| Attachments list | ❌ | blocked by `mail view` 500 |
| Attachments download | ⏳ | not tested (blocked by list) |

