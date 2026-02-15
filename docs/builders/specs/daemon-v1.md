---
type: Reference
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-15
next_review_by: 2026-03-15
source_of_truth: ./daemon-v1.md
read_when: Need daemon v1 contracts, defaults, architecture, and acceptance criteria.
---

# cli-365 daemon mode v1 spec

Status: Draft (discussion complete, implementation NOT started)
Owner: cli-365
Type: Spec / architecture
Last updated: 2026-02-15

---

## 1) Decision snapshot (agreed)

1. Daemon mode is **initially gated behind `--daemon`**.
2. Notifications use a built-in **OpenClaw CLI invocation path** (expects OpenClaw available on same host).
3. Defaults:
   - `max_queue_size = 64`
   - `auth_recovery_timeout = 5m` (overridable)
   - `default_command_timeout = 2m` (overridable)
   - `duplicate_write_window_mail = 12h`
   - `duplicate_write_window_calendar = 1h`
4. During auth pause: **reject new requests**.
5. Cross-platform target: **Linux + macOS only**.
6. Use `DISPLAY=:1` for managed browser connections.
7. Duplicate-write override flag is global: `--allow-duplicate-write`.
8. `mail attachments download` is **not** coalesced by default (filesystem side effects).

---

## 2) Goals

- Keep a long-lived process that owns browser/session state to avoid repeated scaffolding.
- Reuse one browser + one primary OWA tab whenever possible.
- Route CLI command execution through daemon in FIFO order.
- Centralize auth/session/token/CDP management.
- Improve reliability (recovery, retries, bounded queueing) and safety (flood controls).

---

## 3) Non-goals (v1)

- No Windows support.
- No distributed/multi-host daemon clustering.
- No multi-tenant isolation inside one daemon process.
- No full workflow scheduler (cron-like behavior remains external).
- No implementation in this phase (spec only).

---

## 4) UX / CLI contract

### 4.1 Activation

- New global flag: `--daemon`.
- Without `--daemon`, existing behavior remains unchanged.
- With `--daemon`, the CLI becomes a client:
  1. Try connect to daemon socket.
  2. If absent, auto-start daemon.
  3. Submit command request.
  4. Wait for response (bounded by timeout).

### 4.2 Daemon subcommands

Add:
- `cli-365 daemon run` (internal long-running server)
- `cli-365 daemon status`
- `cli-365 daemon stop`
- `cli-365 daemon ping` (optional but useful for health checks)

### 4.3 CDP port consistency

If daemon is running and caller passes `--cdp-port` differing from daemon’s active CDP port:
- return hard error: `daemon running with cdp-port=<X>; requested=<Y>`.

### 4.4 Error behavior

- Queue full -> immediate deterministic error (`QUEUE_FULL`).
- Auth paused -> immediate deterministic error for new requests (`AUTH_PAUSED`).
- Auth recovery timeout -> fail all pending queued requests (`AUTH_TIMEOUT`).

---

## 5) High-level architecture

```text
cli-365 --daemon <command>
   │
   ├─(UDS JSON RPC request)──► cli-365 daemon
   │                           ├─ Queue manager (FIFO, bounded)
   │                           ├─ Worker (single execution lane)
   │                           ├─ Browser/session manager
   │                           ├─ Token/auth manager
   │                           └─ OpenClaw notifier
   │
   ◄─────────────(response)────┘
```

### 5.1 IPC transport

- Unix domain socket (UDS), local host only.
- Paths under runtime/state dir, e.g.:
  - socket: `$XDG_STATE_HOME/cli-365/daemon.sock`
  - lock: `$XDG_STATE_HOME/cli-365/daemon.lock`
  - pid/status: `$XDG_STATE_HOME/cli-365/daemon.json`
- permissions:
  - directory `0700`
  - socket + files `0600`

### 5.2 Single instance

- Enforce single daemon via lock file (`flock`).
- Prevent split-brain browser/session ownership.

### 5.3 Execution model

- One worker goroutine executes queued requests in FIFO.
- Browser + tab + token state owned only by daemon.
- Commands are dispatched in-process to shared service layer (not shelling out recursively).

---

## 6) Internal components

### 6.1 Queue manager

- Data structure: bounded FIFO queue.
- Configurable capacity (`max_queue_size`, default 64).
- Supports:
  - enqueue
  - dequeue
  - reject on full
  - pause/resume
  - drain-fail all pending (used on auth timeout/shutdown)

### 6.2 Browser/session manager

Responsibilities:
- Connect/start browser.
- Enforce `DISPLAY=:1` when daemon launches managed browser.
- Keep one primary OWA tab id.
- Health-check tab/browser between requests and on interval.
- Recover:
  - if tab closed -> create new tab and navigate to OWA.
  - if browser dead -> restart browser, reconnect, recreate tab.

### 6.3 Auth/token manager

Responsibilities:
- Track session validity (`IsSessionValid`-style probe).
- Parse JWT access token `exp` where possible and refresh ahead of expiry.
- Keep token cache/session headers current.
- On auth-required signals, trigger recovery state machine.

### 6.4 Auth recovery coordinator

State machine:
- `READY` -> `AUTH_RECOVERING` -> `READY` or `AUTH_FAILED`

On entering `AUTH_RECOVERING`:
1. Pause queue consumption.
2. Reject new requests (`AUTH_PAUSED`).
3. Launch secure input command.
4. Notify operator via OpenClaw with current login URL.
5. Poll for session valid until timeout (`auth_recovery_timeout`, default 5m).

Outcomes:
- Success: resume queue.
- Timeout/failure: fail all pending requests with `AUTH_TIMEOUT`, keep daemon alive for next attempts.

### 6.5 OpenClaw notifier

Built-in notifier uses **CLI invocation** (not direct HTTP in v1), e.g. `openclaw message send ...`.

Notification payload/content should include:
- service/app (`cli-365`)
- severity (`warning`/`error`)
- reason (`auth_required`, `auth_timeout`)
- URL requiring interaction
- timestamp
- queue depth snapshot

Configurable target channel (`discord` or `whatsapp`) and target id/name.

### 6.6 QoS / safety controls

- Request size limit (avoid huge payload abuse).
- Per-request timeout (default 2m).
- Panic guard around request execution.
- Structured logs (without token leakage).

---

## 7) Request/response protocol (UDS JSON)

## 7.1 Request envelope

```json
{
  "request_id": "uuid",
  "submitted_at": "RFC3339",
  "command": "mail search",
  "argv": ["mail", "search", "invoice", "--limit", "5"],
  "global": {
    "json": true,
    "readonly": false,
    "daemon": true,
    "cdp_port": 9222,
    "timeout_ms": 120000
  },
  "coalesce_key": "sha256(...)"
}
```

### 7.2 Response envelope

```json
{
  "request_id": "uuid",
  "ok": true,
  "exit_code": 0,
  "stdout": "...",
  "stderr": "...",
  "started_at": "RFC3339",
  "finished_at": "RFC3339",
  "queue_wait_ms": 41,
  "exec_ms": 390
}
```

Error responses should include stable code + human-readable message:
- `QUEUE_FULL`
- `AUTH_PAUSED`
- `AUTH_TIMEOUT`
- `CDP_PORT_MISMATCH`
- `DAEMON_UNAVAILABLE`
- `REQUEST_TIMEOUT`

---

## 8) Queue semantics and dedup/coalescing

## 8.1 FIFO baseline

- Execution order is FIFO for enqueued work.
- No priority lanes in v1.

## 8.2 Identical-command optimization (recommended)

User-proposed idea accepted with constraints:

- Apply only to **safe read-class commands**.
- Use **in-flight/queued coalescing**:
  - If identical request is already queued or running, attach requester as waiter.
  - Execute once, fan out same response to all attached waiters.
- Do **not** coalesce write commands.

### 8.3 Coalescing eligibility

Eligible (read-only):
- `mail search`, `mail view`, `mail thread get`, `mail attachments list`
- `calendar list`, `calendar get`
- `auth status`, `browser status`, `daemon status`

Not eligible (write/mutating or side-effecting):
- `mail send`, `mail reply`, draft mutations
- `mail attachments download` (filesystem output path side effects)
- `calendar create/update/delete`
- `auth login/logout`

### 8.4 Coalesce key

Hash of normalized:
- command path
- semantic args/flags
- identity-sensitive context (profile/account/tenant/cdp)
- output mode (`--json` etc.)

(Exclude request_id and timestamps.)

---

## 9) Write flood controls (recommended)

User-proposed controls are good and should be included in v1 minimal form.

### 9.1 Duplicate write suppression

For high-risk commands (`mail send`, `mail reply`, `calendar create`):
- compute fingerprint of normalized write payload.
- if same fingerprint appears within suppression window, reject with `DUPLICATE_WRITE_SUSPECTED`.
- default suppression windows:
  - mail writes: `12h`
  - calendar writes: `1h`
- allow opt-out with explicit global override flag: `--allow-duplicate-write`.

### 9.2 Per-recipient throttling

- Token bucket per recipient (email address key).
- Default example: 6/minute, burst 2.

### 9.3 Global write throttling

- Global write bucket (all write commands): default e.g. 20/minute.

These controls reduce accidental floods and script bugs.

---

## 10) Backoff, jitter, and retry policy

### 10.1 Read commands

For upstream `429` and transient `5xx`:
- exponential backoff with full jitter
- bounded retries (e.g. 3)
- cap total retry budget within request timeout

### 10.2 Write commands

- Conservative default: **no automatic replay** for non-idempotent writes.
- Return explicit error with retry guidance.
- Future optional idempotency keys can expand safe retries.

---

## 11) Token/session lifecycle details

- Track access token expiry from JWT `exp` when available.
- Refresh proactively before expiry (e.g. 5-minute lead).
- Because refresh token expiry is often not explicit/reliable in this flow, rely on:
  - proactive access-token refresh,
  - session-valid probes,
  - auth recovery path when invalid.

---

## 12) Config additions (proposed)

```yaml
daemon:
  enabled: false                 # gated behind --daemon initially
  socket_path: ""               # default: $XDG_STATE_HOME/cli-365/daemon.sock
  lock_path: ""                 # default: $XDG_STATE_HOME/cli-365/daemon.lock
  max_queue_size: 64
  default_command_timeout: "2m"
  auth_recovery_timeout: "5m"
  reject_new_while_auth_paused: true
  display: ":1"
  coalesce_identical_reads: true
  duplicate_write_window_mail: "12h"
  duplicate_write_window_calendar: "1h"
  write_rate_limit_per_minute: 20
  recipient_write_rate_limit_per_minute: 6

  notify:
    provider: "openclaw-cli"
    openclaw_cmd: "openclaw"
    channel: "discord"          # or whatsapp
    target: ""                  # channel/user target for alerts
```

Notes:
- Existing `auth.secure_input` config remains source-of-truth for secure input command.
- `--cdp-port` mismatch against active daemon should fail fast.

---

## 13) Security requirements

- Never log bearer/canary tokens.
- Redact auth headers in debug logs.
- UDS permissions locked to current user.
- Keep allowlist/readonly checks server-side (not only in client).
- Reject oversized IPC payloads.
- Validate command path against known command table.

---

## 14) Resource management requirements

- Reuse one browser and one OWA tab by default.
- Close temporary pages immediately after use.
- Clear large transient buffers after response (where practical).
- Bound in-memory queue and response buffering.
- On daemon stop: graceful drain policy + browser cleanup.

---

## 15) TDD plan (required)

## 15.1 Unit tests

1. Queue
   - FIFO ordering
   - max size enforcement
   - pause/resume semantics
   - drain-fail behavior
2. Coalescing
   - read command coalesced
   - write command never coalesced
3. Rate limits/flood controls
   - duplicate suppression window
   - per-recipient + global bucket behavior
4. Auth recovery coordinator
   - transition to paused state
   - reject new while paused
   - timeout fan-out errors
5. CDP consistency
   - mismatch rejected

## 15.2 Integration tests (daemon-only)

1. First `--daemon` call auto-starts daemon.
2. Subsequent calls connect and reuse same daemon/tab.
3. Browser crash simulated -> daemon recovers.
4. Auth-required simulated -> queue paused + notifier invoked + secure input invoked.
5. Auth timeout -> pending requests fail with stable error codes.

## 15.3 Contract tests

- For supported commands, ensure output/exit semantics match non-daemon mode (modulo latency/metadata).

---

## 16) Implementation phases (after approval)

Phase A: daemon skeleton + UDS + lifecycle/status/stop.
Phase B: queue + worker + command dispatch through shared service layer.
Phase C: browser/tab ownership + crash recovery + CDP mismatch checks.
Phase D: auth pause/recovery + secure-input + OpenClaw notify.
Phase E: coalescing for reads + flood controls + rate limiting.
Phase F: retry/backoff policy + hardening + docs.

---

## 17) Open points to confirm before coding

None currently blocking.

---

## 18) Acceptance criteria (definition of done for v1)

- `--daemon` path works end-to-end on Linux and macOS.
- First daemon call starts daemon; next calls reuse it.
- Queue is FIFO and bounded; full queue errors are deterministic.
- Auth pause rejects new work and recovers or times out with fan-out errors.
- Same `--cdp-port` requirement enforced.
- Browser/tab crash recovery demonstrated by tests.
- OpenClaw notification sent on auth-required and auth-timeout.
- No token leaks in logs.
