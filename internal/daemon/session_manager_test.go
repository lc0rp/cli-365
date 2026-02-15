package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lc0rp/cli-365/internal/owa"
	"github.com/lc0rp/cli-365/internal/paths"
)

func TestParseJWTExpiry(t *testing.T) {
	expiresAt := time.Unix(1_800_000_000, 0).UTC()
	bearer := bearerWithExp(t, expiresAt)

	got, ok := parseJWTExpiry(bearer)
	if !ok {
		t.Fatal("parseJWTExpiry() ok = false, want true")
	}
	if !got.Equal(expiresAt) {
		t.Fatalf("parseJWTExpiry() = %s, want %s", got.Format(time.RFC3339), expiresAt.Format(time.RFC3339))
	}
}

func TestRefreshTokensIfNeededTriggersRefresherNearExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	nearExpiry := now.Add(2 * time.Minute)
	farExpiry := now.Add(45 * time.Minute)

	srv := NewServer(testOptions(t), nil)
	srv.tokenLoader = func() (*owa.Tokens, error) {
		return &owa.Tokens{
			Canary: "cached-canary",
			Bearer: bearerWithExp(t, nearExpiry),
		}, nil
	}

	refreshCalls := int32(0)
	srv.tokenRefresher = func(_ context.Context) (*owa.Tokens, error) {
		atomic.AddInt32(&refreshCalls, 1)
		return &owa.Tokens{
			Canary: "fresh-canary",
			Bearer: bearerWithExp(t, farExpiry),
		}, nil
	}

	var saved *owa.Tokens
	srv.tokenSaver = func(tokens *owa.Tokens) error {
		saved = tokens
		return nil
	}

	if err := srv.refreshTokensIfNeeded(context.Background(), now); err != nil {
		t.Fatalf("refreshTokensIfNeeded() error: %v", err)
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Fatalf("tokenRefresher calls = %d, want 1", got)
	}
	if saved == nil {
		t.Fatal("tokenSaver not called")
	}
	if saved.Canary != "fresh-canary" {
		t.Fatalf("saved.Canary = %q, want fresh-canary", saved.Canary)
	}
	if saved.ExpiresAt.IsZero() {
		t.Fatal("saved.ExpiresAt is zero, want parsed expiry")
	}
	if !saved.ExpiresAt.Equal(farExpiry) {
		t.Fatalf("saved.ExpiresAt = %s, want %s", saved.ExpiresAt.Format(time.RFC3339), farExpiry.Format(time.RFC3339))
	}
}

func TestRefreshTokensIfNeededSkipsHealthyToken(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	farExpiry := now.Add(30 * time.Minute)

	srv := NewServer(testOptions(t), nil)
	srv.tokenLoader = func() (*owa.Tokens, error) {
		return &owa.Tokens{
			Canary: "cached-canary",
			Bearer: bearerWithExp(t, farExpiry),
		}, nil
	}

	refreshCalls := int32(0)
	srv.tokenRefresher = func(_ context.Context) (*owa.Tokens, error) {
		atomic.AddInt32(&refreshCalls, 1)
		return &owa.Tokens{}, nil
	}

	saveCalls := int32(0)
	srv.tokenSaver = func(_ *owa.Tokens) error {
		atomic.AddInt32(&saveCalls, 1)
		return nil
	}

	if err := srv.refreshTokensIfNeeded(context.Background(), now); err != nil {
		t.Fatalf("refreshTokensIfNeeded() error: %v", err)
	}
	if got := atomic.LoadInt32(&refreshCalls); got != 0 {
		t.Fatalf("tokenRefresher calls = %d, want 0", got)
	}
	if got := atomic.LoadInt32(&saveCalls); got != 0 {
		t.Fatalf("tokenSaver calls = %d, want 0", got)
	}
}

func TestExecuteTaskSessionProbeTriggersAuthRecovery(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	runtimeDir := filepath.Dir(paths.RuntimePath())

	opts := testOptions(t)
	opts.StateDir = runtimeDir
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.AuthRecoveryTimeout = 200 * time.Millisecond

	var execCalls int32
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		atomic.AddInt32(&execCalls, 1)
		return ExecResult{ExitCode: 0, Stdout: "ok\n"}
	})

	srv.tokenLoader = func() (*owa.Tokens, error) {
		return &owa.Tokens{
			Canary: "cached-canary",
			Bearer: bearerWithExp(t, time.Now().Add(time.Hour)),
		}, nil
	}
	srv.tokenSaver = func(_ *owa.Tokens) error { return nil }
	srv.tokenRefresher = func(_ context.Context) (*owa.Tokens, error) {
		t.Fatal("tokenRefresher should not be called for healthy token")
		return nil, nil
	}
	srv.sessionProbe = func(_ context.Context) (bool, error) { return false, nil }

	srv.authProbe = func(_ context.Context) (bool, error) { return true, nil }
	srv.secureInputRunner = func(_ context.Context, _ string) error { return nil }
	notifyCalls := int32(0)
	srv.notifyAuth = func(_ context.Context, _ AuthNotification) error {
		atomic.AddInt32(&notifyCalls, 1)
		return nil
	}

	resp := srv.executeTask(context.Background(), queuedExec{
		req: Request{
			RequestID:   "session-probe",
			CommandPath: "mail search",
		},
		argv:       []string{"mail", "search", "inbox"},
		timeout:    time.Second,
		enqueuedAt: time.Now().UTC(),
		respCh:     make(chan Response, 1),
	})

	if !resp.OK {
		t.Fatalf("response not ok: code=%q stderr=%q", resp.ErrorCode, resp.Stderr)
	}
	if got := atomic.LoadInt32(&notifyCalls); got != 1 {
		t.Fatalf("notifyAuth calls = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&execCalls); got != 1 {
		t.Fatalf("exec calls = %d, want 1", got)
	}
}

func TestExecuteTaskSessionProbeUnavailableAttemptsBrowserRecovery(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	runtimeDir := filepath.Dir(paths.RuntimePath())

	opts := testOptions(t)
	opts.StateDir = runtimeDir
	opts.AuthProbeInterval = 5 * time.Millisecond
	opts.AuthRecoveryTimeout = 200 * time.Millisecond

	var (
		mu       sync.Mutex
		execArgs [][]string
	)
	srv := NewServer(opts, func(_ context.Context, argv []string, _ time.Duration) ExecResult {
		copied := append([]string{}, argv...)
		mu.Lock()
		execArgs = append(execArgs, copied)
		mu.Unlock()

		if len(argv) >= 2 && argv[0] == "browser" && argv[1] == "start" {
			return ExecResult{ExitCode: 0, Stdout: "browser started\n"}
		}
		return ExecResult{ExitCode: 0, Stdout: "ok\n"}
	})

	srv.tokenLoader = func() (*owa.Tokens, error) {
		return &owa.Tokens{
			Canary: "cached-canary",
			Bearer: bearerWithExp(t, time.Now().Add(time.Hour)),
		}, nil
	}
	srv.tokenSaver = func(_ *owa.Tokens) error { return nil }
	srv.tokenRefresher = func(_ context.Context) (*owa.Tokens, error) {
		t.Fatal("tokenRefresher should not be called for healthy token")
		return nil, nil
	}

	probeCalls := int32(0)
	srv.sessionProbe = func(_ context.Context) (bool, error) {
		if atomic.AddInt32(&probeCalls, 1) == 1 {
			return false, errSessionProbeUnavailable
		}
		return true, nil
	}

	notifyCalls := int32(0)
	srv.authProbe = func(_ context.Context) (bool, error) { return true, nil }
	srv.secureInputRunner = func(_ context.Context, _ string) error {
		t.Fatal("secureInputRunner should not be called when browser recovery succeeds")
		return nil
	}
	srv.notifyAuth = func(_ context.Context, _ AuthNotification) error {
		atomic.AddInt32(&notifyCalls, 1)
		return nil
	}

	resp := srv.executeTask(context.Background(), queuedExec{
		req: Request{
			RequestID:   "session-recover-browser",
			CommandPath: "mail search",
		},
		argv:       []string{"mail", "search", "inbox"},
		timeout:    time.Second,
		enqueuedAt: time.Now().UTC(),
		respCh:     make(chan Response, 1),
	})

	if !resp.OK {
		t.Fatalf("response not ok: code=%q stderr=%q", resp.ErrorCode, resp.Stderr)
	}
	if got := atomic.LoadInt32(&probeCalls); got != 2 {
		t.Fatalf("sessionProbe calls = %d, want 2", got)
	}
	if got := atomic.LoadInt32(&notifyCalls); got != 0 {
		t.Fatalf("notifyAuth calls = %d, want 0", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(execArgs) != 2 {
		t.Fatalf("exec calls = %d, want 2", len(execArgs))
	}
	if len(execArgs[0]) < 2 || execArgs[0][0] != "browser" || execArgs[0][1] != "start" {
		t.Fatalf("first exec argv = %v, want [browser start ...]", execArgs[0])
	}
	if len(execArgs[1]) < 2 || execArgs[1][0] != "mail" || execArgs[1][1] != "search" {
		t.Fatalf("second exec argv = %v, want [mail search ...]", execArgs[1])
	}
}

func bearerWithExp(t *testing.T, exp time.Time) string {
	t.Helper()

	headerRaw, err := json.Marshal(map[string]interface{}{
		"alg": "none",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadRaw, err := json.Marshal(map[string]interface{}{
		"exp": exp.Unix(),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerRaw)
	payload := base64.RawURLEncoding.EncodeToString(payloadRaw)
	return "Bearer " + header + "." + payload + ".signature"
}
