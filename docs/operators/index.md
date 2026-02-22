---
type: Reference
primary_audience: Operators
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../README.md
read_when: Operating cli-365 daemon/browser/auth in day-2 scenarios.
---

# Operators Hub

## Read when

You need to diagnose, restart, or stabilize runtime behavior on a host.

## Runtime checks

```bash
cli-365 auth status
cli-365 daemon status
cli-365 browser status
```

## Recovery sequence

```bash
cli-365 auth login
cli-365 daemon stop
cli-365 daemon start
cli-365 --daemon=false mail search "healthcheck" --limit 1
```

## Diagnostics capture

```bash
cli-365 debug discover --out ./owa-templates.json
cli-365 debug capture --netlog ./owa-capture.json
```

## Related docs

- `docs/users/howto-troubleshoot.md`
- `docs/RELEASING.md`
- `docs/builders/daemon-v1-validation.md`
