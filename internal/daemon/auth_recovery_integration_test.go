package daemon

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestServerAuthRecoveryRejectsNewRequestsWhilePaused(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}
	opts.AuthRecoveryTimeout = 2 * time.Second
	opts.AuthProbeInterval = 10 * time.Millisecond
	opts.SecureInputCommand = "secure-targeted-input"

	var (
		mu          sync.Mutex
		execCalls   int
		secureCalls int
	)
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		mu.Lock()
		execCalls++
		call := execCalls
		mu.Unlock()

		if call == 1 {
			return ExecResult{
				ExitCode: 1,
				Err:      ErrAuthRequired,
				Stderr:   "not logged in - run 'auth login' first",
			}
		}
		return ExecResult{
			ExitCode: 0,
			Stdout:   "ok\n",
		}
	})

	releaseProbe := make(chan struct{})
	srv.authProbe = func(ctx context.Context) (bool, error) {
		select {
		case <-releaseProbe:
			return true, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	srv.secureInputRunner = func(_ context.Context, command string) error {
		mu.Lock()
		secureCalls++
		mu.Unlock()
		if command != "secure-targeted-input" {
			t.Fatalf("secure input command = %q, want secure-targeted-input", command)
		}
		return nil
	}
	srv.notifyAuth = func(_ context.Context, _ AuthNotification) error { return nil }

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()
	waitForPing(t, opts.SocketPath, 3*time.Second)

	type firstCallResult struct {
		resp Response
		err  error
	}
	firstDone := make(chan firstCallResult, 1)
	go func() {
		resp, err := Call(opts.SocketPath, Request{
			RequestID:   "auth-paused-1",
			Command:     CommandExec,
			CommandPath: "mail draft create",
			Argv:        []string{"mail", "draft", "create", "--subject", "a"},
		}, 3*time.Second)
		firstDone <- firstCallResult{resp: resp, err: err}
	}()

	deadline := time.Now().Add(400 * time.Millisecond)
	for !srv.isPaused() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !srv.isPaused() {
		t.Fatal("server did not enter paused state")
	}

	second, err := Call(opts.SocketPath, Request{
		RequestID:   "auth-paused-2",
		Command:     CommandExec,
		CommandPath: "mail draft create",
		Argv:        []string{"mail", "draft", "create", "--subject", "b"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("second Call() error: %v", err)
	}
	if second.OK {
		t.Fatal("second response OK, want AUTH_PAUSED")
	}
	if second.ErrorCode != ErrorCodeAuthPaused {
		t.Fatalf("second ErrorCode = %q, want %q", second.ErrorCode, ErrorCodeAuthPaused)
	}

	close(releaseProbe)

	select {
	case result := <-firstDone:
		if result.err != nil {
			t.Fatalf("first Call() error: %v", result.err)
		}
		if !result.resp.OK || result.resp.ExitCode != 0 {
			t.Fatalf("first response not ok: code=%q stderr=%q", result.resp.ErrorCode, result.resp.Stderr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for first request response")
	}

	mu.Lock()
	gotExecCalls := execCalls
	gotSecureCalls := secureCalls
	mu.Unlock()
	if gotExecCalls != 2 {
		t.Fatalf("execCalls = %d, want 2 (retry after recovery)", gotExecCalls)
	}
	if gotSecureCalls != 1 {
		t.Fatalf("secure input calls = %d, want 1", gotSecureCalls)
	}

	if err := Stop(opts.SocketPath, 2*time.Second); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for server shutdown")
	}
}

func TestServerAuthRecoveryTimeoutReturnsAuthTimeout(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}
	opts.AuthRecoveryTimeout = 80 * time.Millisecond
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.SecureInputCommand = "secure-targeted-input"

	var (
		mu        sync.Mutex
		execCalls int
		notes     []AuthNotification
	)
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		mu.Lock()
		execCalls++
		mu.Unlock()
		return ExecResult{
			ExitCode: 1,
			Err:      ErrAuthRequired,
			Stderr:   "status 401 unauthorized",
		}
	})
	srv.authProbe = func(_ context.Context) (bool, error) { return false, nil }
	srv.secureInputRunner = func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}
	srv.notifyAuth = func(_ context.Context, note AuthNotification) error {
		mu.Lock()
		notes = append(notes, note)
		mu.Unlock()
		return nil
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()
	waitForPing(t, opts.SocketPath, 3*time.Second)

	resp, err := Call(opts.SocketPath, Request{
		RequestID:   "auth-timeout",
		Command:     CommandExec,
		CommandPath: "mail search",
		Argv:        []string{"mail", "search", "invoice"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}
	if resp.OK {
		t.Fatal("response OK, want AUTH_TIMEOUT")
	}
	if resp.ErrorCode != ErrorCodeAuthTimeout {
		t.Fatalf("ErrorCode = %q, want %q", resp.ErrorCode, ErrorCodeAuthTimeout)
	}

	if got := srv.currentAuthState(); got != AuthStateFailed {
		t.Fatalf("auth state = %s, want %s", got, AuthStateFailed)
	}
	if srv.isPaused() {
		t.Fatal("server should not remain paused after timeout")
	}

	mu.Lock()
	gotExecCalls := execCalls
	gotNotes := append([]AuthNotification{}, notes...)
	mu.Unlock()
	if gotExecCalls != 1 {
		t.Fatalf("execCalls = %d, want 1", gotExecCalls)
	}
	if len(gotNotes) < 2 {
		t.Fatalf("notifications = %d, want >=2", len(gotNotes))
	}
	if gotNotes[0].Reason != "auth_required" {
		t.Fatalf("first notification reason = %q, want auth_required", gotNotes[0].Reason)
	}
	if gotNotes[1].Reason != "auth_timeout" {
		t.Fatalf("second notification reason = %q, want auth_timeout", gotNotes[1].Reason)
	}

	if err := Stop(opts.SocketPath, 2*time.Second); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for server shutdown")
	}
}
