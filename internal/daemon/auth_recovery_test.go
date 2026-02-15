package daemon

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAuthRecoverySuccessTransitionsToReady(t *testing.T) {
	opts := testOptions(t)
	opts.AuthRecoveryTimeout = 250 * time.Millisecond
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.LoginURL = "https://outlook.office.com/mail/"
	opts.SecureInputCommand = "secure-targeted-input"

	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		return ExecResult{ExitCode: 0}
	})

	var probeCalls int
	srv.authProbe = func(_ context.Context) (bool, error) {
		probeCalls++
		return probeCalls >= 2, nil
	}

	var secureCalls int
	srv.secureInputRunner = func(_ context.Context, command string) error {
		secureCalls++
		if command != "secure-targeted-input" {
			t.Fatalf("secure input command = %q, want secure-targeted-input", command)
		}
		return nil
	}

	var notes []AuthNotification
	srv.notifyAuth = func(_ context.Context, note AuthNotification) error {
		notes = append(notes, note)
		return nil
	}

	ok := srv.runAuthRecovery(context.Background())
	if !ok {
		t.Fatal("runAuthRecovery() = false, want true")
	}
	if got := srv.currentAuthState(); got != AuthStateReady {
		t.Fatalf("auth state = %s, want %s", got, AuthStateReady)
	}
	if srv.isPaused() {
		t.Fatal("server remained paused after successful recovery")
	}
	if secureCalls != 1 {
		t.Fatalf("secure input calls = %d, want 1", secureCalls)
	}
	if probeCalls < 2 {
		t.Fatalf("probe calls = %d, want >=2", probeCalls)
	}
	if len(notes) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notes))
	}
	if notes[0].Reason != "auth_required" {
		t.Fatalf("notification reason = %q, want auth_required", notes[0].Reason)
	}
	if notes[0].LoginURL != opts.LoginURL {
		t.Fatalf("notification login_url = %q, want %q", notes[0].LoginURL, opts.LoginURL)
	}
}

func TestAuthRecoveryTimeoutTransitionsToFailedAndDrainsQueue(t *testing.T) {
	opts := testOptions(t)
	opts.AuthRecoveryTimeout = 80 * time.Millisecond
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.SecureInputCommand = "secure-targeted-input"

	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		return ExecResult{ExitCode: 0}
	})
	srv.authProbe = func(_ context.Context) (bool, error) {
		return false, nil
	}
	srv.secureInputRunner = func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}

	var (
		mu    sync.Mutex
		notes []AuthNotification
	)
	srv.notifyAuth = func(_ context.Context, note AuthNotification) error {
		mu.Lock()
		notes = append(notes, note)
		mu.Unlock()
		return nil
	}

	p1 := queuedExec{
		req:    Request{RequestID: "pending-1"},
		respCh: make(chan Response, 1),
	}
	p2 := queuedExec{
		req:    Request{RequestID: "pending-2"},
		respCh: make(chan Response, 1),
	}
	srv.execQ <- p1
	srv.execQ <- p2

	ok := srv.runAuthRecovery(context.Background())
	if ok {
		t.Fatal("runAuthRecovery() = true, want false on timeout")
	}
	if got := srv.currentAuthState(); got != AuthStateFailed {
		t.Fatalf("auth state = %s, want %s", got, AuthStateFailed)
	}
	if srv.isPaused() {
		t.Fatal("server remained paused after timed-out recovery")
	}

	assertAuthTimeout := func(ch <-chan Response, wantID string) {
		t.Helper()
		select {
		case resp := <-ch:
			if resp.RequestID != wantID {
				t.Fatalf("response request_id = %q, want %q", resp.RequestID, wantID)
			}
			if resp.ErrorCode != ErrorCodeAuthTimeout {
				t.Fatalf("response error_code = %q, want %q", resp.ErrorCode, ErrorCodeAuthTimeout)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for %s", wantID)
		}
	}

	assertAuthTimeout(p1.respCh, "pending-1")
	assertAuthTimeout(p2.respCh, "pending-2")

	if len(srv.execQ) != 0 {
		t.Fatalf("queue depth = %d, want 0", len(srv.execQ))
	}

	mu.Lock()
	defer mu.Unlock()
	if len(notes) < 2 {
		t.Fatalf("notifications = %d, want at least 2", len(notes))
	}
	reasons := notes[0].Reason + "," + notes[1].Reason
	if !strings.Contains(reasons, "auth_required") || !strings.Contains(reasons, "auth_timeout") {
		t.Fatalf("notification reasons = %q, want auth_required and auth_timeout", reasons)
	}
}

func TestServerRejectsNewRequestsWhileAuthRecoveryPaused(t *testing.T) {
	opts := testOptions(t)
	opts.AuthRecoveryTimeout = time.Second
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.RejectNewWhilePaused = true

	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		return ExecResult{ExitCode: 0}
	})

	release := make(chan struct{})
	srv.authProbe = func(ctx context.Context) (bool, error) {
		select {
		case <-release:
			return true, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	srv.secureInputRunner = func(_ context.Context, _ string) error { return nil }
	srv.notifyAuth = func(_ context.Context, _ AuthNotification) error { return nil }

	done := make(chan bool, 1)
	go func() {
		done <- srv.runAuthRecovery(context.Background())
	}()

	deadline := time.Now().Add(300 * time.Millisecond)
	for !srv.isPaused() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !srv.isPaused() {
		t.Fatal("server did not enter paused state")
	}

	code := srv.tryEnqueue(queuedExec{
		req:    Request{RequestID: "queued-while-paused"},
		respCh: make(chan Response, 1),
	})
	if code != ErrorCodeAuthPaused {
		t.Fatalf("tryEnqueue code = %q, want %q", code, ErrorCodeAuthPaused)
	}

	close(release)
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("runAuthRecovery() = false, want true")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for auth recovery completion")
	}
}

func TestExecuteTaskAuthRequiredRunsRecoveryAndRetries(t *testing.T) {
	opts := testOptions(t)
	opts.AuthRecoveryTimeout = 250 * time.Millisecond
	opts.AuthProbeInterval = 5 * time.Millisecond

	var calls int
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		calls++
		if calls == 1 {
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
	srv.authProbe = func(_ context.Context) (bool, error) { return true, nil }
	srv.secureInputRunner = func(_ context.Context, _ string) error { return nil }
	srv.notifyAuth = func(_ context.Context, _ AuthNotification) error { return nil }

	resp := srv.executeTask(context.Background(), queuedExec{
		req: Request{
			RequestID:   "auth-retry",
			CommandPath: "mail search",
		},
		argv:       []string{"mail", "search", "invoice"},
		timeout:    time.Second,
		enqueuedAt: time.Now().UTC(),
		respCh:     make(chan Response, 1),
	})

	if !resp.OK {
		t.Fatalf("response not ok: code=%q stderr=%q", resp.ErrorCode, resp.Stderr)
	}
	if resp.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0", resp.ExitCode)
	}
	if resp.Stdout != "ok\n" {
		t.Fatalf("stdout = %q, want ok\\n", resp.Stdout)
	}
	if calls != 2 {
		t.Fatalf("exec calls = %d, want 2", calls)
	}
}

func TestExecuteTaskAuthRecoveryTimeoutReturnsAuthTimeout(t *testing.T) {
	opts := testOptions(t)
	opts.AuthRecoveryTimeout = 60 * time.Millisecond
	opts.AuthProbeInterval = 5 * time.Millisecond

	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
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
	srv.notifyAuth = func(_ context.Context, _ AuthNotification) error { return nil }

	pending := queuedExec{
		req:    Request{RequestID: "pending-after-timeout"},
		respCh: make(chan Response, 1),
	}
	srv.execQ <- pending

	resp := srv.executeTask(context.Background(), queuedExec{
		req: Request{
			RequestID:   "auth-timeout",
			CommandPath: "mail search",
		},
		argv:       []string{"mail", "search", "invoice"},
		timeout:    500 * time.Millisecond,
		enqueuedAt: time.Now().UTC(),
		respCh:     make(chan Response, 1),
	})
	if resp.OK {
		t.Fatal("response ok, want auth timeout failure")
	}
	if resp.ErrorCode != ErrorCodeAuthTimeout {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodeAuthTimeout)
	}

	select {
	case drained := <-pending.respCh:
		if drained.ErrorCode != ErrorCodeAuthTimeout {
			t.Fatalf("drained error_code = %q, want %q", drained.ErrorCode, ErrorCodeAuthTimeout)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for pending request drain response")
	}
}
