package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/lc0rp/cli-365/internal/daemon"
)

func daemonCommand() *cli.Command {
	return &cli.Command{
		Name:  "daemon",
		Usage: "Manage cli-365 daemon process",
		Subcommands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Run daemon server in foreground",
				Action: func(c *cli.Context) error {
					if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
						return cli.Exit(daemon.ErrUnsupportedPlatform.Error(), 1)
					}

					cfg, err := loadConfig(c)
					if err != nil {
						return err
					}
					if c.IsSet("cdp-port") {
						cfg.Browser.CDPPort = c.Int("cdp-port")
					}
					applyDaemonRuntimeEnv(cfg.Daemon.Display)
					opts := daemon.ResolveOptions(cfg)
					server := daemon.NewServer(opts, daemonExecFunc(opts.MaxResponseBytes))
					server.SetLogWriter(os.Stderr)

					ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
					defer stop()

					err = server.Run(ctx)
					if errors.Is(err, daemon.ErrAlreadyRunning) {
						return cli.Exit("daemon already running", 1)
					}
					return err
				},
			},
			{
				Name:  "status",
				Usage: "Show daemon status",
				Action: func(c *cli.Context) error {
					cfg, err := loadConfig(c)
					if err != nil {
						return err
					}
					opts := daemon.ResolveOptions(cfg)

					status, err := daemon.StatusFromOptions(opts, 500*time.Millisecond)
					if err != nil {
						return err
					}

					if c.Bool("json") {
						return outputJSON(status)
					}

					if status.Running {
						fmt.Printf("running pid=%d\nsocket: %s\n", status.PID, status.SocketPath)
						return nil
					}
					fmt.Println("stopped")
					return nil
				},
			},
			{
				Name:  "ping",
				Usage: "Ping daemon health endpoint",
				Action: func(c *cli.Context) error {
					cfg, err := loadConfig(c)
					if err != nil {
						return err
					}
					opts := daemon.ResolveOptions(cfg)
					if err := daemon.Ping(opts.SocketPath, 1*time.Second); err != nil {
						return cli.Exit(fmt.Sprintf("unreachable: %v", err), 1)
					}
					if c.Bool("json") {
						return outputJSON(map[string]interface{}{
							"ok":      true,
							"running": true,
						})
					}
					fmt.Println("pong")
					return nil
				},
			},
			{
				Name:  "stop",
				Usage: "Stop running daemon",
				Action: func(c *cli.Context) error {
					cfg, err := loadConfig(c)
					if err != nil {
						return err
					}
					opts := daemon.ResolveOptions(cfg)

					if err := daemon.Stop(opts.SocketPath, 2*time.Second); err != nil {
						status, statusErr := daemon.StatusFromOptions(opts, 300*time.Millisecond)
						if statusErr == nil && !status.Running {
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

func applyDaemonRuntimeEnv(display string) {
	display = strings.TrimSpace(display)
	if display == "" {
		return
	}
	_ = os.Setenv("DISPLAY", display)
}
