# MVP Status

## Mail

Legend: ✅ working | ⚠️ partial | ❌ failing | ⏳ not tested

| Capability | Status | Notes |
| --- | --- | --- |
| Search (basic query) | ✅ | `mail search "test"` works |
| Search (filters: from/since/etc) | ✅ | `--from`, `--since` works |
| Search limit | ✅ | `--limit` respected even after query |
| View message | ✅ | `mail view --index` works |
| Thread get | ❌ | `mail thread get --index` fails (message not found) |
| Reply | ✅ | `mail reply` works (Send via CreateItem) |
| Send (direct) | ✅ | `mail send` works after payload update |
| Draft create | ✅ | `mail draft create` works |
| Draft update | ✅ | `mail draft update` works |
| Draft send | ❌ | `mail draft send` returns 404 |
| Draft delete | ❌ | `mail draft delete` returns 500 |
| Attachments list | ✅ | `mail attachments list` works |
| Attachments download | ❌ | `mail attachments download` returns 500 |
