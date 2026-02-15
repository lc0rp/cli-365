package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/lc0rp/cli-365/internal/daemon"
)

func runViaDaemon(c *cli.Context) error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return cli.Exit(daemon.ErrUnsupportedPlatform.Error(), 1)
	}

	cfg, err := loadConfig(c)
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}
	opts := daemon.ResolveOptions(cfg)

	if err := ensureDaemonAvailable(c, opts); err != nil {
		return cli.Exit(fmt.Sprintf("%s: %v", daemon.ErrorCodeDaemonUnavailable, err), 1)
	}

	argv := stripDaemonFlag(os.Args[1:])
	if len(argv) == 0 {
		return cli.Exit("no command to execute", 1)
	}

	timeout := opts.DefaultCommandTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	callTimeout := timeout + (5 * time.Second)
	requestCDPPort := 0
	if c.IsSet("cdp-port") {
		requestCDPPort = c.Int("cdp-port")
	}

	resp, err := daemon.Call(opts.SocketPath, daemon.Request{
		RequestID:   fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid()),
		SubmittedAt: time.Now().UTC(),
		Command:     daemon.CommandExec,
		CommandPath: buildCommandPath(c),
		Argv:        argv,
		TimeoutMS:   int(timeout / time.Millisecond),
		CDPPort:     requestCDPPort,
	}, callTimeout)
	if err != nil {
		return cli.Exit(fmt.Sprintf("%s: %v", daemon.ErrorCodeDaemonUnavailable, err), 1)
	}

	if resp.Stdout != "" {
		_, _ = io.WriteString(os.Stdout, resp.Stdout)
	}
	if resp.Stderr != "" {
		_, _ = io.WriteString(os.Stderr, resp.Stderr)
	}
	if resp.ExitCode != 0 {
		return cli.Exit("", resp.ExitCode)
	}
	return cli.Exit("", 0)
}

func ensureDaemonAvailable(c *cli.Context, opts daemon.Options) error {
	if err := daemon.Ping(opts.SocketPath, 300*time.Millisecond); err == nil {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	args := make([]string, 0, 8)
	if configPath := strings.TrimSpace(c.String("config")); configPath != "" {
		args = append(args, "--config", configPath)
	}
	if c.IsSet("cdp-port") {
		args = append(args, "--cdp-port", strconv.Itoa(c.Int("cdp-port")))
	}
	args = append(args, "daemon", "run")

	cmd := exec.Command(exePath, args...)
	cmd.Env = os.Environ()
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := daemon.Ping(opts.SocketPath, 300*time.Millisecond); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon did not start within timeout")
}

func stripDaemonFlag(args []string) []string {
	out := make([]string, 0, len(args))
	skipNext := false
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--daemon" {
			if i+1 < len(args) {
				next := strings.ToLower(strings.TrimSpace(args[i+1]))
				if next == "true" || next == "false" || next == "1" || next == "0" {
					skipNext = true
				}
			}
			continue
		}
		if strings.HasPrefix(arg, "--daemon=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}
