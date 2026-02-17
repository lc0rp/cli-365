package daemon

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDefaultAuthRecoveryProbePrefersLiveSessionState(t *testing.T) {
	opts := testOptions(t)
	srv := NewServer(opts, func(_ context.Context, argv []string, _ time.Duration) ExecResult {
		if len(argv) >= 2 && argv[0] == "auth" && argv[1] == "status" {
			return ExecResult{
				ExitCode: 0,
				Stdout:   `{"authenticated":true}`,
			}
		}
		return ExecResult{ExitCode: 0}
	})
	srv.sessionProbe = func(_ context.Context) (bool, error) {
		return false, nil
	}

	ok, err := srv.defaultAuthRecoveryProbe(context.Background())
	if err != nil {
		t.Fatalf("defaultAuthRecoveryProbe() error = %v", err)
	}
	if ok {
		t.Fatal("defaultAuthRecoveryProbe() = true, want false when live session is not ready")
	}
}

func TestDefaultAuthRecoveryProbeReturnsUnavailableSignal(t *testing.T) {
	opts := testOptions(t)
	srv := NewServer(opts, nil)
	srv.sessionProbe = func(_ context.Context) (bool, error) {
		return false, errSessionProbeUnavailable
	}

	ok, err := srv.defaultAuthRecoveryProbe(context.Background())
	if ok {
		t.Fatal("defaultAuthRecoveryProbe() = true, want false")
	}
	if !errors.Is(err, errSessionProbeUnavailable) {
		t.Fatalf("defaultAuthRecoveryProbe() err = %v, want errSessionProbeUnavailable", err)
	}
}

func TestRunAuthRecoveryDoesNotExitEarlyFromCachedAuthStatus(t *testing.T) {
	opts := testOptions(t)
	opts.AuthRecoveryTimeout = 80 * time.Millisecond
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.SecureInputCommand = "secure-targeted-input"

	srv := NewServer(opts, func(_ context.Context, argv []string, _ time.Duration) ExecResult {
		if len(argv) >= 2 && argv[0] == "auth" && argv[1] == "status" {
			return ExecResult{
				ExitCode: 0,
				Stdout:   `{"authenticated":true}`,
			}
		}
		return ExecResult{ExitCode: 0}
	})
	srv.sessionProbe = func(_ context.Context) (bool, error) {
		return false, nil
	}
	srv.secureInputRunner = func(ctx context.Context, _ string, _ func(string)) error {
		<-ctx.Done()
		return ctx.Err()
	}
	srv.notifyAuth = func(_ context.Context, _ AuthNotification) error { return nil }

	ok := srv.runAuthRecovery(context.Background())
	if ok {
		t.Fatal("runAuthRecovery() = true, want false when login is still incomplete")
	}
}

func TestRunAuthRecoveryRetriesSecureInputRunnerErrors(t *testing.T) {
	opts := testOptions(t)
	opts.AuthRecoveryTimeout = 2 * time.Second
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.SecureInputCommand = "secure-targeted-input"

	srv := NewServer(opts, nil)

	var secureCalls int
	srv.secureInputRunner = func(_ context.Context, _ string, _ func(string)) error {
		secureCalls++
		if secureCalls == 1 {
			return errors.New("temporary secure-input failure")
		}
		return nil
	}
	srv.authProbe = func(_ context.Context) (bool, error) {
		return secureCalls >= 2, nil
	}
	srv.notifyAuth = func(_ context.Context, _ AuthNotification) error { return nil }

	ok := srv.runAuthRecovery(context.Background())
	if !ok {
		t.Fatal("runAuthRecovery() = false, want true after secure-input retry")
	}
	if secureCalls < 2 {
		t.Fatalf("secure input calls = %d, want >=2", secureCalls)
	}
}

func TestRunAuthRecoveryProbeUnavailableTriggersBrowserRecovery(t *testing.T) {
	opts := testOptions(t)
	opts.AuthRecoveryTimeout = 250 * time.Millisecond
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.SecureInputCommand = "secure-targeted-input"

	var browserStartCalls int
	srv := NewServer(opts, func(_ context.Context, argv []string, _ time.Duration) ExecResult {
		if len(argv) >= 2 && argv[0] == "browser" && argv[1] == "start" {
			browserStartCalls++
			return ExecResult{ExitCode: 0}
		}
		return ExecResult{ExitCode: 0}
	})
	srv.maintainPrimaryFn = func() {}

	var probeCalls int
	srv.authProbe = func(_ context.Context) (bool, error) {
		probeCalls++
		if probeCalls == 1 {
			return false, errSessionProbeUnavailable
		}
		return true, nil
	}
	srv.secureInputRunner = func(_ context.Context, _ string, _ func(string)) error { return nil }
	srv.notifyAuth = func(_ context.Context, _ AuthNotification) error { return nil }

	ok := srv.runAuthRecovery(context.Background())
	if !ok {
		t.Fatal("runAuthRecovery() = false, want true")
	}
	if browserStartCalls != 1 {
		t.Fatalf("browser start calls = %d, want 1", browserStartCalls)
	}
}

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
	srv.secureInputRunner = func(_ context.Context, command string, _ func(string)) error {
		secureCalls++
		if !strings.HasPrefix(command, "secure-targeted-input") {
			t.Fatalf("secure input command = %q, want secure-targeted-input prefix", command)
		}
		if !strings.Contains(command, "--selector ") {
			t.Fatalf("secure input command = %q, want selector defaults", command)
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
	srv.secureInputRunner = func(ctx context.Context, _ string, _ func(string)) error {
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

func TestAuthRecoveryStopsWhenDaemonStopRequested(t *testing.T) {
	opts := testOptions(t)
	opts.AuthRecoveryTimeout = 5 * time.Second
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.SecureInputCommand = "secure-targeted-input"

	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		return ExecResult{ExitCode: 0}
	})
	srv.authProbe = func(_ context.Context) (bool, error) { return false, nil }
	srv.secureInputRunner = func(ctx context.Context, _ string, _ func(string)) error {
		<-ctx.Done()
		return ctx.Err()
	}
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

	srv.requestGracefulStop()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("runAuthRecovery() = true, want false after daemon stop request")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runAuthRecovery did not stop after daemon stop request")
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
	srv.secureInputRunner = func(_ context.Context, _ string, _ func(string)) error { return nil }
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
	srv.secureInputRunner = func(_ context.Context, _ string, _ func(string)) error { return nil }
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
	srv.secureInputRunner = func(ctx context.Context, _ string, _ func(string)) error {
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

func TestPrepareSecureInputCommandAddsSelectorsForBareCommand(t *testing.T) {
	got := prepareSecureInputCommand("secure-targeted-input", 9222, "", "", "")
	if got == "" {
		t.Fatal("prepareSecureInputCommand() returned empty command")
	}
	if !strings.Contains(got, "--selector ") {
		t.Fatalf("prepared command = %q, want --selector", got)
	}
	if !strings.Contains(got, "--selector-2 ") {
		t.Fatalf("prepared command = %q, want --selector-2", got)
	}
	if !strings.Contains(got, "--submit-selector #idSIButton9,button[type=submit],input[type=submit]") {
		t.Fatalf("prepared command = %q, want password submit selector", got)
	}
	if !strings.Contains(got, "--target-tab-url microsoftonline.com") {
		t.Fatalf("prepared command = %q, want --target-tab-url microsoftonline.com", got)
	}
	if !strings.Contains(got, "--cdp-port 9222") {
		t.Fatalf("prepared command = %q, want --cdp-port 9222", got)
	}
}

func TestPrepareSecureInputCommandPreservesCustomSelector(t *testing.T) {
	input := "secure-targeted-input --selector input[type=password] --target-tab-url login.microsoftonline.com"
	got := prepareSecureInputCommand(input, 9222, "", "", "")
	if !strings.Contains(got, "--selector input[type=password]") {
		t.Fatalf("prepareSecureInputCommand() = %q, want existing selector", got)
	}
	if !strings.Contains(got, "--submit-selector #idSIButton9,button[type=submit],input[type=submit]") {
		t.Fatalf("prepareSecureInputCommand() = %q, want default submit selector", got)
	}
	if !strings.Contains(got, "--cdp-port 9222") {
		t.Fatalf("prepareSecureInputCommand() = %q, want --cdp-port 9222", got)
	}
}

func TestPrepareSecureInputCommandLeavesNonSecureInputCommandsUntouched(t *testing.T) {
	input := "/usr/local/bin/custom-auth"
	got := prepareSecureInputCommand(input, 9222, "", "", "")
	if got != input {
		t.Fatalf("prepareSecureInputCommand() = %q, want %q", got, input)
	}
}

func TestPrepareSecureInputOTPCommandAddsOTPSelectors(t *testing.T) {
	got := prepareSecureInputOTPCommand("secure-targeted-input", 9222, "", "", "")
	if got == "" {
		t.Fatal("prepareSecureInputOTPCommand() returned empty command")
	}
	if !strings.Contains(got, "--selector ") {
		t.Fatalf("prepared command = %q, want --selector", got)
	}
	if !strings.Contains(got, "one-time-code") {
		t.Fatalf("prepared command = %q, want otp selector", got)
	}
	if !strings.Contains(got, "--submit-selector #idSubmit_SAOTCC_Continue,#idSIButton9") {
		t.Fatalf("prepared command = %q, want otp submit selector", got)
	}
	if !strings.Contains(got, "--cdp-port 9222") {
		t.Fatalf("prepared command = %q, want --cdp-port 9222", got)
	}
}

func TestPrepareSecureInputCommandRespectsExplicitCDPArgs(t *testing.T) {
	input := "secure-targeted-input --selector input[type=password] --cdp-endpoint ws://127.0.0.1:1234/devtools/browser/x"
	got := prepareSecureInputCommand(input, 9222, "", "", "")
	if strings.Count(got, "--cdp-port") != 0 {
		t.Fatalf("prepareSecureInputCommand() = %q, want no appended --cdp-port when endpoint is explicit", got)
	}
}

func TestFirstURL(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "plain https url",
			line: "https://127.0.0.1:45321/?token=abc",
			want: "https://127.0.0.1:45321/?token=abc",
		},
		{
			name: "url wrapped in punctuation",
			line: "open this: (https://127.0.0.1:45321/?token=abc)",
			want: "https://127.0.0.1:45321/?token=abc",
		},
		{
			name: "no url",
			line: "waiting for secure input",
			want: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := firstURL(tt.line)
			if got != tt.want {
				t.Fatalf("firstURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAuthRecoveryEmitsSecureInputURLNotification(t *testing.T) {
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

	srv.secureInputRunner = func(_ context.Context, _ string, onURL func(string)) error {
		if onURL != nil {
			onURL("https://127.0.0.1:44444/?token=abc")
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
	if len(notes) != 2 {
		t.Fatalf("notifications = %d, want 2", len(notes))
	}
	if notes[0].Reason != "auth_required" {
		t.Fatalf("first notification reason = %q, want auth_required", notes[0].Reason)
	}
	if notes[1].Reason != "secure_input_url" {
		t.Fatalf("second notification reason = %q, want secure_input_url", notes[1].Reason)
	}
	if notes[1].SecureInputURL != "https://127.0.0.1:44444/?token=abc" {
		t.Fatalf("secure_input_url = %q", notes[1].SecureInputURL)
	}
	if notes[1].SecureInputExpiresAt.IsZero() {
		t.Fatal("secure input expiry not set")
	}
	if !notes[1].SecureInputExpiresAt.After(notes[1].At) {
		t.Fatalf(
			"secure input expiry = %s, want after notification time %s",
			notes[1].SecureInputExpiresAt.Format(time.RFC3339Nano),
			notes[1].At.Format(time.RFC3339Nano),
		)
	}
}

func TestAuthRecoveryInjectsCDPPortIntoSecureInputCommand(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	opts := testOptions(t)
	opts.AuthRecoveryTimeout = 250 * time.Millisecond
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.SecureInputCommand = "secure-targeted-input"
	opts.CDPPort = 9222

	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		return ExecResult{ExitCode: 0}
	})

	var probeCalls int
	srv.authProbe = func(_ context.Context) (bool, error) {
		probeCalls++
		return probeCalls >= 2, nil
	}

	var gotCommand string
	srv.secureInputRunner = func(_ context.Context, command string, _ func(string)) error {
		gotCommand = command
		return nil
	}
	srv.notifyAuth = func(_ context.Context, _ AuthNotification) error { return nil }

	ok := srv.runAuthRecovery(context.Background())
	if !ok {
		t.Fatal("runAuthRecovery() = false, want true")
	}
	if !strings.Contains(gotCommand, "--cdp-port 9222") {
		t.Fatalf("secure input command = %q, want --cdp-port 9222", gotCommand)
	}
}

func TestPrepareSecureInputCommandPrefersCDPEndpoint(t *testing.T) {
	got := prepareSecureInputCommand("secure-targeted-input", 0, "ws://127.0.0.1:45521/devtools/browser/test", "", "")
	if !strings.Contains(got, "--cdp-endpoint ws://127.0.0.1:45521/devtools/browser/test") {
		t.Fatalf("prepared command = %q, want --cdp-endpoint", got)
	}
	if strings.Contains(got, "--cdp-port") {
		t.Fatalf("prepared command = %q, want no --cdp-port", got)
	}
}

func TestPrepareSecureInputCommandAddsTargetTabID(t *testing.T) {
	got := prepareSecureInputCommand("secure-targeted-input", 0, "", "target-123", "")
	if !strings.Contains(got, "--target-tab-id target-123") {
		t.Fatalf("prepared command = %q, want --target-tab-id target-123", got)
	}
}

func TestPrepareSecureInputCommandAddsRuntimeFileAndSkipsEndpointWhenAvailable(t *testing.T) {
	got := prepareSecureInputCommand(
		"secure-targeted-input",
		0,
		"ws://127.0.0.1:45521/devtools/browser/test",
		"",
		"/tmp/runtime.json",
	)
	if !strings.Contains(got, "--cli-365-runtime-config-file /tmp/runtime.json") {
		t.Fatalf("prepared command = %q, want --cli-365-runtime-config-file", got)
	}
	if strings.Contains(got, "--cdp-endpoint") {
		t.Fatalf("prepared command = %q, want no --cdp-endpoint when runtime file is present", got)
	}
}
