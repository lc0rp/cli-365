---
type: Reference
primary_audience: Users
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../../README.md
read_when: >-
  You need exact CLI patterns for scripting, JSON automation, daemon routing,
  and safety controls.
---

# CLI Patterns Reference

## Summary

Lookup page for stable command patterns. Keep scripts deterministic:
explicit date ranges, explicit limits, `--json` + `jq` for selection.

## Global flags

Use before subcommands:

- `--json`
- `--readonly`
- `--cdp-port`
- `--daemon`
- `--allow-duplicate-write`

Example:

```bash
cli-365 --json --daemon=false mail search "invoice" --limit 5
```

## Calendar targeting pattern

Preferred path:

1. Use `--calendar` with exact name/email/calendar_id.

Example:

```bash
cli-365 calendar list --calendar "Alice Adams" --start 2026-02-22 --end 2026-02-28
cli-365 calendar list --calendar "alice@example.com" --start 2026-02-22 --end 2026-02-28
```

Fallback path (when you already have `folder_id`):

```bash
FOLDER_ID=$(cli-365 --json calendar calendars | \
  jq -r '.[] | select(.name=="Alice Adams") | .folder_id')
cli-365 calendar list --folder "$FOLDER_ID" --start 2026-02-22 --end 2026-02-28
```

## Directory calendar add semantics

`calendar add-from-directory`:

- Accepts `--email`, `--name`, or positional `[email-or-name]`.
- Checks existing added calendars first (exact name/email) to avoid duplicates.
- Prints `Folder ID` and, when available, `Calendar ID`.
- Runs direct-mode by default; can be forced to daemon path via global
  `--daemon`.

## Mail search patterns

```bash
cli-365 mail search --from "alice@example.com" --since 2026-02-01 --limit 20
cli-365 mail search --query 'subject:"Quarterly" hasattachment:true'
```

Date formats accepted by query builder flags:

- `YYYY-MM-DD`
- RFC3339

## JSON output notes

- Use `--json` for machine parsing.
- `mail search` includes both `Messages` and `Conversations` arrays.
- `calendar calendars` output includes metadata from live folders plus local
  added-calendar registry.

## Edge cases

- `mail view --index N` and `mail reply #N` require a prior `mail search` cache.
- Daemon bypass default applies to `calendar add-from-directory`.
- `--readonly` blocks write operations (`send`, `draft`, calendar writes).
- If daemon is stale after binary update, restart with `daemon stop/start`.

## Version notes

Validated on release line `1.0.0`.
