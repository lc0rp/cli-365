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
	"github.com/lc0rp/cli-365/internal/owa"
)

type probeFetchResult struct {
	OK                bool   `json:"ok"`
	MessageCount      int    `json:"message_count,omitempty"`
	ConversationCount int    `json:"conversation_count,omitempty"`
	TotalCount        int    `json:"total_count,omitempty"`
	Error             string `json:"error,omitempty"`
}

func debugCommand() *cli.Command {
	return &cli.Command{
		Name:  "debug",
		Usage: "E2E helpers for browser discovery",
		Subcommands: []*cli.Command{
			{
				Name:  "discover",
				Usage: "Launch browser, login, discover templates, and prime fetch",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "headless",
						Usage: "Run browser headless (overrides config)",
						Value: false,
					},
					&cli.DurationFlag{
						Name:    "timeout",
						Aliases: []string{"t"},
						Value:   5 * time.Minute,
						Usage:   "Login timeout",
					},
					&cli.StringFlag{
						Name:  "out",
						Usage: "Write discovery + summary JSON to file",
					},
					&cli.StringFlag{
						Name:  "netlog",
						Usage: "Write network log JSON to file",
					},
					&cli.IntFlag{
						Name:  "netlog-max",
						Usage: "Max network log entries to keep",
						Value: 500,
					},
					&cli.BoolFlag{
						Name:  "probe-fetch",
						Usage: "Run a minimal FindItem to validate fetch",
						Value: true,
					},
				},
				Action: func(c *cli.Context) error {
					cfg, err := loadConfig(c)
					if err != nil {
						return err
					}
					if c.IsSet("headless") {
						cfg.Browser.Headless = c.Bool("headless")
					} else {
						cfg.Browser.Headless = false
					}

					ctx := context.Background()
					b, err := browser.EnsureBrowser(ctx, cfg)
					if err != nil {
						return err
					}

					client := owa.NewClient(b)
					if err := client.Connect(); err != nil {
						return err
					}

					page := client.Page()
					logger, stopNetlog, err := owa.StartNetworkLogger(page, c.Int("netlog-max"))
					if err != nil {
						return err
					}
					defer func() {
						if stopNetlog != nil {
							stopNetlog()
						}
					}()

					if err := owa.NavigateToOWA(page); err != nil {
						return err
					}

					if !owa.IsLoggedIn(page) {
						if !c.Bool("json") {
							fmt.Println("Please complete login in the browser window...")
						}
						if err := owa.WaitForLogin(page, c.Duration("timeout")); err != nil {
							return err
						}
					}

					tokens, err := owa.LoadOrDiscoverTokens(page)
					if err != nil {
						return err
					}
					client.SetTokens(tokens)

					discovery, err := owa.DiscoverTemplates(page)
					if err != nil {
						return err
					}
					summary := owa.AnalyzeTemplates(discovery)

					var probe *probeFetchResult
					if c.Bool("probe-fetch") {
						probe = &probeFetchResult{}
						var (
							result *owa.SearchResult
							err    error
						)
						if tokens.Canary != "" {
							result, err = owa.SearchMessages(page, tokens.Canary, "", "", 1)
						} else if tokens.Bearer != "" {
							result, err = owa.SearchMessagesWithBearer(page, tokens.Bearer, "", "", 1)
						} else {
							err = fmt.Errorf("missing canary and bearer tokens")
						}
						if err != nil {
							probe.OK = false
							probe.Error = err.Error()
						} else {
							probe.OK = true
							probe.MessageCount = len(result.Messages)
							probe.ConversationCount = len(result.Conversations)
							probe.TotalCount = result.TotalCount
						}
					}
					if stopNetlog != nil {
						stopNetlog()
						stopNetlog = nil
					}
					netlog := logger.Snapshot()
					if tokens.Canary == "" {
						if canary := findCanaryInNetlog(netlog); canary != "" {
							tokens.Canary = canary
							_ = owa.SaveTokens(tokens)
						}
					}

					payload := map[string]interface{}{
						"templates": discovery,
						"summary":   summary,
						"netlog":    netlog,
					}
					if probe != nil {
						payload["probe_fetch"] = probe
					}

					outPath := c.String("out")
					if outPath != "" {
						if err := writeJSONFile(outPath, payload); err != nil {
							return err
						}
					}
					netlogPath := c.String("netlog")
					if netlogPath != "" {
						if err := writeJSONFile(netlogPath, netlog); err != nil {
							return err
						}
					}

					if c.Bool("json") {
						return outputJSON(payload)
					}

					if outPath != "" {
						fmt.Printf("wrote %s\n", outPath)
					}
					if netlogPath != "" {
						fmt.Printf("wrote %s\n", netlogPath)
					}
					fmt.Printf("templates: window=%d state=%d\n", len(discovery.WindowTemplates), len(discovery.StateTemplates))
					fmt.Printf("network: entries=%d dropped=%d\n", len(netlog.Entries), netlog.Dropped)
					if probe != nil {
						if probe.OK {
							fmt.Printf("fetch probe: ok (messages=%d total=%d)\n", probe.MessageCount, probe.TotalCount)
						} else {
							fmt.Printf("fetch probe: failed (%s)\n", probe.Error)
						}
					}

					return nil
				},
			},
		},
	}
}

func writeJSONFile(path string, payload interface{}) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func findCanaryInNetlog(netlog owa.NetworkLog) string {
	for _, entry := range netlog.Entries {
		if val := headerValue(entry.RequestHeaders, "x-owa-canary"); val != "" {
			return val
		}
		if val := headerValue(entry.ResponseHeaders, "x-owa-canary"); val != "" {
			return val
		}
	}
	return ""
}

func headerValue(headers map[string]string, key string) string {
	if len(headers) == 0 {
		return ""
	}
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}
