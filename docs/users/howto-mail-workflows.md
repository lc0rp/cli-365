---
type: How-to
primary_audience: Users
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../../README.md
read_when: >-
  You need common mail operations (search, view, reply, send) without guessing
  flags.
---

# How To Run Core Mail Workflows

## Overview

Use this page for high-frequency mail tasks.

## Prerequisites

- `cli-365 auth status` returns authenticated
- You have run at least one `mail search` if you plan to use `--index`/`#N`.

## Steps

1. Search with filters.

```bash
cli-365 mail search --from "alice@example.com" --since 2026-01-01 --limit 20
cli-365 mail search --subject "Q4" --has-attachments
cli-365 mail search --query 'from:"alice@example.com" subject:"Q4"'
```

1. Open one message.

```bash
cli-365 mail view <message-id>
cli-365 mail view --index 2
cli-365 mail view #2
```

1. Reply.

```bash
cli-365 mail reply <message-id> --body "Thanks, received."
cli-365 mail reply --index 2 --body "Looks good"
cli-365 mail reply #2 "Looks good"
```

1. Send a new mail.

```bash
cli-365 mail send --to "user@example.com" --subject "Status" --body "Done"
```

1. Draft flow (optional).

```bash
cli-365 mail draft create --to "user@example.com" \
  --subject "Draft" --body "Body"
cli-365 mail draft update <draft-id> --body "Updated"
cli-365 mail draft send <draft-id>
```

## Validation

- Search returns expected conversation/message rows.
- Reply/send returns success and appears in Sent Items.

## Troubleshooting

- `401`/auth errors: run `cli-365 auth login` then retry.
- `#N` or `--index` fails: rerun `mail search` to refresh cached results.
- Query mismatch: use `--query` for exact raw Outlook query syntax.

## Next steps

- Advanced query and JSON patterns: `docs/users/reference-cli-patterns.md`
- Full troubleshooting: `docs/users/howto-troubleshoot.md`
