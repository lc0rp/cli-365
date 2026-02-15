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

- [x] Confirm baseline branch builds/tests pass before daemon work starts (`go test ./...`).
- [x] Confirm secure-input config path is valid (`auth.secure_input`) for auth recovery flow.
- [x] Confirm secure-input dependency contract: source repo `/path/to/projects/secure-targeted-input`, daemon target binary `secure-targeted-input` resolved from `PATH`.
- [x] Confirm OpenClaw CLI is available on target dev/test hosts. (daemon startup now preflights notifier command lookup and emits structured `notifier_unavailable` warning when missing)

### Phase A: daemon skeleton + IPC

- [x] Add global `--daemon` flag.
- [x] Add `daemon run|status|stop` commands.
- [x] Add optional `daemon ping` command for health checks.
- [x] Add UDS socket + lock + status file lifecycle.
- [x] Enforce single instance via lock file.
- [x] Enforce runtime permissions (`0700` state dir, `0600` socket/lock/status).
- [x] Implement `--daemon` client flow: connect -> auto-start if absent -> submit -> await response.
- [x] Scope guard v1 to Linux/macOS.
- [x] Add daemon config schema + defaults (`max_queue_size`, `default_command_timeout`, `auth_recovery_timeout`, `display`, notify config).

### Phase B: queue + worker dispatch

- [x] Implement bounded FIFO queue (`max_queue_size = 64` default).
- [x] Return deterministic `QUEUE_FULL` on enqueue overflow.
- [x] Route commands through in-process dispatcher (no recursive shell exec).
- [x] Add request timeout (`default_command_timeout = 2m`).
- [x] Add queue pause/resume + drain-fail semantics for auth timeout/shutdown.
- [x] Implement request/response envelope contract (`request_id`, timing fields, `queue_wait_ms`, `exec_ms`).
- [x] Return stable daemon transport codes (`DAEMON_UNAVAILABLE`, `REQUEST_TIMEOUT`).

### Phase C: browser/session ownership

- [x] Daemon owns one browser + one primary OWA tab. (daemon tracks primary tab, closes extras, runs periodic maintenance between requests, and ties session preflight/recovery to the managed browser runtime)
- [x] Add health/recovery for closed tab and dead browser. (daemon recovers closed-tab state on subsequent browser-managed commands, has guarded crash-recovery integration coverage, and session preflight attempts in-process `browser start` recovery when probe reports unavailable)
- [x] Enforce daemon/client `--cdp-port` consistency.
- [x] Enforce `DISPLAY=:1` for daemon-managed browser connections.
- [x] Ensure temporary pages are closed after use.
- [x] Add token/session manager flow (daemon now parses cached JWT `exp`, refreshes near-expiry token cache from primary OWA tab, and runs a preflight session-valid probe before `mail|calendar`; failed probe triggers auth recovery).

### Phase D: auth recovery path

- [x] Pause queue on auth-required signal.
- [x] Reject new requests while paused (`AUTH_PAUSED`).
- [x] Invoke secure input (`secure-targeted-input` binary from `PATH`) + OpenClaw notification.
- [x] Timeout pending requests at `auth_recovery_timeout = 5m` (`AUTH_TIMEOUT`).
- [x] Model explicit state machine transitions (`READY -> AUTH_RECOVERING -> READY|AUTH_FAILED`).
- [x] Fail all queued pending requests on auth timeout (fan-out).
- [x] Include login URL + queue depth in auth-required/auth-timeout notifications.

### Phase E: coalescing + flood controls

- [x] Coalesce identical read-class requests only.
- [x] Keep write requests non-coalesced.
- [x] Keep `mail attachments download` non-coalesced (filesystem side effects).
- [x] Implement coalesce key normalization (command path, semantic args, identity context, output mode).
- [x] Add duplicate write suppression windows (mail 12h, calendar 1h).
- [x] Add global override flag: `--allow-duplicate-write`.
- [x] Add per-recipient + global write throttles.
- [x] Return deterministic `DUPLICATE_WRITE_SUSPECTED` on duplicate suppression hits.

### Phase F: hardening + parity tests

- [x] Retry/backoff for read-command transient `429/5xx`.
- [x] No automatic replay for non-idempotent writes.
- [x] Redact token/canary from logs.
- [x] Emit structured daemon logs without auth/token leakage.
- [x] Keep allowlist/readonly enforcement server-side in daemon path.
- [x] Add IPC payload size limits and command-table validation.
- [x] Add panic guard around request execution.
- [x] Bound in-memory response buffering for large outputs.
- [x] Implement graceful daemon stop with queue drain policy + browser cleanup. (daemon now runs stop cleanup on shutdown, resets cached tab/browser handles, and only stops managed browser instances when `state_dir` matches `paths.RuntimePath()` parent)
- [x] Add contract tests for daemon vs non-daemon output parity. (deterministic parity now covers `help`, unknown command, missing help topic, top-level command help defaults for `auth|browser|daemon|mail|calendar|debug`, `auth status` text/json for both empty cache and seeded token-cache states, `auth logout`, `browser status` text/json, `browser stop`, `daemon status` text/json, `daemon ping` text/json, and help topic paths for `mail|calendar|auth|browser|daemon|debug`)

## Required test stories

- [x] Unit: queue FIFO/capacity/pause-resume/drain-fail.
- [x] Unit: coalescing (reads coalesce, writes do not).
- [x] Unit: flood controls (duplicate window + per-recipient/global buckets).
- [x] Unit: auth recovery coordinator transitions and timeout fan-out.
- [x] Unit: `CDP_PORT_MISMATCH` path.
- [x] Integration: first `--daemon` call auto-starts daemon.
- [x] Integration: later calls reuse same daemon/browser/tab. (daemon process reuse + repeated `--daemon browser start` primary-tab reuse integration test; host prerequisites required, test auto-skips when unavailable)
- [x] Integration: browser crash recovery. (guarded integration test added; auto-skips when browser host prerequisites unavailable)
- [x] Integration: closed primary-tab recovery. (guarded integration test closes all OWA tabs then verifies daemon recreates a single primary tab)
- [x] Integration: auth-required triggers pause + secure-input + notifier. (daemon IPC integration coverage)
- [x] Integration: auth timeout fails pending requests with stable error codes. (daemon IPC integration coverage)
- [x] Contract: supported commands keep non-daemon output/exit semantics (latency/metadata excluded). (deterministic parity includes top-level command help defaults for `auth|browser|daemon|mail|calendar|debug`, `auth status` text/json in empty+cached-token states, `auth logout`, `browser status` text/json, `browser stop`, `daemon status` text/json, `daemon ping` text/json, and help topics for `mail|calendar|auth|browser|daemon|debug`; non-deterministic authenticated/write flows are covered by integration tests)
- [x] Contract: stable daemon error codes are emitted (`QUEUE_FULL`, `AUTH_PAUSED`, `AUTH_TIMEOUT`, `CDP_PORT_MISMATCH`, `DAEMON_UNAVAILABLE`, `REQUEST_TIMEOUT`).

## Definition of done (v1 readiness)

- [ ] End-to-end `--daemon` works on Linux and macOS.
- [x] Queue is FIFO + bounded with deterministic overflow behavior.
- [x] Auth pause/recovery path works with timeout fan-out errors.
- [x] `--cdp-port` mismatch is enforced.
- [x] OpenClaw notifications fire on auth-required and auth-timeout. (auth recovery path + notifier command invocation tests + startup notifier availability preflight warning)
- [x] No bearer/canary token leakage in daemon logs.
