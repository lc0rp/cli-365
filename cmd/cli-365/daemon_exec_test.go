package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lc0rp/cli-365/internal/daemon"
)

type cliInvocationResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func daemonTestOptions(t *testing.T) daemon.Options {
	t.Helper()
	base := filepath.Join(t.TempDir(), "daemon")
	return daemon.Options{
		StateDir:              base,
		SocketPath:            filepath.Join(base, "daemon.sock"),
		LockPath:              filepath.Join(base, "daemon.lock"),
		StatusPath:            filepath.Join(base, "daemon.json"),
		DefaultCommandTimeout: 2 * time.Second,
		MaxQueueSize:          8,
		MaxRequestBytes:       1024 * 1024,
		RejectNewWhilePaused:  true,
	}
}

func waitForDaemonPing(t *testing.T, socketPath string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := daemon.Ping(socketPath, 200*time.Millisecond); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("daemon did not respond to ping within %s", timeout)
}

func runDirectCLI(t *testing.T, argv []string) cliInvocationResult {
	t.Helper()
	exitCode := 0
	stdout, stderr, err := captureProcessStdio(func() {
		exitCode = runCLI(context.Background(), append([]string{"cli-365"}, argv...), cliAppOptions{})
	})
	if err != nil {
		t.Fatalf("captureProcessStdio() error: %v", err)
	}
	return cliInvocationResult{
		exitCode: exitCode,
		stdout:   stdout,
		stderr:   stderr,
	}
}

func runDaemonCLI(t *testing.T, socketPath string, argv []string) cliInvocationResult {
	t.Helper()
	resp, err := daemon.Call(socketPath, daemon.Request{
		RequestID: "parity",
		Command:   daemon.CommandExec,
		Argv:      argv,
		TimeoutMS: 2000,
	}, 3*time.Second)
	if err != nil {
		t.Fatalf("daemon.Call() error: %v", err)
	}
	return cliInvocationResult{
		exitCode: resp.ExitCode,
		stdout:   resp.Stdout,
		stderr:   resp.Stderr,
	}
}

func TestDaemonInProcessDispatchParity(t *testing.T) {
	opts := daemonTestOptions(t)
	srv := daemon.NewServer(opts, daemonExecFunc())

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()

	waitForDaemonPing(t, opts.SocketPath, 3*time.Second)

	cases := []struct {
		name string
		argv []string
	}{
		{
			name: "help",
			argv: []string{"help"},
		},
		{
			name: "unknown command blocked by allowlist",
			argv: []string{"no-such-command"},
		},
		{
			name: "missing help topic",
			argv: []string{"help", "missing-topic"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			direct := runDirectCLI(t, tc.argv)
			daemonResult := runDaemonCLI(t, opts.SocketPath, tc.argv)

			if daemonResult.exitCode != direct.exitCode {
				t.Fatalf("exit code = %d, want %d", daemonResult.exitCode, direct.exitCode)
			}
			if daemonResult.stdout != direct.stdout {
				t.Fatalf("stdout mismatch\n--- daemon ---\n%s\n--- direct ---\n%s", daemonResult.stdout, direct.stdout)
			}
			if daemonResult.stderr != direct.stderr {
				t.Fatalf("stderr mismatch\n--- daemon ---\n%s\n--- direct ---\n%s", daemonResult.stderr, direct.stderr)
			}
		})
	}

	if err := daemon.Stop(opts.SocketPath, 2*time.Second); err != nil {
		t.Fatalf("daemon.Stop() error: %v", err)
	}

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("daemon.Run() error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for daemon shutdown")
	}
}
