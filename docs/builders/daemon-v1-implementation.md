---
type: How-to
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-15
next_review_by: 2026-03-01
source_of_truth: ./specs/daemon-v1.md
read_when: Starting daemon mode implementation work.
---

# Implement daemon mode v1

This page translates `docs/builders/specs/daemon-v1.md` into coding order.  
Status: in progress (Phase A complete, Phase B queue/transport + in-process dispatch complete, Phase C `CDP_PORT_MISMATCH` guard complete, Phase D auth coordinator complete at unit level, Phase F retry/no-replay + redacted logging + graceful stop queue-drain baseline complete)

## Scope

- Implement daemon path behind global flag: `--daemon`.
- Keep non-daemon behavior unchanged.
- Linux/macOS only for v1.

## Expected user-facing contract (v1)

- `cli-365 --daemon <existing-command>` routes through daemon.
- `cli-365 daemon run|status|stop` available (`ping` optional).
- Queue/full/auth/cdp mismatch return stable error codes.

## Suggested file layout

- `cmd/cli-365/daemon.go` (daemon command wiring)
- `internal/daemon/ipc.go` (UDS request/response protocol)
- `internal/daemon/server.go` (lifecycle, single-instance lock)
- `internal/daemon/queue.go` (bounded FIFO + pause/drain-fail)
- `internal/daemon/worker.go` (single execution lane + timeout)
- `internal/daemon/auth_recovery.go` (pause/recover/timeout flow)
- `internal/daemon/coalesce.go` (read coalescing only)
- `internal/daemon/flood_control.go` (duplicate write + rate limits)
- `internal/daemon/*_test.go` (unit + integration + contract tests)

## Phased implementation checklist

### Phase A: Skeleton + IPC

- [x] Add global `--daemon` flag.
- [x] Add `daemon run|status|stop` commands.
- [x] Add UDS socket + lock + pid/status paths under XDG state.
- [x] Enforce file/socket permissions (`0700` dir, `0600` files).
- [x] Add tests for single-instance lock behavior.

### Phase B: Queue + dispatch

- [x] Build bounded FIFO queue (`max_queue_size`, default 64).
- [x] Return deterministic `QUEUE_FULL`.
- [x] Single worker goroutine; in-process dispatch (no shell recursion).
- [x] Add per-request timeout (default 2m).
- [x] Queue tests: FIFO, capacity, pause/resume, drain-fail.

### Phase C: Browser ownership + CDP consistency

- [ ] Daemon owns browser/tab/session state. (baseline primary-tab manager added in daemon worker path)
- [ ] Reuse one primary OWA tab. (daemon now selects/tracks a primary OWA tab and closes extra OWA/about:blank tabs after browser-affecting commands)
- [ ] Recover tab/browser crash paths.
- [x] Enforce `--cdp-port` mismatch error (`CDP_PORT_MISMATCH`).
- [x] Enforce `DISPLAY=:1` for daemon-managed browser connections.
- [x] Ensure temporary pages are closed after use (extra OWA/about:blank cleanup baseline).
- [ ] Add integration test for crash-recovery.

### Phase D: Auth recovery + notifications

- [x] Auth state machine: `READY -> AUTH_RECOVERING -> READY|AUTH_FAILED`.
- [x] Pause queue consumption on auth required.
- [x] Reject new work while paused (`AUTH_PAUSED`).
- [x] Trigger secure-input command + OpenClaw CLI notification.
- [x] Timeout fan-out failure (`AUTH_TIMEOUT`) after 5m default.
- [x] Add daemon IPC integration tests for paused/timeout auth recovery responses.
- [x] Validate default OpenClaw CLI notifier invocation contract with command-arg tests.

### Phase E: Coalescing + flood controls

- [x] Coalesce identical read-class commands only.
- [x] Keep write commands non-coalesced.
- [x] Add duplicate write suppression windows:
  - mail writes: 12h
  - calendar writes: 1h
- [x] Add `--allow-duplicate-write` override.
- [x] Add per-recipient and global write rate limits.

### Phase F: Retry + hardening

- [x] Read-command retry/backoff for transient `429/5xx`.
- [x] No automatic replay for non-idempotent writes.
- [x] Redact tokens/canary in all logs.
- [x] Add payload size limits and command table validation.
- [x] Bound in-memory response buffering for large outputs.
- [x] Graceful stop with queue drain-fail policy for pending requests (browser cleanup still pending browser/session manager work).
- [ ] Complete contract tests to match non-daemon command semantics. (deterministic parity currently covers `help`, unknown command, missing help topic, `auth status`, `browser status` text/json, and help topics)

## Test gate before merge

- `go test ./...`
- Unit tests for queue/coalescing/flood-control/auth coordinator.
- Integration tests for auto-start, reuse, recovery, timeout behavior.
- Contract tests for output parity vs non-daemon mode.
  - stable daemon error-code contract coverage now includes `QUEUE_FULL`, `AUTH_PAUSED`, `AUTH_TIMEOUT`, `CDP_PORT_MISMATCH`, `DAEMON_UNAVAILABLE`, `REQUEST_TIMEOUT`.
