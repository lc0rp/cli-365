---
type: Concept
primary_audience: Users
owner: cli-365 maintainers
last_verified: 2026-02-22
next_review_by: 2026-03-22
source_of_truth: ../../README.md
read_when: >-
  You want the mental model for how cli-365 authenticates, executes commands,
  and uses daemon/browser state.
---

# About cli-365

## Overview

`cli-365` is a terminal client that automates Outlook Web App (OWA) using a
managed browser session.

It is not a direct Microsoft Graph OAuth app integration. It uses your
existing browser-authenticated OWA context.

## Core model

1. Browser session: managed Chromium instance with persistent profile.
2. Auth/session extraction: OWA tokens/cookies discovered from the browser context.
3. Command execution: `mail`/`calendar` commands call OWA-backed APIs via page
   context.
4. Optional daemon: command routing through local daemon for warm browser/auth
   and request controls.

## Why this design

- Fast local automation without Azure app registration.
- Works with existing enterprise login and conditional access flow.
- Supports both human CLI usage and scriptable `--json` workflows.

## Tradeoffs

- Depends on browser/session health.
- Some flows are page-module dependent (`calendar add-from-directory` defaults
  to direct mode).
- Behavior can vary with OWA frontend changes.

## Related pages

- Start guide: `docs/users/tutorial-first-success.md`
- Task guides: `docs/users/howto-mail-workflows.md`,
  `docs/users/howto-calendar-workflows.md`
- Troubleshooting: `docs/users/howto-troubleshoot.md`
