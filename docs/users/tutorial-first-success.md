---
type: Tutorial
primary_audience: Users
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../../README.md
read_when: First time using cli-365, or setting up a new machine/profile.
---

# First Success: Login And Read Mail

## Overview

Goal: login once, run first mail search, open one message.

## Prerequisites

- `cli-365` installed and on `PATH`
- GUI session available for browser login
- Corporate OWA account access

On devbox GUI sessions, prefix commands with:

```bash
DISPLAY=:1 XAUTHORITY=$HOME/.Xauthority
```

## Steps

1. Login.

```bash
cli-365 auth login
```

1. Confirm auth.

```bash
cli-365 auth status
```

1. Search inbox.

```bash
cli-365 mail search "invoice" --limit 5
```

1. Open one result by index.

```bash
cli-365 mail view --index 1
```

## Validation

Success signals:

- `auth status` shows authenticated.
- `mail search` returns results (or empty list, no auth error).
- `mail view --index 1` prints message details.

## Next steps

- Mail tasks: `docs/users/howto-mail-workflows.md`
- Calendar tasks: `docs/users/howto-calendar-workflows.md`
- Automation and advanced flags: `docs/users/reference-cli-patterns.md`
