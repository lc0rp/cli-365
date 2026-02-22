# cli-365

A command-line interface for Outlook Web App (OWA) using browser automation. This tool allows you to interact with your Outlook mail through the command line, leveraging OWA's web interface for authentication and API access.

## Features

- **Browser-based authentication**: Uses your existing OWA session, no OAuth app registration needed
- **Mail operations**: Search, view, reply, drafts, attachments (thread view is experimental)
- **Calendar operations**: List/get/create/update/delete events, add directory calendars
- **Persistent sessions**: Browser profile persists between runs for seamless auth
- **JSON output**: All commands support `--json` (global) for scripting

## Installation

```bash
go install github.com/lc0rp/cli-365/cmd/cli-365@latest
```

Or build from source:

```bash
git clone https://github.com/lc0rp/cli-365.git
cd cli-365
go build -o cli-365 ./cmd/cli-365
```

Recommended local build target (devbox/mac): install into `~/.local/bin` so daemon and shell
can resolve the binary from `PATH`.

```bash
go build -o ~/.local/bin/cli-365 ./cmd/cli-365
```

## Configuration

Configuration file: `~/.config/cli-365/config.yaml`

```yaml
profile_dir: "~/.config/cli-365/profile"
browser:
  headless: true
  no_sandbox: false  # Set true for containers/root users
  cdp_endpoint: ""   # Empty = auto-start managed browser
  cdp_port: 0        # Optional fixed CDP port for managed browser
auth:
  tenant: "common"
  account_hint: ""   # Pre-fill email on login
  readonly: false    # Global readonly mode
  scopes: ["mail.readwrite", "mail.send"]
security:
  allowlist: ["mail", "calendar", "auth", "browser"]  # Commands allowed (empty = all)
  keyring: "os"      # Token storage: os | encrypted-file | plain
daemon:
  enabled: true      # Default: route commands via daemon unless --daemon=false is set
```

## Quickstart

Note: on devbox, prepend `DISPLAY=:1 XAUTHORITY=$HOME/.Xauthority` to commands to enable GUI.

```bash
# Start/login once (opens browser for authentication)
cli-365 --cdp-port 9222 auth login

# Search your inbox
cli-365 --cdp-port 9222 mail search "invoice" --limit 5

# View a message from the last search (index is 1-based)
cli-365 --cdp-port 9222 mail view --index 1
```

## User Docs

- Start here (beginner + expert routes): `docs/users/index.md`
- First run tutorial: `docs/users/tutorial-first-success.md`
- Mail workflows: `docs/users/howto-mail-workflows.md`
- Calendar workflows: `docs/users/howto-calendar-workflows.md`
- Troubleshooting: `docs/users/howto-troubleshoot.md`
- Scripting/reference patterns: `docs/users/reference-cli-patterns.md`

### Security Features

**Readonly Mode**: Restricts operations to read-only commands (search, view, list).
```bash
# Via flag
cli-365 --readonly mail search "test"  # Allowed
cli-365 --readonly mail send ...       # Blocked

# Or set in config
auth:
  readonly: true
```

**Command Allowlist**: Restrict which commands can run (useful for sandboxed/agent environments).
```yaml
security:
  allowlist: ["mail search", "mail view", "auth"]  # Only these commands work
```

**Keyring Storage**: Securely store tokens using OS keyring or encrypted file.
```yaml
security:
  keyring: "os"             # macOS Keychain, Linux secret-tool
  keyring: "encrypted-file" # AES-256-GCM encrypted file
  keyring: "plain"          # Plain JSON file (not recommended)
```

## Usage

### Global Flags

These flags must appear **before** the subcommand:

```bash
cli-365 --json auth status
cli-365 --cdp-port 9222 mail search "invoice"
```

Available:
- `--json`: JSON output for any command
- `--cdp-port`: override configured CDP port for this run
- `--readonly`: block write commands (send/draft/delete)
- `--version`: print CLI version

### Versioning (SemVer)

`cli-365` uses semantic versioning for build output. Runtime version is sourced from
`cmd/cli-365/version.go` and updated by semantic-release.

Manual build stamping (optional):

```bash
go build -ldflags "-X main.version=1.2.3" -o ~/.local/bin/cli-365 ./cmd/cli-365
```

Check installed version:

```bash
cli-365 --version
```

### Release Automation

Releases are commit-driven via semantic-release (Conventional Commits):
- `fix:` / `perf:` => patch
- `feat:` => minor
- `BREAKING CHANGE` / `!` => major

Release tooling requires Node.js `>=22`.

Default release happens in CI on push to `main` (`.github/workflows/release.yml`).

Local preview:

```bash
scripts/release.sh --dry-run
```

Local execution (rare, requires `GITHUB_TOKEN`):

```bash
scripts/release.sh
```

Commit messages are enforced by commitlint (`.github/workflows/commitlint.yml`).
See `docs/RELEASING.md` for full workflow details.

### Browser Management

```bash
# Start the managed browser
cli-365 browser start

# Start with a fixed CDP port
cli-365 browser start --cdp-port 9222

# Check browser status
cli-365 browser status

# Stop the browser
cli-365 browser stop
```

### Authentication

```bash
# Login (opens browser for authentication)
cli-365 auth login

# Check auth status
cli-365 auth status

# Logout (clear stored credentials)
cli-365 auth logout
```

### Mail Operations

```bash
# Search messages
cli-365 mail search "keyword"
cli-365 mail search --limit 50 "meeting"
cli-365 mail search --provider searchservice "meeting"
cli-365 mail search --from "alice@example.com" --since 2026-01-01
cli-365 mail search --subject "quarterly report" --before 2026-01-20
cli-365 mail search --cc "team@example.com" --has-attachments --unread
cli-365 mail search --query 'from:"alice@example.com" subject:"Q4" hasattachment:true'

# View a single message
cli-365 mail view <message-id>
cli-365 mail view --index 3
cli-365 mail view #3

# Get a thread/conversation (experimental)
cli-365 mail thread get <conversation-id>
cli-365 mail thread get --index 3
cli-365 mail thread get --message-id <message-id>

# Create a draft
cli-365 mail draft create --to "user@example.com" --subject "Hello" --body "Message body"

# Update a draft
cli-365 mail draft update <draft-id> --subject "New Subject" --body "Updated body"

# Delete a draft
cli-365 mail draft delete <draft-id>

# Send a draft
cli-365 mail draft send <draft-id>

# Send a message directly (creates and sends)
cli-365 mail send --to "user@example.com" --subject "Hello" --body "Message body"

# Reply to a message
cli-365 mail reply <message-id> --body "Thanks!"
cli-365 mail reply <message-id> --body "All here" --all
cli-365 mail reply <message-id> "Thanks!"  # body as positional
cli-365 mail reply --index 3 --body "Thanks!"  # use cached search index
cli-365 mail reply #3 "Thanks!"               # shorthand

# List attachments for a message
cli-365 mail attachments list <message-id>

# Download an attachment
cli-365 mail attachments download <attachment-id> [output-path]
```

Notes:
- Search uses the Outlook searchservice endpoint by default and falls back to OWA service.svc.
- Use `--provider owa|searchservice|auto` to force a backend.
- `--from`, `--to`, `--cc`, `--bcc`, `--subject`, `--since`, `--after`, `--before`, `--has-attachments`, `--unread`, `--is-read` compile into the query string used by Outlook search.
- Dates accept `YYYY-MM-DD` or RFC3339. RFC3339 values preserve time granularity in the query.
- `--query` passes a raw query string and skips auto-escaping/assembly.
- `--limit` can appear after the query; other search flags should appear before the query (or use `--query`).
- Last search results are cached at `~/.local/state/cli-365/last_search.json` and power `--index` / `#N` shortcuts for `mail view`, `mail reply`, and `mail thread get`.
- For `mail reply`, flags can appear after the message ID; you can also pass the body as the second positional argument.

### Calendar Operations

```bash
# List events (defaults to now → 7 days)
cli-365 calendar list

# List events in a range
cli-365 calendar list --start 2026-01-01 --end 2026-01-07

# List events from a specific calendar/folder
cli-365 calendar list --folder "<folder-id>" --start 2026-01-01 --end 2026-01-07

# Get a single event
cli-365 calendar get <event-id>

# Create an event
cli-365 calendar create --subject "Standup" --start 2026-01-02T09:00:00-05:00 --end 2026-01-02T09:15:00-05:00 --location "Zoom" --attendee "alice@example.com"

# Update an event
cli-365 calendar update <event-id> --subject "Updated" --start 2026-01-02T10:00:00-05:00 --end 2026-01-02T10:30:00-05:00

# Delete an event
cli-365 calendar delete <event-id>

# List calendars (alias: folders)
cli-365 calendar calendars

# Add a colleague/resource calendar from directory (email)
cli-365 calendar add-from-directory --email "alice@example.com"

# Add from directory by name (uses directory lookup)
cli-365 calendar add-from-directory --name "Alice Adams"
```

Notes:
- `calendar list` defaults to now → 7 days if `--start` is omitted.
- `calendar list` filters by `--folder` (default folder is `calendar`).
- Use `--attendee` and `--optional-attendee` multiple times to add attendees.
- `calendar calendars` (alias: `calendar folders`) lists each calendar with `folder_id` and, when known, `calendar_id` and email metadata.
- `calendar calendars` also includes directory calendars tracked in local registry (`~/.local/state/cli-365/added_calendars.json`) even when they are not returned by live `GetCalendarFolders`.
- `calendar add-from-directory` (alias: `calendar add-directory`) accepts `--email`, `--name`, or a positional `[email-or-name]`.
- `calendar add-from-directory` checks existing calendars first (exact name/email match) to avoid duplicate adds.
- `calendar add-from-directory` may return `Calendar already added` with the existing `folder_id` / `calendar_id` instead of adding again.
- Name lookup uses directory search and can fail on ambiguous matches (use `--email` or `--allow-ambiguous`).
- To target by name/email, resolve to `folder_id` first:

```bash
FOLDER_ID=$(cli-365 --json calendar calendars | jq -r '.[] | select(.name=="Alice Adams") | .folder_id')
cli-365 calendar list --folder "$FOLDER_ID" --start 2026-01-01 --end 2026-01-07
```

### JSON Output

Add `--json` (global) to any command for JSON output:

```bash
cli-365 --json mail search "invoice"
cli-365 --json auth status
```

#### Search JSON shape

`mail search` returns:

```json
{
  "TotalCount": 42,
  "Messages": [
    {
      "ItemId": "...",
      "ConversationId": "...",
      "Subject": "...",
      "BodyPreview": "...",
      "DateTimeReceived": "...",
      "From": { "EmailAddress": "..." }
    }
  ],
  "Conversations": [
    {
      "ConversationId": "...",
      "ConversationTopic": "...",
      "LastDeliveryTime": "...",
      "UnreadCount": 0,
      "MessageCount": 1,
      "Preview": "..."
    }
  ]
}
```

Notes:
- When using searchservice, `Messages` are representative items for each conversation hit (use `mail view` or `mail thread get` for full details).

### CDP Port Override

Override the configured CDP port for a single run:

```bash
cli-365 --cdp-port 9222 mail search "invoice"
```

### Daemon CDP/Auth Preflight

By default (`daemon.enabled: true`), `mail`/`calendar` commands run through daemon preflight that ensures CDP/browser availability and performs auth recovery before command execution.

Exception: `calendar add-from-directory` / `calendar add-directory` runs direct (non-daemon) by default unless `--daemon` is explicitly set. This command depends on in-page OWA calendar modules that may not be loaded in daemon-maintained tabs.

```bash
cli-365 calendar list --limit 10
# Disable daemon path for one run
cli-365 --daemon=false calendar list --limit 10
# Force daemon path for add-from-directory (not recommended unless you need it)
cli-365 --daemon calendar add-from-directory --email "alice@example.com"
```

### Debug / Discovery

```bash
# Launch browser, wait for login, discover template-like values, and probe fetch
cli-365 debug discover --out ./owa-templates.json

# Include network-level request log
cli-365 debug discover --netlog ./owa-netlog.json

# Capture network activity until Enter is pressed
cli-365 debug capture --netlog ./owa-capture.json

# Capture all pages or all CDP targets (including service/background workers)
cli-365 debug capture --all-pages --netlog ./owa-capture.json
cli-365 debug capture --all-targets --netlog ./owa-capture.json
```

## Troubleshooting

- **Browser window not showing**: ensure a GUI is available and export `DISPLAY`/`XAUTHORITY` (e.g. `DISPLAY=:1 XAUTHORITY=$HOME/.Xauthority`).
- **401 Unauthorized**: run `cli-365 auth login` to refresh tokens.
- **500 OwaSerializationException**: capture a netlog (`cli-365 debug capture --netlog ...`) and re-run after login.
- **Stale CDP**: use daemon mode (`--daemon`) and/or set `--cdp-port` to a known port. For `calendar add-from-directory`, direct mode is default; use `--daemon` only if you explicitly need daemon routing.

## Known Issues

- `mail thread get` is experimental and may return `message not found`.
- `mail draft send` can fail with 404 (SendItem endpoint variance).
- `mail draft delete` can fail with 500.
- `mail attachments download` can fail with 500 on some accounts.

## How It Works

1. **Browser Session**: The CLI manages a Chromium browser instance using rod (Go CDP client)
2. **OWA Authentication**: Uses the standard Microsoft login flow through the browser
3. **Token Discovery**: Extracts X-OWA-CANARY and bearer tokens from cookies/page state
4. **API Calls**: Uses OWA service API calls; conversation fallback may query Graph/Substrate (experimental)

## Architecture

```
cmd/cli-365/
├── main.go              # CLI entrypoint and commands

internal/
├── browser/
│   └── manager.go       # Browser lifecycle (start/stop/connect)
├── config/
│   └── config.go        # Configuration loading
├── owa/
│   ├── client.go        # OWA client wrapper
│   ├── discovery.go     # Token extraction
│   ├── fetch.go         # In-page API calls
│   ├── calendar.go      # Calendar operations
│   ├── mail.go          # Mail operations
│   └── models.go        # Data models
└── paths/
    └── paths.go         # XDG path helpers
```

## Development

```bash
# Run tests
go test ./...

# Build
go build ./cmd/cli-365

# Run with verbose output
./cli-365 --help
```

## Requirements

- Go 1.21+
- Chromium browser (auto-downloaded by rod if not present)

## Notes

- The `no_sandbox: true` option may be needed when running as root or in containers
- Browser profile is stored in `~/.config/cli-365/profile/` for session persistence
- Tokens are cached in `~/.local/state/cli-365/tokens.json`

## License

MIT
