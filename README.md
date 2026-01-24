# Outlook Browser CLI

A command-line interface for Outlook Web App (OWA) using browser automation. This tool allows you to interact with your Outlook mail through the command line, leveraging OWA's web interface for authentication and API access.

## Features

- **Browser-based authentication**: Uses your existing OWA session, no OAuth app registration needed
- **Mail operations**: Search, read threads, create/send drafts, manage attachments
- **Persistent sessions**: Browser profile persists between runs for seamless auth
- **JSON output**: All commands support `--json` flag for scripting

## Installation

```bash
go install github.com/lc0rp/outlook-browser-cli/cmd/outlook-browser-cli@latest
```

Or build from source:

```bash
git clone https://github.com/lc0rp/cli-365.git
cd cli-365
go build -o outlook-browser-cli ./cmd/outlook-browser-cli
```

## Configuration

Configuration file: `~/.config/outlook-browser-cli/config.yaml`

```yaml
profile_dir: "~/.config/outlook-browser-cli/profile"
browser:
  headless: true
  no_sandbox: false  # Set true for containers/root users
  cdp_endpoint: ""   # Empty = auto-start managed browser
auth:
  tenant: "common"
  account_hint: ""   # Pre-fill email on login
  readonly: false
  scopes: ["mail.readwrite", "mail.send"]
security:
  allowlist: ["mail", "auth", "browser"]
  keyring: "os"      # os | encrypted-file
```

## Usage

### Browser Management

```bash
# Start the managed browser
outlook-browser-cli browser start

# Check browser status
outlook-browser-cli browser status

# Stop the browser
outlook-browser-cli browser stop
```

### Authentication

```bash
# Login (opens browser for authentication)
outlook-browser-cli auth login

# Check auth status
outlook-browser-cli auth status

# Logout (clear stored credentials)
outlook-browser-cli auth logout
```

### Mail Operations

```bash
# Search messages
outlook-browser-cli mail search "keyword"
outlook-browser-cli mail search --limit 50 "meeting"

# Get a thread/conversation
outlook-browser-cli mail thread get <conversation-id>

# Create a draft
outlook-browser-cli mail draft create --to "user@example.com" --subject "Hello" --body "Message body"

# Update a draft
outlook-browser-cli mail draft update <draft-id> --subject "New Subject" --body "Updated body"

# Delete a draft
outlook-browser-cli mail draft delete <draft-id>

# Send a draft
outlook-browser-cli mail draft send <draft-id>

# Send a message directly (creates and sends)
outlook-browser-cli mail send --to "user@example.com" --subject "Hello" --body "Message body"

# List attachments for a message
outlook-browser-cli mail attachments list <message-id>

# Download an attachment
outlook-browser-cli mail attachments download <attachment-id> [output-path]
```

### JSON Output

Add `--json` flag to any command for JSON output:

```bash
outlook-browser-cli --json mail search "invoice"
outlook-browser-cli --json auth status
```

## How It Works

1. **Browser Session**: The CLI manages a Chromium browser instance using rod (Go CDP client)
2. **OWA Authentication**: Uses the standard Microsoft login flow through the browser
3. **Token Discovery**: Extracts X-OWA-CANARY token from cookies/page state
4. **API Calls**: Makes OWA service API calls via in-page fetch() to use the authenticated session

## Architecture

```
cmd/outlook-browser-cli/
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
go build ./cmd/outlook-browser-cli

# Run with verbose output
./outlook-browser-cli --help
```

## Requirements

- Go 1.21+
- Chromium browser (auto-downloaded by rod if not present)

## Notes

- The `no_sandbox: true` option may be needed when running as root or in containers
- Browser profile is stored in `~/.config/outlook-browser-cli/profile/` for session persistence
- Tokens are cached in `~/.local/state/outlook-browser-cli/tokens.json`

## License

MIT
