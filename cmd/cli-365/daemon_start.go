package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/lc0rp/cli-365/internal/daemon"
)

func runDaemonStart(c *cli.Context) error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return cli.Exit(daemon.ErrUnsupportedPlatform.Error(), 1)
	}

	cfg, err := loadConfig(c)
	if err != nil {
		return err
	}
	opts := daemon.ResolveOptions(cfg)

	logPath := strings.TrimSpace(c.String("log-file"))
	if logPath == "" {
		logPath = filepath.Join(opts.StateDir, "daemon.log")
	}
	logPath = filepath.Clean(logPath)

	waitTimeout := c.Duration("wait")
	if waitTimeout <= 0 {
		waitTimeout = 5 * time.Second
	}

	status, err := daemon.StatusFromOptions(opts, 300*time.Millisecond)
	alreadyRunning := err == nil && status.Running
	if !alreadyRunning {
		printDaemonStartProgress(c, "Starting daemon ...")
		if err := launchDaemonDetached(c, logPath); err != nil {
			return cli.Exit(fmt.Sprintf("failed to launch daemon: %v", err), 1)
		}
		if err := waitForDaemonReady(opts.SocketPath, waitTimeout); err != nil {
			return cli.Exit(fmt.Sprintf("daemon did not become ready: %v (log: %s)", err, logPath), 1)
		}
	}
	warmupErr := warmupDaemonBrowser(c, opts, logPath)
	for _, msg := range warmupWarningMessages(warmupErr) {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}

	status, err = daemon.StatusFromOptions(opts, 500*time.Millisecond)
	if err != nil {
		return err
	}
	return outputDaemonStartStatus(c, status, logPath, alreadyRunning, warmupErr)
}

func outputDaemonStartStatus(c *cli.Context, status daemon.Status, logPath string, alreadyRunning bool, warmupErr error) error {
	if c.Bool("json") {
		warmupError := ""
		if warmupErr != nil {
			warmupError = warmupErr.Error()
		}
		return outputJSON(map[string]interface{}{
			"running":         status.Running,
			"pid":             status.PID,
			"socket_path":     status.SocketPath,
			"lock_path":       status.LockPath,
			"started_at":      status.StartedAt,
			"stopped_at":      status.StoppedAt,
			"last_error":      status.LastError,
			"log_path":        logPath,
			"already_running": alreadyRunning,
			"browser_warmup":  warmupErr == nil,
			"warmup_error":    warmupError,
		})
	}

	if status.Running {
		fmt.Printf("running pid=%d\nsocket: %s\nlog: %s\n", status.PID, status.SocketPath, logPath)
		if alreadyRunning {
			fmt.Println("note: daemon already running")
		}
		return nil
	}

	fmt.Printf("stopped\nlog: %s\n", logPath)
	return nil
}

func launchDaemonDetached(c *cli.Context, logPath string) error {
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

	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmd := exec.Command(exePath, args...)
	cmd.Env = os.Environ()
	detachDaemonProcess(cmd)
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	return nil
}

func waitForDaemonReady(socketPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = daemon.Ping(socketPath, 300*time.Millisecond)
		if lastErr == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out after %s", timeout)
	}
	return lastErr
}

func warmupDaemonBrowser(c *cli.Context, opts daemon.Options, logPath string) error {
	var errs []error

	printDaemonStartProgress(c, "Starting browser ...")
	browserResp, err := callDaemonExec(c, opts, "browser start", []string{"browser", "start"})
	if err != nil {
		return fmt.Errorf("browser warmup failed: %w", err)
	}
	if browserResp.ExitCode != 0 {
		return fmt.Errorf("browser warmup failed: %s", daemonResponseError(browserResp))
	}

	printDaemonStartProgress(c, "Starting auth ...")
	authResp, err := callDaemonExecWithProgress(c, opts, "auth login", []string{"auth", "login"}, logPath)
	if err != nil {
		errs = append(errs, fmt.Errorf("auth warmup failed: %w", err))
	} else if authResp.ExitCode != 0 {
		errs = append(errs, fmt.Errorf("auth warmup failed: %s", daemonResponseError(authResp)))
	}

	return errors.Join(errs...)
}

func callDaemonExec(c *cli.Context, opts daemon.Options, commandPath string, argv []string) (daemon.Response, error) {
	requestTimeout := opts.DefaultCommandTimeout
	if requestTimeout <= 0 {
		requestTimeout = 2 * time.Minute
	}
	callTimeout := requestTimeout + (15 * time.Second)

	requestCDPPort := 0
	if c.IsSet("cdp-port") {
		requestCDPPort = c.Int("cdp-port")
	}

	resp, err := daemon.Call(opts.SocketPath, daemon.Request{
		RequestID:   fmt.Sprintf("daemon-start-%s-%d", strings.ReplaceAll(commandPath, " ", "-"), time.Now().UnixNano()),
		SubmittedAt: time.Now().UTC(),
		Command:     daemon.CommandExec,
		CommandPath: commandPath,
		Argv:        argv,
		TimeoutMS:   int(requestTimeout / time.Millisecond),
		CDPPort:     requestCDPPort,
	}, callTimeout)
	return resp, err
}

func callDaemonExecWithProgress(c *cli.Context, opts daemon.Options, commandPath string, argv []string, logPath string) (daemon.Response, error) {
	if c == nil || c.Bool("json") || strings.TrimSpace(logPath) == "" {
		return callDaemonExec(c, opts, commandPath, argv)
	}

	type callResult struct {
		resp daemon.Response
		err  error
	}
	done := make(chan callResult, 1)
	go func() {
		resp, err := callDaemonExec(c, opts, commandPath, argv)
		done <- callResult{resp: resp, err: err}
	}()

	offset := logFileSize(logPath)
	ticker := time.NewTicker(1200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case result := <-done:
			return result.resp, result.err
		case <-ticker.C:
			messages, nextOffset := readAuthProgressMessages(logPath, offset)
			offset = nextOffset
			for _, msg := range messages {
				printDaemonStartProgress(c, msg)
			}
		}
	}
}

func logFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func readAuthProgressMessages(path string, offset int64) ([]string, int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset
	}

	var messages []string
	reader := bufio.NewReader(f)
	nextOffset := offset
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			nextOffset += int64(len(line))
			msg := authProgressMessage(line)
			if msg != "" {
				messages = append(messages, msg)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}
	}

	return messages, nextOffset
}

func authProgressMessage(line string) string {
	var entry struct {
		Event                   string `json:"event"`
		Stage                   string `json:"stage"`
		Detail                  string `json:"detail"`
		SecureInputExpiresInSec int    `json:"secure_input_expires_in_sec"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &entry); err != nil {
		return ""
	}

	switch entry.Event {
	case "auth_recovery_start":
		return "Auth recovery started ..."
	case "auth_stage_update":
		stage := strings.TrimSpace(entry.Stage)
		if stage == "" {
			return ""
		}
		return "Auth stage: " + strings.ReplaceAll(stage, "_", " ")
	case "auth_secure_input_prompt":
		return "Waiting for secure input submission ..."
	case "auth_secure_input_url":
		if entry.SecureInputExpiresInSec > 0 {
			return fmt.Sprintf(
				"Secure input URL sent to Discord (expires in %s).",
				(time.Duration(entry.SecureInputExpiresInSec) * time.Second).Round(time.Second),
			)
		}
		return "Secure input URL sent to Discord."
	case "auth_kmsi_continue":
		return "Handling stay signed in prompt ..."
	case "auth_recovery_success":
		return "Auth recovery successful."
	case "auth_recovery_timeout":
		return "Auth recovery timed out."
	default:
		return ""
	}
}

func daemonResponseError(resp daemon.Response) string {
	msg := strings.TrimSpace(resp.Stderr)
	if msg == "" {
		msg = strings.TrimSpace(resp.Stdout)
	}
	if msg == "" {
		msg = strings.TrimSpace(resp.ErrorCode)
	}
	if msg == "" {
		msg = "unknown daemon error"
	}
	return fmt.Sprintf("%s (exit=%d)", msg, resp.ExitCode)
}

func warmupWarningMessages(err error) []string {
	if err == nil {
		return nil
	}
	var warnings []string
	for _, part := range strings.Split(err.Error(), "\n") {
		line := strings.TrimSpace(part)
		if line == "" {
			continue
		}
		warnings = append(warnings, line)
	}
	if len(warnings) == 0 {
		return []string{err.Error()}
	}
	return warnings
}

func printDaemonStartProgress(c *cli.Context, message string) {
	if c != nil && c.Bool("json") {
		return
	}
	fmt.Println(message)
}
