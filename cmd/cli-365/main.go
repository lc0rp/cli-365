package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/urfave/cli/v2"

	"github.com/lc0rp/cli-365/internal/browser"
	"github.com/lc0rp/cli-365/internal/config"
	"github.com/lc0rp/cli-365/internal/keyring"
	"github.com/lc0rp/cli-365/internal/owa"
	"github.com/lc0rp/cli-365/internal/paths"
	"github.com/lc0rp/cli-365/internal/security"
)

type cliAppOptions struct {
	DisableDaemonForwarding bool
}

func main() {
	code := runCLI(context.Background(), os.Args, cliAppOptions{})
	if code != 0 {
		os.Exit(code)
	}
}

func newCLIApp(opts cliAppOptions) *cli.App {
	return &cli.App{
		Name:  "cli-365",
		Usage: "CLI for Outlook OWA via a managed browser",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to config file (default: XDG config dir)",
			},
			&cli.BoolFlag{
				Name:    "json",
				Aliases: []string{"j"},
				Usage:   "Output in JSON format",
			},
			&cli.BoolFlag{
				Name:  "readonly",
				Usage: "Restrict to read-only operations (no send/draft/delete)",
			},
			&cli.BoolFlag{
				Name:  "ensure-cdp",
				Usage: "Start managed browser if CDP is unavailable and wait for login",
			},
			&cli.DurationFlag{
				Name:  "ensure-cdp-timeout",
				Usage: "How long to wait for CDP/login when --ensure-cdp is set",
				Value: 5 * time.Minute,
			},
			&cli.IntFlag{
				Name:  "cdp-port",
				Usage: "Override browser.cdp_port for this run",
			},
			&cli.BoolFlag{
				Name:  "daemon",
				Usage: "Route command execution through local cli-365 daemon",
			},
		},
		ExitErrHandler: func(_ *cli.Context, _ error) {},
		Before:         newAppBefore(opts),
		Commands: []*cli.Command{
			authCommand(),
			browserCommand(),
			daemonCommand(),
			mailCommand(),
			calendarCommand(),
			debugCommand(),
		},
	}
}

func runCLI(ctx context.Context, args []string, opts cliAppOptions) int {
	return runCLIApp(ctx, newCLIApp(opts), args)
}

func runCLIApp(ctx context.Context, app *cli.App, args []string) int {
	var err error
	if ctx == nil {
		err = app.Run(args)
	} else {
		err = app.RunContext(ctx, args)
	}
	if err == nil {
		return 0
	}

	if ec, ok := err.(cli.ExitCoder); ok {
		if ec.ExitCode() == 0 {
			return 0
		}
		msg := strings.TrimSpace(err.Error())
		if msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
		return ec.ExitCode()
	}

	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 1
}

func newAppBefore(opts cliAppOptions) func(*cli.Context) error {
	return func(c *cli.Context) error {
		commandPath := buildCommandPath(c)
		if !opts.DisableDaemonForwarding && c.Bool("daemon") && !strings.HasPrefix(commandPath, "daemon") && commandPath != "" {
			if err := enforceSecurityPolicy(c); err != nil {
				return err
			}
			return runViaDaemon(c)
		}
		return enforceSecurityPolicy(c)
	}
}

// enforceSecurityPolicy checks allowlist and readonly restrictions before command execution.
func enforceSecurityPolicy(c *cli.Context) error {
	// Skip check for help commands
	if c.NArg() == 0 || c.Args().First() == "help" {
		return nil
	}

	cfg, err := loadConfig(c)
	if err != nil {
		return err
	}

	// Build command path from args
	commandPath := buildCommandPath(c)

	// Create policy from config and flags
	policy := security.Policy{
		Readonly:  cfg.Auth.Readonly || c.Bool("readonly"),
		Allowlist: cfg.Security.Allowlist,
	}

	// Check security policy
	return policy.Check(commandPath)
}

// buildCommandPath extracts the command path from CLI context.
func buildCommandPath(c *cli.Context) string {
	args := c.Args().Slice()
	var parts []string

	// Extract command parts (stop at first flag or end)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			break
		}
		parts = append(parts, arg)
	}

	return strings.Join(parts, " ")
}

func parseSearchProvider(raw string) (owa.SearchProvider, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return owa.SearchProviderAuto, nil
	case "owa":
		return owa.SearchProviderOWA, nil
	case "searchservice":
		return owa.SearchProviderSearchService, nil
	default:
		return "", fmt.Errorf("invalid provider %q (expected auto, owa, searchservice)", raw)
	}
}

func buildSearchQuery(base string, from string, to string, cc string, bcc string, subject string, hasAttachments bool, unread bool, isRead bool, since string, before string) (string, error) {
	parts := make([]string, 0, 10)
	if base = strings.TrimSpace(base); base != "" {
		parts = append(parts, base)
	}
	if from = strings.TrimSpace(from); from != "" {
		parts = append(parts, fmt.Sprintf("from:\"%s\"", escapeQueryValue(from)))
	}
	if to = strings.TrimSpace(to); to != "" {
		parts = append(parts, fmt.Sprintf("to:\"%s\"", escapeQueryValue(to)))
	}
	if cc = strings.TrimSpace(cc); cc != "" {
		parts = append(parts, fmt.Sprintf("cc:\"%s\"", escapeQueryValue(cc)))
	}
	if bcc = strings.TrimSpace(bcc); bcc != "" {
		parts = append(parts, fmt.Sprintf("bcc:\"%s\"", escapeQueryValue(bcc)))
	}
	if subject = strings.TrimSpace(subject); subject != "" {
		parts = append(parts, fmt.Sprintf("subject:\"%s\"", escapeQueryValue(subject)))
	}
	if hasAttachments {
		parts = append(parts, "hasattachment:true")
	}
	if unread && isRead {
		return "", fmt.Errorf("cannot set both --unread and --is-read")
	}
	if unread {
		parts = append(parts, "isread:false")
	}
	if isRead {
		parts = append(parts, "isread:true")
	}
	if since = strings.TrimSpace(since); since != "" {
		date, err := normalizeDateTime(since)
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("received>=%s", date))
	}
	if before = strings.TrimSpace(before); before != "" {
		date, err := normalizeDateTime(before)
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("received<=%s", date))
	}
	return strings.Join(parts, " "), nil
}

func escapeQueryValue(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

func normalizeDateTime(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.Format("2006-01-02"), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		if strings.Contains(raw, ".") {
			return t.Format(time.RFC3339Nano), nil
		}
		return t.Format(time.RFC3339), nil
	}
	return "", fmt.Errorf("invalid date %q (use YYYY-MM-DD or RFC3339)", raw)
}

func loadConfig(c *cli.Context) (config.Config, error) {
	path := c.String("config")
	return config.Load(path)
}

func ensureBrowser(ctx context.Context, c *cli.Context, cfg config.Config) (*rod.Browser, error) {
	if c.IsSet("cdp-port") {
		cfg.Browser.CDPPort = c.Int("cdp-port")
		cfg.Browser.CDPEndpoint = ""
	}
	if !c.Bool("ensure-cdp") {
		return browser.EnsureBrowser(ctx, cfg)
	}

	if cfg.Browser.CDPPort > 0 {
		if endpoint, err := browser.ResolveWSEndpoint(cfg.Browser.CDPPort); err == nil {
			if b, err := browser.ConnectEndpoint(endpoint); err == nil {
				_ = browser.SaveRuntime(&browser.RuntimeInfo{
					WSEndpoint: endpoint,
					PID:        0,
					Managed:    false,
					StartedAt:  time.Now(),
				})
				if err := ensureLoggedIn(c, b); err != nil {
					return nil, err
				}
				return b, nil
			}
		}
		fmt.Printf("[ensure-cdp] CDP unavailable on port %d; starting managed browser...\n", cfg.Browser.CDPPort)
		cfgNoCDP := cfg
		cfgNoCDP.Browser.CDPEndpoint = ""
		b, err := browser.EnsureBrowser(ctx, cfgNoCDP)
		if err != nil {
			return nil, err
		}
		if err := ensureLoggedIn(c, b); err != nil {
			return nil, err
		}
		return b, nil
	}

	if cfg.Browser.CDPEndpoint != "" {
		b, err := browser.ConnectEndpoint(cfg.Browser.CDPEndpoint)
		if err == nil {
			_ = browser.SaveRuntime(&browser.RuntimeInfo{
				WSEndpoint: cfg.Browser.CDPEndpoint,
				PID:        0,
				Managed:    false,
				StartedAt:  time.Now(),
			})
			if err := ensureLoggedIn(c, b); err != nil {
				return nil, err
			}
			return b, nil
		}
		fmt.Printf("[ensure-cdp] CDP unavailable at %s; starting managed browser...\n", cfg.Browser.CDPEndpoint)
	}

	cfgNoCDP := cfg
	cfgNoCDP.Browser.CDPEndpoint = ""
	b, err := browser.EnsureBrowser(ctx, cfgNoCDP)
	if err != nil {
		return nil, err
	}
	if err := ensureLoggedIn(c, b); err != nil {
		return nil, err
	}
	return b, nil
}

func ensureLoggedIn(c *cli.Context, b *rod.Browser) error {
	client := owa.NewClient(b)
	if err := client.Connect(); err != nil {
		return err
	}
	page := client.Page()
	if owa.IsLoggedIn(page) {
		return nil
	}
	if info, err := page.Info(); err == nil {
		if info.URL == "" || info.URL == "about:blank" {
			if err := owa.NavigateToOWA(page); err != nil {
				return err
			}
		}
	} else {
		if err := owa.NavigateToOWA(page); err != nil {
			return err
		}
	}
	fmt.Println("[ensure-cdp] Please complete login in the browser window...")
	timeout := c.Duration("ensure-cdp-timeout")
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return owa.WaitForLoggedIn(page, timeout)
}

// getTokenStorage returns the appropriate token storage based on config.
func getTokenStorage(cfg config.Config) (*keyring.TokenStorage, error) {
	return keyring.NewTokenStorage(cfg.Security.Keyring)
}

// getOWAClient gets a connected OWA client with valid tokens.
func getOWAClient(c *cli.Context) (*owa.Client, error) {
	ctx := context.Background()
	cfg, err := loadConfig(c)
	if err != nil {
		return nil, err
	}

	b, err := ensureBrowser(ctx, c, cfg)
	if err != nil {
		return nil, err
	}

	client := owa.NewClient(b)
	if err := client.Connect(); err != nil {
		return nil, err
	}

	tokens, err := owa.LoadOrDiscoverTokens(client.Page())
	if err != nil {
		return nil, err
	}
	client.SetTokens(tokens)
	owa.SetSessionHeaders(tokens.Session)

	return client, nil
}

func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func authCommand() *cli.Command {
	return &cli.Command{
		Name:  "auth",
		Usage: "Authenticate and manage stored credentials",
		Subcommands: []*cli.Command{
			{
				Name:  "login",
				Usage: "Login to Outlook Web App",
				Flags: []cli.Flag{
					&cli.DurationFlag{
						Name:    "timeout",
						Aliases: []string{"t"},
						Value:   5 * time.Minute,
						Usage:   "Login timeout",
					},
				},
				Action: func(c *cli.Context) error {
					ctx := context.Background()
					cfg, err := loadConfig(c)
					if err != nil {
						return err
					}

					// Ensure browser is running
					b, err := ensureBrowser(ctx, c, cfg)
					if err != nil {
						return fmt.Errorf("failed to start browser: %w", err)
					}

					client := owa.NewClient(b)
					if err := client.Connect(); err != nil {
						return fmt.Errorf("failed to connect: %w", err)
					}

					page := client.Page()

					// Navigate to OWA
					fmt.Println("Opening Outlook Web App...")
					if err := owa.NavigateToOWA(page); err != nil {
						return fmt.Errorf("failed to navigate: %w", err)
					}

					// Check if already logged in
					if owa.IsLoggedIn(page) {
						fmt.Println("Already logged in!")
					} else {
						fmt.Println("Please complete login in the browser window...")
						fmt.Println("Waiting for authentication...")

						timeout := c.Duration("timeout")
						if err := owa.WaitForLogin(page, timeout); err != nil {
							return err
						}
						fmt.Println("Login successful!")
					}

					// Discover and save tokens
					tokens, err := owa.DiscoverTokens(page)
					if err != nil {
						return fmt.Errorf("failed to extract tokens: %w", err)
					}

					if err := owa.SaveTokens(tokens); err != nil {
						return fmt.Errorf("failed to save tokens: %w", err)
					}

					fmt.Printf("Authenticated as: %s\n", tokens.UserEmail)
					return nil
				},
			},
			{
				Name:  "logout",
				Usage: "Clear stored credentials",
				Action: func(c *cli.Context) error {
					if err := owa.ClearTokens(); err != nil {
						return fmt.Errorf("failed to clear tokens: %w", err)
					}
					fmt.Println("Logged out (credentials cleared)")
					return nil
				},
			},
			{
				Name:  "status",
				Usage: "Show current authentication status",
				Action: func(c *cli.Context) error {
					tokens, err := owa.LoadTokens()
					if err != nil {
						if os.IsNotExist(err) {
							fmt.Println("Not authenticated")
							return nil
						}
						return err
					}

					if c.Bool("json") {
						return outputJSON(map[string]interface{}{
							"authenticated": true,
							"user_email":    tokens.UserEmail,
							"extracted_at":  tokens.ExtractedAt,
							"has_canary":    tokens.Canary != "",
							"has_bearer":    tokens.Bearer != "",
						})
					}

					fmt.Println("Authenticated: yes")
					if tokens.UserEmail != "" {
						fmt.Printf("User: %s\n", tokens.UserEmail)
					}
					fmt.Printf("Token extracted at: %s\n", tokens.ExtractedAt.Format(time.RFC3339))
					return nil
				},
			},
		},
	}
}

func browserCommand() *cli.Command {
	return &cli.Command{
		Name:  "browser",
		Usage: "Manage the Outlook browser session",
		Subcommands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start or attach to a managed browser",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "cdp-port",
						Aliases: []string{"p"},
						Usage:   "Fixed CDP port for the managed browser",
					},
				},
				Action: func(c *cli.Context) error {
					cfg, err := loadConfig(c)
					if err != nil {
						return err
					}
					if c.IsSet("cdp-port") {
						cfg.Browser.CDPPort = c.Int("cdp-port")
						cfg.Browser.CDPEndpoint = ""
					}
					ctx := context.Background()
					rt, err := browser.Start(ctx, cfg)
					if err != nil {
						return err
					}
					if c.Bool("json") {
						return outputJSON(rt)
					}
					fmt.Printf("browser started (managed=%t)\nws endpoint: %s\n", rt.Managed, rt.WSEndpoint)
					return nil
				},
			},
			{
				Name:  "status",
				Usage: "Show current browser status",
				Action: func(c *cli.Context) error {
					rt, err := browser.Status()
					if err != nil {
						if os.IsNotExist(err) {
							if c.Bool("json") {
								return outputJSON(map[string]interface{}{"running": false})
							}
							fmt.Println("stopped")
							return nil
						}
						return err
					}
					if c.Bool("json") {
						return outputJSON(map[string]interface{}{
							"running":     true,
							"managed":     rt.Managed,
							"pid":         rt.PID,
							"ws_endpoint": rt.WSEndpoint,
							"started_at":  rt.StartedAt,
						})
					}
					fmt.Printf("managed=%t pid=%d\nws endpoint: %s\n", rt.Managed, rt.PID, rt.WSEndpoint)
					return nil
				},
			},
			{
				Name:  "stop",
				Usage: "Stop the managed browser",
				Action: func(c *cli.Context) error {
					if err := browser.Stop(); err != nil {
						if os.IsNotExist(err) {
							fmt.Println("stopped")
							return nil
						}
						return err
					}
					fmt.Println("stopped")
					return nil
				},
			},
		},
	}
}

func mailCommand() *cli.Command {
	return &cli.Command{
		Name:  "mail",
		Usage: "Mail operations (MVP)",
		Subcommands: []*cli.Command{
			{
				Name:      "search",
				Usage:     "Search messages",
				ArgsUsage: "[query]",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "limit",
						Aliases: []string{"n"},
						Value:   20,
						Usage:   "Maximum results to return",
					},
					&cli.StringFlag{
						Name:  "folder",
						Usage: "Folder ID to search in (default: inbox)",
					},
					&cli.StringFlag{
						Name:  "provider",
						Usage: "Search backend: auto | owa | searchservice",
						Value: "auto",
					},
					&cli.StringFlag{
						Name:  "from",
						Usage: "Filter by sender (query syntax)",
					},
					&cli.StringFlag{
						Name:  "to",
						Usage: "Filter by recipient (query syntax)",
					},
					&cli.StringFlag{
						Name:  "subject",
						Usage: "Filter by subject (query syntax)",
					},
					&cli.StringFlag{
						Name:  "cc",
						Usage: "Filter by CC recipient (query syntax)",
					},
					&cli.StringFlag{
						Name:  "bcc",
						Usage: "Filter by BCC recipient (query syntax)",
					},
					&cli.BoolFlag{
						Name:  "has-attachments",
						Usage: "Filter to messages with attachments",
					},
					&cli.BoolFlag{
						Name:  "unread",
						Usage: "Filter to unread messages",
					},
					&cli.BoolFlag{
						Name:  "is-read",
						Usage: "Filter to read messages",
					},
					&cli.StringFlag{
						Name:  "query",
						Usage: "Raw query string (disables auto-escaping/assembly)",
					},
					&cli.StringFlag{
						Name:  "since",
						Usage: "Filter received since date/time (YYYY-MM-DD or RFC3339)",
					},
					&cli.StringFlag{
						Name:  "after",
						Usage: "Alias for --since (date/time)",
					},
					&cli.StringFlag{
						Name:  "before",
						Usage: "Filter received before date/time (YYYY-MM-DD or RFC3339)",
					},
				},
				Action: func(c *cli.Context) error {
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}

					rawQuery := strings.TrimSpace(c.String("query"))
					if rawQuery == "" {
						rawQuery = c.Args().First()
					}
					query := rawQuery
					since := c.String("since")
					after := c.String("after")
					if after != "" {
						if since != "" && since != after {
							return fmt.Errorf("--since and --after must match if both are set")
						}
						since = after
					}
					if c.String("query") == "" {
						query, err = buildSearchQuery(
							rawQuery,
							c.String("from"),
							c.String("to"),
							c.String("cc"),
							c.String("bcc"),
							c.String("subject"),
							c.Bool("has-attachments"),
							c.Bool("unread"),
							c.Bool("is-read"),
							since,
							c.String("before"),
						)
						if err != nil {
							return err
						}
					}
					limit := c.Int("limit")
					if !c.IsSet("limit") {
						if parsed, ok := parseTrailingIntFlag(c.Args().Slice(), []string{"--limit", "-n"}); ok {
							limit = parsed
						}
					}
					folder := c.String("folder")
					provider, err := parseSearchProvider(c.String("provider"))
					if err != nil {
						return err
					}

					result, err := owa.SearchMessagesWithProvider(client.Page(), client.Tokens(), query, folder, limit, provider)
					if err != nil {
						if strings.Contains(err.Error(), "ErrorInternalServerError") {
							if convResult, convErr := owa.SearchConversations(client.Page(), client.Tokens(), query, folder, limit); convErr == nil {
								result = convResult
								err = nil
							}
						}
						if err != nil {
							return err
						}
					}

					if c.Bool("json") {
						return outputJSON(result)
					}

					if len(result.Messages) == 0 && len(result.Conversations) == 0 {
						fmt.Println("No messages found")
						return nil
					}

					_ = saveLastSearch(query, result)

					for i, msg := range result.Messages {
						from := ""
						if msg.From != nil {
							from = msg.From.Address
							if msg.From.Name != "" {
								from = msg.From.Name
							}
						}
						subject := msg.Subject
						if subject == "" {
							subject = "(no subject)"
						}
						fmt.Printf("%d %s %s - %s\n", i+1, msg.ID, from, subject)
					}
					if len(result.Messages) == 0 && len(result.Conversations) > 0 {
						for _, conv := range result.Conversations {
							topic := conv.Topic
							if topic == "" {
								topic = "(no subject)"
							}
							fmt.Printf("  [%s] %s (%d messages)\n", conv.ID[:8], topic, conv.MessageCount)
						}
					}
					return nil
				},
			},
			{
				Name:      "view",
				Usage:     "View a single message",
				ArgsUsage: "<message-id>",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "index", Aliases: []string{"i"}, Usage: "View cached search result index"},
				},
				Action: func(c *cli.Context) error {
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}

					args := c.Args().Slice()
					messageID := ""
					index := c.Int("index")
					if index == 0 && len(args) > 0 && strings.HasPrefix(args[0], "#") {
						if parsed, err := parseIndexArg(args[0]); err == nil {
							index = parsed
							args = args[1:]
						}
					}
					if index > 0 {
						cached, err := resolveCachedMessage(index)
						if err != nil {
							return err
						}
						messageID = cached.ID
					} else if len(args) > 0 {
						messageID = args[0]
					}
					if strings.TrimSpace(messageID) == "" {
						return cli.Exit("message ID required", 1)
					}
					msg, err := owa.GetMessage(client.Page(), client.Tokens(), messageID)
					if err != nil {
						return err
					}

					if c.Bool("json") {
						return outputJSON(msg)
					}

					from := ""
					if msg.From != nil {
						from = msg.From.Address
						if msg.From.Name != "" {
							from = fmt.Sprintf("%s <%s>", msg.From.Name, msg.From.Address)
						}
					}

					fmt.Printf("From: %s\n", from)
					fmt.Printf("Subject: %s\n", msg.Subject)
					fmt.Printf("Date: %s\n", msg.DateTimeSent)

					if len(msg.ToRecipients) > 0 {
						fmt.Print("To: ")
						for i, r := range msg.ToRecipients {
							if i > 0 {
								fmt.Print(", ")
							}
							if r.Name != "" {
								fmt.Printf("%s <%s>", r.Name, r.Address)
							} else {
								fmt.Print(r.Address)
							}
						}
						fmt.Println()
					}

					if msg.HasAttachments {
						fmt.Printf("Attachments: %d\n", len(msg.Attachments))
					}

					fmt.Println()
					if msg.Body != nil {
						fmt.Println(msg.Body.Value)
					} else if msg.BodyPreview != "" {
						fmt.Println(msg.BodyPreview)
					}

					return nil
				},
			},
			{
				Name:  "thread",
				Usage: "Thread operations",
				Subcommands: []*cli.Command{
					{
						Name:      "get",
						Usage:     "Get thread/conversation details",
						ArgsUsage: "<conversation-id>",
						Flags: []cli.Flag{
							&cli.IntFlag{Name: "index", Aliases: []string{"i"}, Usage: "Use cached search result index"},
							&cli.StringFlag{Name: "message-id", Usage: "Resolve conversation from message ID"},
						},
						Action: func(c *cli.Context) error {
							client, err := getOWAClient(c)
							if err != nil {
								return err
							}

							args := c.Args().Slice()
							index := c.Int("index")
							messageID := strings.TrimSpace(c.String("message-id"))
							convID := ""
							folderID := ""
							if index == 0 && len(args) > 0 && strings.HasPrefix(args[0], "#") {
								if parsed, err := parseIndexArg(args[0]); err == nil {
									index = parsed
									args = args[1:]
								}
							}
							if index > 0 {
								cached, err := resolveCachedMessage(index)
								if err != nil {
									return err
								}
								messageID = cached.ID
								if cached.ConversationID != "" {
									convID = cached.ConversationID
								}
								if cached.ParentFolderID != "" {
									folderID = cached.ParentFolderID
								}
							}
							if messageID != "" {
								if convID == "" {
									msg, err := owa.GetMessage(client.Page(), client.Tokens(), messageID)
									if err != nil {
										return err
									}
									convID = msg.ConversationID
									folderID = msg.ParentFolderId
								}
							} else if len(args) > 0 {
								convID = args[0]
							}
							if strings.TrimSpace(convID) == "" {
								return cli.Exit("conversation ID required", 1)
							}

							conv, err := owa.GetConversation(client.Page(), client.Tokens(), convID, folderID)
							if err != nil {
								return err
							}

							if c.Bool("json") {
								return outputJSON(conv)
							}

							fmt.Printf("Conversation: %s\n", conv.Topic)
							fmt.Printf("Messages: %d\n\n", len(conv.Messages))
							for i, msg := range conv.Messages {
								from := ""
								if msg.From != nil {
									from = msg.From.Address
									if msg.From.Name != "" {
										from = msg.From.Name
									}
								}
								fmt.Printf("--- Message %d ---\n", i+1)
								fmt.Printf("From: %s\n", from)
								fmt.Printf("Date: %s\n", msg.DateTimeSent)
								fmt.Printf("Subject: %s\n", msg.Subject)
								if msg.Body != nil {
									fmt.Printf("\n%s\n", msg.Body.Value)
								}
								fmt.Println()
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "draft",
				Usage: "Draft operations",
				Subcommands: []*cli.Command{
					{
						Name:  "create",
						Usage: "Create a new draft",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "to", Required: true, Usage: "Recipient email"},
							&cli.StringFlag{Name: "subject", Aliases: []string{"s"}, Required: true, Usage: "Email subject"},
							&cli.StringFlag{Name: "body", Aliases: []string{"b"}, Usage: "Email body"},
							&cli.StringFlag{Name: "cc", Usage: "CC recipients (comma-separated)"},
							&cli.StringFlag{Name: "body-type", Value: "Text", Usage: "Body type (Text or HTML)"},
						},
						Action: func(c *cli.Context) error {
							client, err := getOWAClient(c)
							if err != nil {
								return err
							}

							draft := &owa.Draft{
								Subject: c.String("subject"),
								ToRecipients: []owa.EmailAddress{
									{Address: c.String("to")},
								},
							}

							if body := c.String("body"); body != "" {
								draft.Body = &owa.MessageBody{
									BodyType: c.String("body-type"),
									Value:    body,
								}
							}

							if cc := c.String("cc"); cc != "" {
								for _, addr := range strings.Split(cc, ",") {
									draft.CcRecipients = append(draft.CcRecipients, owa.EmailAddress{
										Address: strings.TrimSpace(addr),
									})
								}
							}

							msg, err := owa.CreateDraft(client.Page(), client.Tokens(), draft)
							if err != nil {
								return err
							}

							if c.Bool("json") {
								return outputJSON(msg)
							}

							fmt.Printf("Draft created: %s\n", msg.ID)
							return nil
						},
					},
					{
						Name:      "update",
						Usage:     "Update an existing draft",
						ArgsUsage: "<draft-id>",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "to", Usage: "New recipient email"},
							&cli.StringFlag{Name: "subject", Aliases: []string{"s"}, Usage: "New subject"},
							&cli.StringFlag{Name: "body", Aliases: []string{"b"}, Usage: "New body"},
							&cli.StringFlag{Name: "body-type", Value: "Text", Usage: "Body type (Text or HTML)"},
						},
						Action: func(c *cli.Context) error {
							if c.NArg() < 1 {
								return cli.Exit("draft ID required", 1)
							}

							client, err := getOWAClient(c)
							if err != nil {
								return err
							}

							draftID := c.Args().First()
							draft := &owa.Draft{}

							if to := c.String("to"); to != "" {
								draft.ToRecipients = []owa.EmailAddress{{Address: to}}
							}
							if subject := c.String("subject"); subject != "" {
								draft.Subject = subject
							}
							if body := c.String("body"); body != "" {
								draft.Body = &owa.MessageBody{
									BodyType: c.String("body-type"),
									Value:    body,
								}
							}

							msg, err := owa.UpdateDraft(client.Page(), client.Tokens(), draftID, draft)
							if err != nil {
								return err
							}

							if c.Bool("json") {
								return outputJSON(msg)
							}

							fmt.Println("Draft updated")
							return nil
						},
					},
					{
						Name:      "delete",
						Usage:     "Delete a draft",
						ArgsUsage: "<draft-id>",
						Action: func(c *cli.Context) error {
							if c.NArg() < 1 {
								return cli.Exit("draft ID required", 1)
							}

							client, err := getOWAClient(c)
							if err != nil {
								return err
							}

							draftID := c.Args().First()
							if err := owa.DeleteDraft(client.Page(), client.Tokens(), draftID); err != nil {
								return err
							}

							fmt.Println("Draft deleted")
							return nil
						},
					},
					{
						Name:      "send",
						Usage:     "Send an existing draft",
						ArgsUsage: "<draft-id>",
						Action: func(c *cli.Context) error {
							if c.NArg() < 1 {
								return cli.Exit("draft ID required", 1)
							}

							client, err := getOWAClient(c)
							if err != nil {
								return err
							}

							draftID := c.Args().First()
							if err := owa.SendDraft(client.Page(), client.Tokens(), draftID); err != nil {
								return err
							}

							fmt.Println("Draft sent")
							return nil
						},
					},
				},
			},
			{
				Name:  "send",
				Usage: "Send a message directly",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "to", Required: true, Usage: "Recipient email"},
					&cli.StringFlag{Name: "subject", Aliases: []string{"s"}, Required: true, Usage: "Email subject"},
					&cli.StringFlag{Name: "body", Aliases: []string{"b"}, Usage: "Email body"},
					&cli.StringFlag{Name: "cc", Usage: "CC recipients (comma-separated)"},
					&cli.StringFlag{Name: "body-type", Value: "Text", Usage: "Body type (Text or HTML)"},
				},
				Action: func(c *cli.Context) error {
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}

					draft := &owa.Draft{
						Subject: c.String("subject"),
						ToRecipients: []owa.EmailAddress{
							{Address: c.String("to")},
						},
					}

					if body := c.String("body"); body != "" {
						draft.Body = &owa.MessageBody{
							BodyType: c.String("body-type"),
							Value:    body,
						}
					}

					if cc := c.String("cc"); cc != "" {
						for _, addr := range strings.Split(cc, ",") {
							draft.CcRecipients = append(draft.CcRecipients, owa.EmailAddress{
								Address: strings.TrimSpace(addr),
							})
						}
					}

					if err := owa.SendMessage(client.Page(), client.Tokens(), draft); err != nil {
						return err
					}

					fmt.Println("Message sent")
					return nil
				},
			},
			{
				Name:      "reply",
				Usage:     "Reply to a message",
				ArgsUsage: "<message-id>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "all", Usage: "Reply all recipients"},
					&cli.IntFlag{Name: "index", Aliases: []string{"i"}, Usage: "Reply to cached search result index"},
					&cli.StringFlag{Name: "body", Aliases: []string{"b"}, Usage: "Reply body"},
					&cli.StringFlag{Name: "body-type", Value: "Text", Usage: "Body type (Text or HTML)"},
				},
				Action: func(c *cli.Context) error {
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}

					args := c.Args().Slice()
					messageID := ""
					index := c.Int("index")
					if index == 0 && len(args) > 0 && strings.HasPrefix(args[0], "#") {
						if parsed, err := parseIndexArg(args[0]); err == nil {
							index = parsed
							args = args[1:]
						}
					}
					if index > 0 {
						cached, err := resolveCachedMessage(index)
						if err != nil {
							return err
						}
						messageID = cached.ID
					} else if len(args) > 0 {
						messageID = args[0]
					}
					if strings.TrimSpace(messageID) == "" {
						return cli.Exit("message ID required", 1)
					}
					replyAll := c.Bool("all")
					bodyType := c.String("body-type")
					bodyValue := c.String("body")
					if !c.IsSet("all") || !c.IsSet("body") || !c.IsSet("body-type") {
						for i := 1; i < len(args); i++ {
							switch args[i] {
							case "--all":
								replyAll = true
							case "--body", "-b":
								if bodyValue == "" && i+1 < len(args) {
									bodyValue = args[i+1]
									i++
								}
							case "--body-type":
								if bodyType == "" && i+1 < len(args) {
									bodyType = args[i+1]
									i++
								}
							default:
								if bodyValue == "" && !strings.HasPrefix(args[i], "-") {
									bodyValue = args[i]
								}
							}
						}
					}

					if strings.TrimSpace(bodyValue) == "" {
						return cli.Exit("reply body required (use --body or provide as second argument)", 1)
					}
					body := &owa.MessageBody{
						BodyType: bodyType,
						Value:    bodyValue,
					}

					if err := owa.ReplyToMessage(client.Page(), client.Tokens(), messageID, body, replyAll); err != nil {
						return err
					}

					if c.Bool("json") {
						return outputJSON(map[string]string{"status": "sent"})
					}

					fmt.Println("Reply sent")
					return nil
				},
			},
			{
				Name:  "attachments",
				Usage: "Attachment operations",
				Subcommands: []*cli.Command{
					{
						Name:      "list",
						Usage:     "List attachments for a message",
						ArgsUsage: "<message-id>",
						Action: func(c *cli.Context) error {
							if c.NArg() < 1 {
								return cli.Exit("message ID required", 1)
							}

							client, err := getOWAClient(c)
							if err != nil {
								return err
							}

							messageID := c.Args().First()
							attachments, err := owa.ListAttachments(client.Page(), client.Tokens(), messageID)
							if err != nil {
								return err
							}

							if c.Bool("json") {
								return outputJSON(attachments)
							}

							if len(attachments) == 0 {
								fmt.Println("No attachments")
								return nil
							}

							for _, att := range attachments {
								fmt.Printf("[%s] %s (%s, %d bytes)\n", att.ID[:8], att.Name, att.ContentType, att.Size)
							}
							return nil
						},
					},
					{
						Name:      "download",
						Usage:     "Download an attachment",
						ArgsUsage: "<attachment-id> [output-path]",
						Action: func(c *cli.Context) error {
							if c.NArg() < 1 {
								return cli.Exit("attachment ID required", 1)
							}

							client, err := getOWAClient(c)
							if err != nil {
								return err
							}

							attachmentID := c.Args().Get(0)
							content, name, err := owa.GetAttachment(client.Page(), client.Tokens(), attachmentID)
							if err != nil {
								return err
							}

							outputPath := c.Args().Get(1)
							if outputPath == "" {
								outputPath = name
							}

							if err := os.WriteFile(outputPath, content, 0o644); err != nil {
								return fmt.Errorf("failed to write file: %w", err)
							}

							fmt.Printf("Downloaded: %s (%d bytes)\n", outputPath, len(content))
							return nil
						},
					},
				},
			},
		},
	}
}

type cachedSearch struct {
	Query    string          `json:"query"`
	SavedAt  time.Time       `json:"saved_at"`
	Messages []cachedMessage `json:"messages"`
}

type cachedMessage struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id,omitempty"`
	ParentFolderID string `json:"parent_folder_id,omitempty"`
	From           string `json:"from,omitempty"`
	Subject        string `json:"subject,omitempty"`
}

func saveLastSearch(query string, result *owa.SearchResult) error {
	if result == nil {
		return nil
	}
	cache := cachedSearch{
		Query:   query,
		SavedAt: time.Now(),
	}
	for _, msg := range result.Messages {
		if msg.ID == "" {
			continue
		}
		from := ""
		if msg.From != nil {
			from = msg.From.Address
			if msg.From.Name != "" {
				from = msg.From.Name
			}
		}
		cache.Messages = append(cache.Messages, cachedMessage{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			ParentFolderID: msg.ParentFolderId,
			From:           from,
			Subject:        msg.Subject,
		})
	}
	path := lastSearchPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func resolveCachedMessage(index int) (cachedMessage, error) {
	if index <= 0 {
		return cachedMessage{}, fmt.Errorf("index must be >= 1")
	}
	path := lastSearchPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return cachedMessage{}, fmt.Errorf("load cached search: %w", err)
	}
	var cache cachedSearch
	if err := json.Unmarshal(data, &cache); err != nil {
		return cachedMessage{}, fmt.Errorf("parse cached search: %w", err)
	}
	if len(cache.Messages) == 0 {
		return cachedMessage{}, fmt.Errorf("cached search has no messages")
	}
	if index > len(cache.Messages) {
		return cachedMessage{}, fmt.Errorf("index out of range: %d (max %d)", index, len(cache.Messages))
	}
	return cache.Messages[index-1], nil
}

func resolveCachedMessageID(index int) (string, error) {
	if index <= 0 {
		return "", fmt.Errorf("index must be >= 1")
	}
	msg, err := resolveCachedMessage(index)
	if err != nil {
		return "", err
	}
	if msg.ID == "" {
		return "", fmt.Errorf("cached message missing ID")
	}
	return msg.ID, nil
}

func parseIndexArg(raw string) (int, error) {
	if !strings.HasPrefix(raw, "#") {
		return 0, fmt.Errorf("index must start with #")
	}
	value := strings.TrimPrefix(raw, "#")
	if value == "" {
		return 0, fmt.Errorf("index missing")
	}
	index, err := strconv.Atoi(value)
	if err != nil || index <= 0 {
		return 0, fmt.Errorf("invalid index: %s", raw)
	}
	return index, nil
}

func parseTrailingIntFlag(args []string, names []string) (int, bool) {
	if len(args) == 0 || len(names) == 0 {
		return 0, false
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		for _, name := range names {
			if arg == name {
				if i+1 < len(args) {
					if value, err := strconv.Atoi(args[i+1]); err == nil {
						return value, true
					}
				}
				continue
			}
			prefix := name + "="
			if strings.HasPrefix(arg, prefix) {
				value := strings.TrimPrefix(arg, prefix)
				if parsed, err := strconv.Atoi(value); err == nil {
					return parsed, true
				}
				continue
			}
			if strings.HasPrefix(name, "-") && !strings.HasPrefix(name, "--") && strings.HasPrefix(arg, name) && len(arg) > len(name) {
				remainder := strings.TrimPrefix(arg, name)
				if parsed, err := strconv.Atoi(remainder); err == nil {
					return parsed, true
				}
			}
		}
	}
	return 0, false
}

func lastSearchPath() string {
	return filepath.Join(paths.StateDir(), "cli-365", "last_search.json")
}
