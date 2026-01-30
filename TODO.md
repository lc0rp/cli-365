# TODO

## MVP Issues (2026-01-29)

- `mail view` returns 500 for messages fetched from search/index.
- `mail thread get` returns 500 using conversation IDs from search.
- `mail attachments list` fails because `mail view`/`GetMessage` returns 500.
- `mail draft create` returns 500.
- `mail send` previously returned 500 (now fixed) — keep an eye on regressions.
- `mail search --limit` appears to ignore limit (returned > limit).

