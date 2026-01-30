# cli-365

A command-line interface for Outlook Web App (OWA) using browser automation. This tool allows you to interact with your Outlook mail through the command line, leveraging OWA's web interface for authentication and API access.

## Features

- **Browser-based authentication**: Uses your existing OWA session, no OAuth app registration needed
- **Mail operations**: Search, read threads, create/send drafts, manage attachments
- **Persistent sessions**: Browser profile persists between runs for seamless auth
- **JSON output**: All commands support `--json` flag for scripting

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
  allowlist: ["mail", "auth", "browser"]  # Commands allowed (empty = all)
  keyring: "os"      # Token storage: os | encrypted-file | plain
```

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

# Get a thread/conversation
cli-365 mail thread get <conversation-id>

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
- For `mail reply`, flags can appear after the message ID; you can also pass the body as the second positional argument.

### JSON Output

Add `--json` flag to any command for JSON output:

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
cli-365 --cdp-port 9222 --ensure-cdp mail search "invoice"
```

### Ensure CDP

If your config points at a CDP endpoint that might be stale, use `--ensure-cdp` to start a managed browser and wait for login.

```bash
cli-365 --ensure-cdp --ensure-cdp-timeout 5m mail search "invoice"
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

## How It Works

1. **Browser Session**: The CLI manages a Chromium browser instance using rod (Go CDP client)
2. **OWA Authentication**: Uses the standard Microsoft login flow through the browser
3. **Token Discovery**: Extracts X-OWA-CANARY token from cookies/page state
4. **API Calls**: Makes OWA service API calls via in-page fetch() to use the authenticated session

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
