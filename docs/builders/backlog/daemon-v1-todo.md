---
type: How-to
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-15
next_review_by: 2026-02-22
source_of_truth: ../specs/daemon-v1.md
read_when: Selecting next daemon implementation task.
---

# Daemon v1 TODO

## Daemon v1 implementation backlog (2026-02-15)

Source of truth: `docs/builders/specs/daemon-v1.md`

## Pre-dev gate

- [ ] Confirm baseline branch builds/tests pass before daemon work starts (`go test ./...`).
- [ ] Confirm secure-input config path is valid (`auth.secure_input`) for auth recovery flow.
- [ ] Confirm secure-input dependency contract: source repo `/path/to/projects/secure-targeted-input`, daemon target binary `secure-targeted-input` resolved from `PATH`.
- [ ] Confirm OpenClaw CLI is available on target dev/test hosts.

### Phase A: daemon skeleton + IPC

- [ ] Add global `--daemon` flag.
- [ ] Add `daemon run|status|stop` commands.
- [ ] Add optional `daemon ping` command for health checks.
- [ ] Add UDS socket + lock + status file lifecycle.
- [ ] Enforce single instance via lock file.
- [ ] Enforce runtime permissions (`0700` state dir, `0600` socket/lock/status).
- [ ] Implement `--daemon` client flow: connect -> auto-start if absent -> submit -> await response.
- [ ] Scope guard v1 to Linux/macOS.
- [ ] Add daemon config schema + defaults (`max_queue_size`, `default_command_timeout`, `auth_recovery_timeout`, `display`, notify config).

### Phase B: queue + worker dispatch

- [ ] Implement bounded FIFO queue (`max_queue_size = 64` default).
- [ ] Return deterministic `QUEUE_FULL` on enqueue overflow.
- [ ] Route commands through in-process dispatcher (no recursive shell exec).
- [ ] Add request timeout (`default_command_timeout = 2m`).
- [ ] Add queue pause/resume + drain-fail semantics for auth timeout/shutdown.
- [ ] Implement request/response envelope contract (`request_id`, timing fields, `queue_wait_ms`, `exec_ms`).
- [ ] Return stable daemon transport codes (`DAEMON_UNAVAILABLE`, `REQUEST_TIMEOUT`).

### Phase C: browser/session ownership

- [ ] Daemon owns one browser + one primary OWA tab.
- [ ] Add health/recovery for closed tab and dead browser.
- [ ] Enforce daemon/client `--cdp-port` consistency.
- [ ] Enforce `DISPLAY=:1` for daemon-managed browser connections.
- [ ] Ensure temporary pages are closed after use.
- [ ] Add token/session manager flow (session-valid probe + proactive access token refresh before expiry).

### Phase D: auth recovery path

- [ ] Pause queue on auth-required signal.
- [ ] Reject new requests while paused (`AUTH_PAUSED`).
- [ ] Invoke secure input (`secure-targeted-input` binary from `PATH`) + OpenClaw notification.
- [ ] Timeout pending requests at `auth_recovery_timeout = 5m` (`AUTH_TIMEOUT`).
- [ ] Model explicit state machine transitions (`READY -> AUTH_RECOVERING -> READY|AUTH_FAILED`).
- [ ] Fail all queued pending requests on auth timeout (fan-out).
- [ ] Include login URL + queue depth in auth-required/auth-timeout notifications.

### Phase E: coalescing + flood controls

- [ ] Coalesce identical read-class requests only.
- [ ] Keep write requests non-coalesced.
- [ ] Keep `mail attachments download` non-coalesced (filesystem side effects).
- [ ] Implement coalesce key normalization (command path, semantic args, identity context, output mode).
- [ ] Add duplicate write suppression windows (mail 12h, calendar 1h).
- [ ] Add global override flag: `--allow-duplicate-write`.
- [ ] Add per-recipient + global write throttles.
- [ ] Return deterministic `DUPLICATE_WRITE_SUSPECTED` on duplicate suppression hits.

### Phase F: hardening + parity tests

- [ ] Retry/backoff for read-command transient `429/5xx`.
- [ ] No automatic replay for non-idempotent writes.
- [ ] Redact token/canary from logs.
- [ ] Emit structured daemon logs without auth/token leakage.
- [ ] Keep allowlist/readonly enforcement server-side in daemon path.
- [ ] Add IPC payload size limits and command-table validation.
- [ ] Add panic guard around request execution.
- [ ] Bound in-memory response buffering for large outputs.
- [ ] Implement graceful daemon stop with queue drain policy + browser cleanup.
- [ ] Add contract tests for daemon vs non-daemon output parity.

## Required test stories

- [ ] Unit: queue FIFO/capacity/pause-resume/drain-fail.
- [ ] Unit: coalescing (reads coalesce, writes do not).
- [ ] Unit: flood controls (duplicate window + per-recipient/global buckets).
- [ ] Unit: auth recovery coordinator transitions and timeout fan-out.
- [ ] Unit: `CDP_PORT_MISMATCH` path.
- [ ] Integration: first `--daemon` call auto-starts daemon.
- [ ] Integration: later calls reuse same daemon/browser/tab.
- [ ] Integration: browser crash recovery.
- [ ] Integration: auth-required triggers pause + secure-input + notifier.
- [ ] Integration: auth timeout fails pending requests with stable error codes.
- [ ] Contract: supported commands keep non-daemon output/exit semantics (latency/metadata excluded).
- [ ] Contract: stable daemon error codes are emitted (`QUEUE_FULL`, `AUTH_PAUSED`, `AUTH_TIMEOUT`, `CDP_PORT_MISMATCH`, `DAEMON_UNAVAILABLE`, `REQUEST_TIMEOUT`).

## Definition of done (v1 readiness)

- [ ] End-to-end `--daemon` works on Linux and macOS.
- [ ] Queue is FIFO + bounded with deterministic overflow behavior.
- [ ] Auth pause/recovery path works with timeout fan-out errors.
- [ ] `--cdp-port` mismatch is enforced.
- [ ] OpenClaw notifications fire on auth-required and auth-timeout.
- [ ] No bearer/canary token leakage in daemon logs.
