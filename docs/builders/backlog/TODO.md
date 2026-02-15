---
type: How-to
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-15
next_review_by: 2026-02-22
source_of_truth: ../specs/daemon-v1.md
read_when: Selecting next daemon implementation task.
---

# TODO

## Daemon v1 implementation backlog (2026-02-15)

Source of truth: `docs/builders/specs/daemon-v1.md`

### Phase A: daemon skeleton + IPC

- [ ] Add global `--daemon` flag.
- [ ] Add `daemon run|status|stop` commands.
- [ ] Add UDS socket + lock + status file lifecycle.
- [ ] Enforce single instance via lock file.

### Phase B: queue + worker dispatch

- [ ] Implement bounded FIFO queue (`max_queue_size = 64` default).
- [ ] Return deterministic `QUEUE_FULL` on enqueue overflow.
- [ ] Route commands through in-process dispatcher (no recursive shell exec).
- [ ] Add request timeout (`default_command_timeout = 2m`).

### Phase C: browser/session ownership

- [ ] Daemon owns one browser + one primary OWA tab.
- [ ] Add health/recovery for closed tab and dead browser.
- [ ] Enforce daemon/client `--cdp-port` consistency.

### Phase D: auth recovery path

- [ ] Pause queue on auth-required signal.
- [ ] Reject new requests while paused (`AUTH_PAUSED`).
- [ ] Invoke secure input + OpenClaw notification.
- [ ] Timeout pending requests at `auth_recovery_timeout = 5m` (`AUTH_TIMEOUT`).

### Phase E: coalescing + flood controls

- [ ] Coalesce identical read-class requests only.
- [ ] Keep write requests non-coalesced.
- [ ] Add duplicate write suppression windows (mail 12h, calendar 1h).
- [ ] Add global override flag: `--allow-duplicate-write`.
- [ ] Add per-recipient + global write throttles.

### Phase F: hardening + parity tests

- [ ] Retry/backoff for read-command transient `429/5xx`.
- [ ] No automatic replay for non-idempotent writes.
- [ ] Redact token/canary from logs.
- [ ] Add contract tests for daemon vs non-daemon output parity.

## Existing MVP issues to keep in scope

- [ ] `mail draft send` fails with 404 on some accounts.
- [ ] `mail draft delete` fails with 500 on some accounts.
- [ ] `mail attachments download` fails with 500 on some accounts.
- [ ] Re-verify `mail thread get` reliability from cached conversation IDs.
