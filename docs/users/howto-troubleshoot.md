---
type: How-to
primary_audience: Users
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../../README.md
read_when: >-
  Commands fail with auth/browser/daemon/calendar errors and you need a safe
  recovery sequence.
---

# How To Troubleshoot cli-365

## Overview

Run this sequence top-down. Stop when the issue is resolved.

## Prerequisites

- Terminal access on the host where `cli-365` runs
- Ability to open browser session if re-auth is needed

## Steps

1. Check auth, daemon, browser state.

```bash
cli-365 auth status
cli-365 daemon status
cli-365 browser status
```

1. If auth invalid/expired, re-login.

```bash
cli-365 auth login
```

1. If daemon stuck, restart it.

```bash
cli-365 daemon stop
cli-365 daemon start
cli-365 daemon status
```

1. Retry failing command once in direct mode.

```bash
cli-365 --daemon=false mail search "test" --limit 1
```

1. For directory calendar add issues, use default direct mode and explicit
   email.

```bash
cli-365 calendar add-from-directory --email "alice@example.com"
```

1. Capture diagnostics for investigation.

```bash
cli-365 debug discover --out ./owa-templates.json
cli-365 debug capture --netlog ./owa-capture.json
```

## Validation

- Auth status is healthy.
- Daemon responds to `daemon status` and `daemon ping`.
- Failing command reproduces with clearer error or succeeds.

## Troubleshooting map

- `daemon auth recovery failed`: re-login; retry command in direct mode.
- `No help topic for ...`: command name changed; run `cli-365 <group> --help`.
- `failed to parse ...`: capture netlog and include exact error payload.
- `mailbox info unavailable`: prefer `--email` over ambiguous names.

## Next steps

- Usage flow docs: `docs/users/index.md`
- CLI patterns and scripting: `docs/users/reference-cli-patterns.md`
