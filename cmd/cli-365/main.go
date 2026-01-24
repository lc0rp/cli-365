package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/lc0rp/cli-365/internal/browser"
	"github.com/lc0rp/cli-365/internal/config"
	"github.com/lc0rp/cli-365/internal/owa"
)

func main() {
	app := &cli.App{
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
		},
		Commands: []*cli.Command{
			authCommand(),
			browserCommand(),
			mailCommand(),
			debugCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(c *cli.Context) (config.Config, error) {
	path := c.String("config")
	return config.Load(path)
}

// getOWAClient gets a connected OWA client with valid tokens.
func getOWAClient(c *cli.Context) (*owa.Client, error) {
	ctx := context.Background()
	cfg, err := loadConfig(c)
	if err != nil {
		return nil, err
	}

	b, err := browser.EnsureBrowser(ctx, cfg)
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
					b, err := browser.EnsureBrowser(ctx, cfg)
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
				Action: func(c *cli.Context) error {
					cfg, err := loadConfig(c)
					if err != nil {
						return err
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
				},
				Action: func(c *cli.Context) error {
					client, err := getOWAClient(c)
					if err != nil {
						return err
					}

					query := c.Args().First()
					limit := c.Int("limit")
					folder := c.String("folder")

					result, err := owa.SearchMessages(client.Page(), client.Tokens().Canary, query, folder, limit)
					if err != nil {
						return err
					}

					if c.Bool("json") {
						return outputJSON(result)
					}

					if len(result.Messages) == 0 {
						fmt.Println("No messages found")
						return nil
					}

					for _, msg := range result.Messages {
						from := ""
						if msg.From != nil {
							from = msg.From.Address
							if msg.From.Name != "" {
								from = msg.From.Name
							}
						}
						read := " "
						if !msg.IsRead {
							read = "*"
						}
						fmt.Printf("%s [%s] %s - %s\n", read, msg.ID[:8], from, msg.Subject)
					}
					return nil
				},
			},
			{
				Name:      "view",
				Usage:     "View a single message",
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
					msg, err := owa.GetMessage(client.Page(), client.Tokens().Canary, messageID)
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
						Action: func(c *cli.Context) error {
							if c.NArg() < 1 {
								return cli.Exit("conversation ID required", 1)
							}

							client, err := getOWAClient(c)
							if err != nil {
								return err
							}

							convID := c.Args().First()
							conv, err := owa.GetConversation(client.Page(), client.Tokens().Canary, convID, "")
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

							msg, err := owa.CreateDraft(client.Page(), client.Tokens().Canary, draft)
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

							msg, err := owa.UpdateDraft(client.Page(), client.Tokens().Canary, draftID, draft)
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
							if err := owa.DeleteDraft(client.Page(), client.Tokens().Canary, draftID); err != nil {
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
							if err := owa.SendDraft(client.Page(), client.Tokens().Canary, draftID); err != nil {
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

					if err := owa.SendMessage(client.Page(), client.Tokens().Canary, draft); err != nil {
						return err
					}

					fmt.Println("Message sent")
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
							attachments, err := owa.ListAttachments(client.Page(), client.Tokens().Canary, messageID)
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
							content, name, err := owa.GetAttachment(client.Page(), client.Tokens().Canary, attachmentID)
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
