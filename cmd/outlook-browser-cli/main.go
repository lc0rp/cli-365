package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/lc0rp/outlook-browser-cli/internal/browser"
	"github.com/lc0rp/outlook-browser-cli/internal/config"
)

func main() {
	app := &cli.App{
		Name:  "outlook-browser-cli",
		Usage: "CLI for Outlook OWA via a managed browser",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to config file (default: XDG config dir)",
			},
		},
		Commands: []*cli.Command{
			authCommand(),
			browserCommand(),
			mailCommand(),
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

func authCommand() *cli.Command {
	return &cli.Command{
		Name:  "auth",
		Usage: "Authenticate and manage stored credentials",
		Subcommands: []*cli.Command{
			{
				Name:  "login",
				Usage: "Login (TODO)",
				Action: func(*cli.Context) error {
					return cli.Exit("TODO: auth login", 1)
				},
			},
			{
				Name:  "logout",
				Usage: "Logout (TODO)",
				Action: func(*cli.Context) error {
					return cli.Exit("TODO: auth logout", 1)
				},
			},
			{
				Name:  "status",
				Usage: "Auth status (TODO)",
				Action: func(*cli.Context) error {
					return cli.Exit("TODO: auth status", 1)
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
					fmt.Printf("browser started (managed=%t)\nws endpoint: %s\n", rt.Managed, rt.WSEndpoint)
					return nil
				},
			},
			{
				Name:  "status",
				Usage: "Show current browser status",
				Action: func(*cli.Context) error {
					rt, err := browser.Status()
					if err != nil {
						if os.IsNotExist(err) {
							fmt.Println("stopped")
							return nil
						}
						return err
					}
					fmt.Printf("managed=%t pid=%d\nws endpoint: %s\n", rt.Managed, rt.PID, rt.WSEndpoint)
					return nil
				},
			},
			{
				Name:  "stop",
				Usage: "Stop the managed browser",
				Action: func(*cli.Context) error {
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
				Name:  "search",
				Usage: "Search messages (TODO)",
				Action: func(*cli.Context) error {
					return cli.Exit("TODO: mail search", 1)
				},
			},
			{
				Name:  "thread",
				Usage: "Thread operations",
				Subcommands: []*cli.Command{
					{
						Name:  "get",
						Usage: "Get thread details (TODO)",
						Action: func(*cli.Context) error {
							return cli.Exit("TODO: mail thread get", 1)
						},
					},
				},
			},
			{
				Name:  "draft",
				Usage: "Draft operations",
				Subcommands: []*cli.Command{
					{Name: "create", Action: func(*cli.Context) error { return cli.Exit("TODO: mail draft create", 1) }},
					{Name: "update", Action: func(*cli.Context) error { return cli.Exit("TODO: mail draft update", 1) }},
					{Name: "delete", Action: func(*cli.Context) error { return cli.Exit("TODO: mail draft delete", 1) }},
					{Name: "send", Action: func(*cli.Context) error { return cli.Exit("TODO: mail draft send", 1) }},
				},
			},
			{
				Name:  "send",
				Usage: "Send a message (TODO)",
				Action: func(*cli.Context) error {
					return cli.Exit("TODO: mail send", 1)
				},
			},
			{
				Name:  "attachments",
				Usage: "Attachment operations",
				Subcommands: []*cli.Command{
					{Name: "list", Action: func(*cli.Context) error { return cli.Exit("TODO: attachments list", 1) }},
					{Name: "download", Action: func(*cli.Context) error { return cli.Exit("TODO: attachments download", 1) }},
				},
			},
		},
	}
}
