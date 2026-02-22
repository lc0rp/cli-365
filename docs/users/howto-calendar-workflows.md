---
type: How-to
primary_audience: Users
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../../README.md
read_when: >-
  You need to list events, add directory calendars, and target the right
  calendar reliably.
---

# How To Run Core Calendar Workflows

## Overview

Use this page for default calendar plus added directory calendars.

## Prerequisites

- `cli-365 auth status` returns authenticated
- For coworker calendars, know exact email (best) or exact display name

## Steps

1. List events from default calendar.

```bash
cli-365 calendar list
cli-365 calendar list --start 2026-02-22 --end 2026-02-28 --limit 50
```

1. List available calendars/folders.

```bash
cli-365 calendar calendars
cli-365 --json calendar calendars
```

1. Add a directory calendar.

```bash
cli-365 calendar add-from-directory --email "alice@example.com"
cli-365 calendar add-from-directory --name "Alice Adams"
```

1. List events for a non-default calendar.

Use selector by name/email/calendar_id:

```bash
cli-365 calendar list --calendar "Alice Adams" --start 2026-02-22 --end 2026-02-28
cli-365 calendar list --calendar "alice@example.com" --start 2026-02-22 --end 2026-02-28
```

Or use `folder_id` from `calendar calendars` output:

```bash
cli-365 calendar list --folder "<folder-id>" --start 2026-02-22 --end 2026-02-28
```

1. Create/update/delete events.

```bash
cli-365 calendar create --subject "Standup" \
  --start 2026-02-23T09:00:00-05:00 --end 2026-02-23T09:15:00-05:00
cli-365 calendar update <event-id> --subject "Standup (updated)"
cli-365 calendar delete <event-id>
```

## Validation

- `calendar calendars` shows the added calendar entry and `folder_id`.
- `calendar list --folder ...` returns events for that calendar.

## Troubleshooting

- Name lookup ambiguous: use `--email` or `--allow-ambiguous`.
- `Calendar already added`: command found exact existing name/email and avoided
  duplicate add.
- `daemon auth recovery failed` during directory add: retry without forcing
  daemon (`add-from-directory` defaults to direct mode).
- `flag provided but not defined: -calendar`: daemon may be running an older
  binary; run `cli-365 daemon stop` then `cli-365 daemon start`.

## Next steps

- Selector and jq patterns: `docs/users/reference-cli-patterns.md`
- Full troubleshooting: `docs/users/howto-troubleshoot.md`
