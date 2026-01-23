# outlook-browser-cli

CLI for Outlook Web (OWA) via a managed browser session (CDP).

## Quickstart

```bash
# build
cd /path/to/projects/outlook-browser-cli

go build ./cmd/outlook-browser-cli

# run
./outlook-browser-cli browser start
./outlook-browser-cli browser status
```

Config lives at `~/.config/outlook-browser-cli/config.yaml` (XDG-aware).

## TODO

- Implement OWA discovery + in-page fetch
- Mail search/thread/draft/send/attachments
- Auth + keyring storage + allowlist enforcement
