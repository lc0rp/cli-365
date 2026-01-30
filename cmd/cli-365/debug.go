package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/urfave/cli/v2"

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
						Name:  "netlog-bodies",
						Usage: "Capture request/response bodies (redacted)",
						Value: false,
					},
					&cli.BoolFlag{
						Name:  "netlog-all-bodies",
						Usage: "Capture request/response bodies for all URLs (redacted)",
						Value: false,
					},
					&cli.IntFlag{
						Name:  "netlog-body-max",
						Usage: "Max bytes to keep per request/response body",
						Value: 64 * 1024,
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
					b, err := ensureBrowser(ctx, c, cfg)
					if err != nil {
						return err
					}

					client := owa.NewClient(b)
					if err := client.Connect(); err != nil {
						return err
					}

					page := client.Page()
					logger, stopNetlog, err := owa.StartNetworkLogger(page, owa.NetworkLogOptions{
						MaxEntries:    c.Int("netlog-max"),
						CaptureBodies: c.Bool("netlog-bodies"),
						MaxBodyBytes:  c.Int("netlog-body-max"),
						CaptureAll:    c.Bool("netlog-all-bodies"),
					})
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

					if stopNetlog != nil {
						stopNetlog()
						stopNetlog = nil
					}
					netlog := logger.Snapshot()
					features := owa.ExtractNetlogFeatures(netlog)
					if session := logger.SessionHeaders(); !session.IsZero() {
						tokens.Session = owa.MergeSessionHeaders(tokens.Session, session)
						owa.SetSessionHeaders(tokens.Session)
						_ = owa.SaveTokens(tokens)
					}
					if folders := logger.FolderIDs(); len(folders) > 0 {
						if tokens.Folders == nil {
							tokens.Folders = make(map[string]string)
						}
						for k, v := range folders {
							if v == "" {
								continue
							}
							tokens.Folders[k] = v
						}
						_ = owa.SaveTokens(tokens)
					}
					if tokens.Canary == "" {
						if canary := logger.Canary(); canary != "" {
							tokens.Canary = canary
							_ = owa.SaveTokens(tokens)
						} else if canary := findCanaryInNetlog(netlog); canary != "" {
							tokens.Canary = canary
							_ = owa.SaveTokens(tokens)
						}
					}

					var probe *probeFetchResult
					var probeNetlog *owa.NetworkLog
					if c.Bool("probe-fetch") {
						probe = &probeFetchResult{}
						probeLogger, stopProbe, err := owa.StartNetworkLogger(page, owa.NetworkLogOptions{
							MaxEntries:    c.Int("netlog-max"),
							CaptureBodies: c.Bool("netlog-bodies"),
							MaxBodyBytes:  c.Int("netlog-body-max"),
						})
						if err != nil {
							return err
						}
						var (
							result *owa.SearchResult
						)
						if tokens.Canary == "" && tokens.Bearer == "" {
							err = fmt.Errorf("missing canary and bearer tokens")
						} else {
							result, err = owa.SearchMessages(page, tokens, "", "", 1)
							if err != nil && strings.Contains(err.Error(), "ErrorInternalServerError") {
								if convResult, convErr := owa.SearchConversations(page, tokens, "", "", 1); convErr == nil {
									result = convResult
									err = nil
								}
							}
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
						if stopProbe != nil {
							stopProbe()
						}
						if probeLogger != nil {
							snapshot := probeLogger.Snapshot()
							probeNetlog = &snapshot
						}
					}

					payload := map[string]interface{}{
						"templates": discovery,
						"summary":   summary,
						"netlog":    netlog,
						"features":  features,
					}
					if probe != nil {
						payload["probe_fetch"] = probe
					}
					if probeNetlog != nil {
						payload["probe_netlog"] = probeNetlog
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
					if features != nil {
						fmt.Printf("features: service_actions=%d\n", len(features.ServiceActions))
					}
					if tokens != nil {
						fmt.Printf("session: session_id=%t anchor_mailbox=%t tenant_id=%t\n",
							tokens.Session.SessionID != "",
							tokens.Session.AnchorMailbox != "",
							tokens.Session.TenantID != "")
					}
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
			{
				Name:  "capture",
				Usage: "Capture network activity until Enter is pressed",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "headless",
						Usage: "Run browser headless (overrides config)",
						Value: false,
					},
					&cli.DurationFlag{
						Name:    "timeout",
						Aliases: []string{"t"},
						Value:   10 * time.Minute,
						Usage:   "Max capture duration",
					},
					&cli.StringFlag{
						Name:  "netlog",
						Usage: "Write network log JSON to file",
					},
					&cli.IntFlag{
						Name:  "netlog-max",
						Usage: "Max network log entries to keep",
						Value: 2000,
					},
					&cli.BoolFlag{
						Name:  "netlog-bodies",
						Usage: "Capture request/response bodies (redacted)",
						Value: true,
					},
					&cli.BoolFlag{
						Name:  "netlog-all-bodies",
						Usage: "Capture request/response bodies for all URLs (redacted)",
						Value: false,
					},
					&cli.IntFlag{
						Name:  "netlog-body-max",
						Usage: "Max bytes to keep per request/response body",
						Value: 64 * 1024,
					},
					&cli.BoolFlag{
						Name:  "all-pages",
						Usage: "Capture network activity for all browser pages",
						Value: false,
					},
					&cli.BoolFlag{
						Name:  "all-targets",
						Usage: "Capture network activity for all browser targets (includes service/background workers)",
						Value: false,
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
					b, err := ensureBrowser(ctx, c, cfg)
					if err != nil {
						return err
					}

					client := owa.NewClient(b)
					if err := client.Connect(); err != nil {
						return err
					}

					page := client.Page()
					logOpts := owa.NetworkLogOptions{
						MaxEntries:    c.Int("netlog-max"),
						CaptureBodies: c.Bool("netlog-bodies"),
						MaxBodyBytes:  c.Int("netlog-body-max"),
						CaptureAll:    c.Bool("netlog-all-bodies"),
					}
					type loggerHandle struct {
						logger *owa.NetworkLogger
						stop   func()
					}
					var (
						mu      sync.Mutex
						handles = map[string]loggerHandle{}
					)
					startLogger := func(p *rod.Page, keyOverride string) error {
						if p == nil {
							return nil
						}
						key := keyOverride
						if key == "" {
							info, err := p.Info()
							if err != nil || info == nil {
								return nil
							}
							key = string(info.TargetID)
							if key == "" {
								key = info.URL
							}
						}
						mu.Lock()
						if _, ok := handles[key]; ok {
							mu.Unlock()
							return nil
						}
						mu.Unlock()
						logger, stopNetlog, err := owa.StartNetworkLogger(p, logOpts)
						if err != nil {
							return err
						}
						mu.Lock()
						handles[key] = loggerHandle{logger: logger, stop: stopNetlog}
						mu.Unlock()
						return nil
					}
					if err := startLogger(page, ""); err != nil {
						return err
					}
					allPages := c.Bool("all-pages")
					allTargets := c.Bool("all-targets")
					var (
						wg     sync.WaitGroup
						stopCh chan struct{}
					)
					if allTargets {
						stopCh = make(chan struct{})
						wg.Add(1)
						go func() {
							defer wg.Done()
							ticker := time.NewTicker(500 * time.Millisecond)
							defer ticker.Stop()
							for {
								select {
								case <-stopCh:
									return
								case <-ticker.C:
									targets, err := proto.TargetGetTargets{}.Call(b)
									if err != nil || targets == nil {
										continue
									}
									for _, target := range targets.TargetInfos {
										if target.Type == proto.TargetTargetInfoTypeBrowser {
											continue
										}
										p, err := b.PageFromTarget(target.TargetID)
										if err != nil {
											continue
										}
										_ = startLogger(p, string(target.TargetID))
									}
								}
							}
						}()
					} else if allPages {
						stopCh = make(chan struct{})
						wg.Add(1)
						go func() {
							defer wg.Done()
							ticker := time.NewTicker(500 * time.Millisecond)
							defer ticker.Stop()
							for {
								select {
								case <-stopCh:
									return
								case <-ticker.C:
									pages, err := b.Pages()
									if err != nil {
										continue
									}
									for _, p := range pages {
										_ = startLogger(p, "")
									}
								}
							}
						}()
					}

					if err := owa.NavigateToOWA(page); err != nil {
						return err
					}
					if !owa.IsLoggedIn(page) {
						fmt.Println("Please complete login in the browser window...")
						if err := owa.WaitForLogin(page, c.Duration("timeout")); err != nil {
							return err
						}
					}

					fmt.Println("Capture running. Perform the action in the browser, then press Enter to stop.")
					_ = waitForEnter(c.Duration("timeout"))

					if stopCh != nil {
						close(stopCh)
						wg.Wait()
					}

					mu.Lock()
					handleList := make([]loggerHandle, 0, len(handles))
					for _, h := range handles {
						handleList = append(handleList, h)
					}
					mu.Unlock()

					var (
						netlog         owa.NetworkLog
						seenCanary     bool
						seenSession    bool
						initializedLog bool
					)
					for _, h := range handleList {
						if h.stop != nil {
							h.stop()
						}
						if h.logger == nil {
							continue
						}
						snapshot := h.logger.Snapshot()
						if !initializedLog {
							netlog.StartedAt = snapshot.StartedAt
							netlog.EndedAt = snapshot.EndedAt
							netlog.Redacted = snapshot.Redacted
							netlog.BodyCapture = snapshot.BodyCapture
							netlog.MaxBodyBytes = snapshot.MaxBodyBytes
							initializedLog = true
						} else {
							if snapshot.StartedAt.Before(netlog.StartedAt) {
								netlog.StartedAt = snapshot.StartedAt
							}
							if snapshot.EndedAt.After(netlog.EndedAt) {
								netlog.EndedAt = snapshot.EndedAt
							}
						}
						netlog.Dropped += snapshot.Dropped
						netlog.Entries = append(netlog.Entries, snapshot.Entries...)
						if h.logger.Canary() != "" {
							seenCanary = true
						}
						if session := h.logger.SessionHeaders(); !session.IsZero() {
							seenSession = true
						}
					}
					if seenCanary {
						fmt.Printf("canary observed: %t\n", seenCanary)
					}
					if seenSession {
						fmt.Printf("session observed: %t\n", seenSession)
					}

					netlogPath := c.String("netlog")
					if netlogPath != "" {
						if err := writeJSONFile(netlogPath, netlog); err != nil {
							return err
						}
						fmt.Printf("wrote %s\n", netlogPath)
					}
					fmt.Printf("network: entries=%d dropped=%d\n", len(netlog.Entries), netlog.Dropped)
					return nil
				},
			},
		},
	}
}

func waitForEnter(timeout time.Duration) error {
	ch := make(chan struct{})
	go func() {
		reader := bufio.NewReader(os.Stdin)
		if _, err := reader.ReadString('\n'); err == nil {
			close(ch)
		}
	}()
	if timeout <= 0 {
		<-ch
		return nil
	}
	select {
	case <-ch:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("capture timeout")
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
