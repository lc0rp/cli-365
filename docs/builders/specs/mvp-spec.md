---
type: Reference
primary_audience: Builders
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../status/mvp-status.md
read_when: Need historical MVP planning context (legacy snapshot).
---

# cli-365 — Spec (MVP Legacy Snapshot)

Status: Legacy planning snapshot kept for historical context only.  
Superseded by:

- `docs/builders/status/mvp-status.md`
- `docs/builders/specs/daemon-v1.md`

## MVP (Mail)

- Search threads/messages
- View thread details
- Create drafts
- Send emails
- List/download attachments

## Requirements

- Token auto-refresh; auth once, reuse indefinitely
- Least-privilege flags: `--readonly`, scope flags (ex: `--mail-scope`, `--drive-scope`)
- Command allowlist for sandbox/agent runs
- Secure creds: OS keyring or encrypted on-disk keyring (configurable)

## Architecture

- Go CLI + rod (CDP browser manager)
- Internal browser keepalive

  - `ensureBrowser` or explicit `browser start|status|stop`
  - Persistent profile directory

## Discovery / Fetch Layer (outlook-gcal-mirror inspired)

- Load OWA app; capture templates/config
- Extract canary + bearer from in-page state
- Use in-page `fetch` to call OWA endpoints
- Normalize to CLI models (threads/messages/attachments)

## Config Layout

Path: `$XDG_CONFIG_HOME/cli-365/config.yaml`

```yaml
profile_dir: "~/.config/cli-365/profile"
browser:
  headless: true
  no_sandbox: false  # set true for container/host without sandbox support
  cdp_endpoint: ""   # empty => auto-start managed browser
auth:
  tenant: "common"
  account_hint: ""   # email
  readonly: false
  scopes: ["mail.readwrite", "mail.send"]
security:
  allowlist: ["mail", "auth", "browser"]
  keyring: "os"      # os | encrypted-file
```

## Commands (proposed)

- `auth login|logout|status`
- `browser start|status|stop`
- `mail search`
- `mail thread get`
- `mail draft create|update|delete|send`
- `mail send`
- `mail attachments list|download`

## Open Questions

- OWA endpoints + templates for search/thread/draft/send?
- Best source for canary/bearer in OWA bootstrapped state?
- Attachments: stream via in-page fetch vs direct URL?
- Scope mapping for readonly vs send vs attachments?
- Allowlist format: glob vs exact command paths?

## TODO (Next Steps)

- Browser manager + `ensureBrowser` + persistent profile
- OWA discovery: load mailbox, parse config + tokens
- Mail search + thread fetch (core models)
- Draft create/send + attachment list/download
- Auth + keyring storage + allowlist enforcement
