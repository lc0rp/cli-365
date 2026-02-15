package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func testOptions(t *testing.T) Options {
	t.Helper()
	base := filepath.Join(t.TempDir(), "daemon")
	return Options{
		StateDir:               base,
		SocketPath:             filepath.Join(base, "daemon.sock"),
		LockPath:               filepath.Join(base, "daemon.lock"),
		StatusPath:             filepath.Join(base, "daemon.json"),
		DefaultCommandTimeout:  2 * time.Second,
		MaxQueueSize:           8,
		RejectNewWhilePaused:   true,
		CoalesceIdenticalReads: true,
	}
}

func waitForPing(t *testing.T, socketPath string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := Ping(socketPath, 200*time.Millisecond); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("daemon did not respond to ping within %s", timeout)
}

func TestServerRunPingExecStop(t *testing.T) {
	opts := testOptions(t)

	var gotArgv []string
	execFn := func(_ context.Context, argv []string, _ time.Duration) ExecResult {
		gotArgv = append([]string{}, argv...)
		return ExecResult{
			Stdout:   "ok\n",
			ExitCode: 0,
		}
	}

	srv := NewServer(opts, execFn)
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()

	waitForPing(t, opts.SocketPath, 3*time.Second)

	resp, err := Call(opts.SocketPath, Request{
		RequestID: "req-1",
		Command:   CommandExec,
		Argv:      []string{"auth", "status"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}
	if !resp.OK {
		t.Fatalf("resp.OK = false, stderr=%q, code=%q", resp.Stderr, resp.ErrorCode)
	}
	if resp.ExitCode != 0 {
		t.Fatalf("resp.ExitCode = %d, want 0", resp.ExitCode)
	}
	if resp.Stdout != "ok\n" {
		t.Fatalf("resp.Stdout = %q, want %q", resp.Stdout, "ok\n")
	}
	if len(gotArgv) != 2 || gotArgv[0] != "auth" || gotArgv[1] != "status" {
		t.Fatalf("exec argv = %v, want [auth status]", gotArgv)
	}

	status, err := ReadStatus(opts.StatusPath)
	if err != nil {
		t.Fatalf("ReadStatus() error: %v", err)
	}
	if !status.Running {
		t.Fatalf("status.Running = false, want true")
	}
	if status.SocketPath != opts.SocketPath {
		t.Fatalf("status.SocketPath = %q, want %q", status.SocketPath, opts.SocketPath)
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

	stopped, err := ReadStatus(opts.StatusPath)
	if err != nil {
		t.Fatalf("ReadStatus() after stop error: %v", err)
	}
	if stopped.Running {
		t.Fatalf("status.Running = true after stop")
	}
}

func TestServerSingleInstanceLock(t *testing.T) {
	opts := testOptions(t)
	srv1 := NewServer(opts, nil)

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv1.Run(runCtx)
	}()

	waitForPing(t, opts.SocketPath, 3*time.Second)

	srv2 := NewServer(opts, nil)
	err := srv2.Run(context.Background())
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("Run() second instance error = %v, want ErrAlreadyRunning", err)
	}

	if err := Stop(opts.SocketPath, 2*time.Second); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run() first instance error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for first server shutdown")
	}
}

func TestServerPermissions(t *testing.T) {
	opts := testOptions(t)
	srv := NewServer(opts, nil)

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()

	waitForPing(t, opts.SocketPath, 3*time.Second)

	stateInfo, err := os.Stat(opts.StateDir)
	if err != nil {
		t.Fatalf("stat state dir: %v", err)
	}
	if got := stateInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("state dir mode = %o, want 700", got)
	}

	for _, path := range []string{opts.LockPath, opts.StatusPath, opts.SocketPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode = %o, want 600", path, got)
		}
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

func TestServerCDPPortMismatch(t *testing.T) {
	opts := testOptions(t)
	opts.CDPPort = 9222

	execCalled := false
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		execCalled = true
		return ExecResult{ExitCode: 0}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()

	waitForPing(t, opts.SocketPath, 3*time.Second)

	resp, err := Call(opts.SocketPath, Request{
		RequestID: "cdp-mismatch",
		Command:   CommandExec,
		Argv:      []string{"auth", "status"},
		CDPPort:   9333,
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}
	if resp.OK {
		t.Fatal("resp.OK = true, want false")
	}
	if resp.ErrorCode != ErrorCodeCDPPortMismatch {
		t.Fatalf("resp.ErrorCode = %q, want %q", resp.ErrorCode, ErrorCodeCDPPortMismatch)
	}
	if !strings.Contains(resp.Stderr, "requested=9333") {
		t.Fatalf("resp.Stderr = %q, want requested value", resp.Stderr)
	}
	if execCalled {
		t.Fatal("execFn called on cdp mismatch")
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

func TestServerCoalescesReadRequests(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"auth"}
	opts.CoalesceIdenticalReads = true

	var (
		mu        sync.Mutex
		execCalls int
	)
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		mu.Lock()
		execCalls++
		mu.Unlock()
		time.Sleep(150 * time.Millisecond)
		return ExecResult{
			ExitCode: 0,
			Stdout:   "ok\n",
		}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()
	waitForPing(t, opts.SocketPath, 3*time.Second)

	type callResult struct {
		resp Response
		err  error
	}
	results := make(chan callResult, 2)

	go func() {
		resp, err := Call(opts.SocketPath, Request{
			RequestID:   "read-1",
			Command:     CommandExec,
			CommandPath: "auth status",
			Argv:        []string{"auth", "status"},
		}, 2*time.Second)
		results <- callResult{resp: resp, err: err}
	}()
	time.Sleep(20 * time.Millisecond)
	go func() {
		resp, err := Call(opts.SocketPath, Request{
			RequestID:   "read-2",
			Command:     CommandExec,
			CommandPath: "auth status",
			Argv:        []string{"auth", "status"},
		}, 2*time.Second)
		results <- callResult{resp: resp, err: err}
	}()

	for i := 0; i < 2; i++ {
		res := <-results
		if res.err != nil {
			t.Fatalf("Call() error: %v", res.err)
		}
		if !res.resp.OK || res.resp.ExitCode != 0 {
			t.Fatalf("response not ok: code=%d err=%q", res.resp.ExitCode, res.resp.ErrorCode)
		}
		if res.resp.Stdout != "ok\n" {
			t.Fatalf("stdout = %q, want ok\\n", res.resp.Stdout)
		}
	}

	mu.Lock()
	gotCalls := execCalls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatalf("execCalls = %d, want 1 (coalesced)", gotCalls)
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

func TestServerDoesNotCoalesceWriteRequests(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}
	opts.CoalesceIdenticalReads = true

	var (
		mu        sync.Mutex
		execCalls int
	)
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		mu.Lock()
		execCalls++
		mu.Unlock()
		time.Sleep(75 * time.Millisecond)
		return ExecResult{ExitCode: 0}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()
	waitForPing(t, opts.SocketPath, 3*time.Second)

	type callResult struct {
		resp Response
		err  error
	}
	results := make(chan callResult, 2)
	for i := 0; i < 2; i++ {
		id := i
		go func() {
			resp, err := Call(opts.SocketPath, Request{
				RequestID:   "write-" + strconv.Itoa(id),
				Command:     CommandExec,
				CommandPath: "mail draft create",
				Argv:        []string{"mail", "draft", "create", "--to", "a@example.com", "--subject", "s", "--body", "b"},
			}, 2*time.Second)
			results <- callResult{resp: resp, err: err}
		}()
	}

	for i := 0; i < 2; i++ {
		res := <-results
		if res.err != nil {
			t.Fatalf("Call() error: %v", res.err)
		}
		if !res.resp.OK {
			t.Fatalf("response not ok: code=%q stderr=%q", res.resp.ErrorCode, res.resp.Stderr)
		}
	}

	mu.Lock()
	gotCalls := execCalls
	mu.Unlock()
	if gotCalls != 2 {
		t.Fatalf("execCalls = %d, want 2 (writes not coalesced)", gotCalls)
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

func TestServerDuplicateWriteSuppression(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}
	opts.DuplicateWriteWindowMail = time.Hour

	var (
		mu        sync.Mutex
		execCalls int
	)
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		mu.Lock()
		execCalls++
		mu.Unlock()
		return ExecResult{ExitCode: 0}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()
	waitForPing(t, opts.SocketPath, 3*time.Second)

	args := []string{"mail", "send", "--to", "a@example.com", "--subject", "s", "--body", "b"}
	first, err := Call(opts.SocketPath, Request{
		RequestID:   "dup-1",
		Command:     CommandExec,
		CommandPath: "mail send",
		Argv:        args,
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("first Call() error: %v", err)
	}
	if !first.OK {
		t.Fatalf("first response not ok: code=%q stderr=%q", first.ErrorCode, first.Stderr)
	}

	second, err := Call(opts.SocketPath, Request{
		RequestID:   "dup-2",
		Command:     CommandExec,
		CommandPath: "mail send",
		Argv:        args,
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("second Call() error: %v", err)
	}
	if second.OK {
		t.Fatal("second response OK, want duplicate suppression error")
	}
	if second.ErrorCode != ErrorCodeDuplicateWriteSuspected {
		t.Fatalf("second ErrorCode = %q, want %q", second.ErrorCode, ErrorCodeDuplicateWriteSuspected)
	}

	mu.Lock()
	gotCalls := execCalls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatalf("execCalls = %d, want 1", gotCalls)
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

func TestServerDuplicateWriteOverrideFlag(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}
	opts.DuplicateWriteWindowMail = time.Hour

	var (
		mu        sync.Mutex
		execCalls int
	)
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		mu.Lock()
		execCalls++
		mu.Unlock()
		return ExecResult{ExitCode: 0}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()
	waitForPing(t, opts.SocketPath, 3*time.Second)

	baseArgs := []string{"mail", "send", "--to", "a@example.com", "--subject", "s", "--body", "b"}
	first, err := Call(opts.SocketPath, Request{
		RequestID:   "override-1",
		Command:     CommandExec,
		CommandPath: "mail send",
		Argv:        baseArgs,
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("first Call() error: %v", err)
	}
	if !first.OK {
		t.Fatalf("first response not ok: code=%q stderr=%q", first.ErrorCode, first.Stderr)
	}

	overrideArgs := append([]string{}, baseArgs...)
	overrideArgs = append(overrideArgs, "--allow-duplicate-write")
	second, err := Call(opts.SocketPath, Request{
		RequestID:   "override-2",
		Command:     CommandExec,
		CommandPath: "mail send",
		Argv:        overrideArgs,
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("second Call() error: %v", err)
	}
	if !second.OK {
		t.Fatalf("second response not ok: code=%q stderr=%q", second.ErrorCode, second.Stderr)
	}

	mu.Lock()
	gotCalls := execCalls
	mu.Unlock()
	if gotCalls != 2 {
		t.Fatalf("execCalls = %d, want 2", gotCalls)
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

func TestServerGlobalWriteRateLimit(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}
	opts.WriteRateLimitPerMinute = 1
	opts.RecipientWriteRateLimitPerMinute = 0

	var (
		mu        sync.Mutex
		execCalls int
	)
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		mu.Lock()
		execCalls++
		mu.Unlock()
		return ExecResult{ExitCode: 0}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()
	waitForPing(t, opts.SocketPath, 3*time.Second)

	first, err := Call(opts.SocketPath, Request{
		RequestID:   "rate-global-1",
		Command:     CommandExec,
		CommandPath: "mail draft create",
		Argv:        []string{"mail", "draft", "create", "--subject", "a"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("first Call() error: %v", err)
	}
	if !first.OK {
		t.Fatalf("first response not ok: code=%q stderr=%q", first.ErrorCode, first.Stderr)
	}

	second, err := Call(opts.SocketPath, Request{
		RequestID:   "rate-global-2",
		Command:     CommandExec,
		CommandPath: "mail draft create",
		Argv:        []string{"mail", "draft", "create", "--subject", "b"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("second Call() error: %v", err)
	}
	if second.OK {
		t.Fatal("second response OK, want write throttle error")
	}
	if second.ErrorCode != ErrorCodeWriteThrottled {
		t.Fatalf("second ErrorCode = %q, want %q", second.ErrorCode, ErrorCodeWriteThrottled)
	}

	mu.Lock()
	gotCalls := execCalls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatalf("execCalls = %d, want 1", gotCalls)
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

func TestServerRecipientWriteRateLimit(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}
	opts.WriteRateLimitPerMinute = 10
	opts.RecipientWriteRateLimitPerMinute = 1
	opts.DuplicateWriteWindowMail = time.Millisecond

	var (
		mu        sync.Mutex
		execCalls int
	)
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		mu.Lock()
		execCalls++
		mu.Unlock()
		return ExecResult{ExitCode: 0}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()
	waitForPing(t, opts.SocketPath, 3*time.Second)

	first, err := Call(opts.SocketPath, Request{
		RequestID:   "rate-recipient-1",
		Command:     CommandExec,
		CommandPath: "mail send",
		Argv:        []string{"mail", "send", "--to", "a@example.com", "--subject", "a"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("first Call() error: %v", err)
	}
	if !first.OK {
		t.Fatalf("first response not ok: code=%q stderr=%q", first.ErrorCode, first.Stderr)
	}

	second, err := Call(opts.SocketPath, Request{
		RequestID:   "rate-recipient-2",
		Command:     CommandExec,
		CommandPath: "mail send",
		Argv:        []string{"mail", "send", "--to", "a@example.com", "--subject", "b"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("second Call() error: %v", err)
	}
	if second.OK {
		t.Fatal("second response OK, want recipient write throttle error")
	}
	if second.ErrorCode != ErrorCodeWriteThrottled {
		t.Fatalf("second ErrorCode = %q, want %q", second.ErrorCode, ErrorCodeWriteThrottled)
	}

	mu.Lock()
	gotCalls := execCalls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatalf("execCalls = %d, want 1", gotCalls)
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

func TestServerRejectsOversizedRequest(t *testing.T) {
	opts := testOptions(t)
	opts.MaxRequestBytes = 128

	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		return ExecResult{ExitCode: 0}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()

	waitForPing(t, opts.SocketPath, 3*time.Second)

	resp, err := Call(opts.SocketPath, Request{
		RequestID: "too-big",
		Command:   CommandExec,
		Argv:      []string{strings.Repeat("x", 1024)},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}
	if resp.ErrorCode != ErrorCodeInvalidRequest {
		t.Fatalf("resp.ErrorCode = %q, want %q", resp.ErrorCode, ErrorCodeInvalidRequest)
	}
	if resp.OK {
		t.Fatal("resp.OK = true, want false")
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

func TestServerRejectsDisallowedCommandPath(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}

	execCalled := false
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		execCalled = true
		return ExecResult{ExitCode: 0}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()

	waitForPing(t, opts.SocketPath, 3*time.Second)

	resp, err := Call(opts.SocketPath, Request{
		RequestID:   "disallowed-path",
		Command:     CommandExec,
		CommandPath: "browser status",
		Argv:        []string{"browser", "status"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}
	if resp.OK {
		t.Fatal("resp.OK = true, want false")
	}
	if resp.ErrorCode != ErrorCodeInvalidRequest {
		t.Fatalf("resp.ErrorCode = %q, want %q", resp.ErrorCode, ErrorCodeInvalidRequest)
	}
	if !strings.Contains(resp.Stderr, "allowlist") {
		t.Fatalf("resp.Stderr = %q, want allowlist detail", resp.Stderr)
	}
	if execCalled {
		t.Fatal("execFn called for disallowed command")
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

func TestServerRejectsReadonlyWriteCommand(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}
	opts.Readonly = true

	execCalled := false
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		execCalled = true
		return ExecResult{ExitCode: 0}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()

	waitForPing(t, opts.SocketPath, 3*time.Second)

	resp, err := Call(opts.SocketPath, Request{
		RequestID:   "readonly-write",
		Command:     CommandExec,
		CommandPath: "mail send",
		Argv:        []string{"mail", "send", "--to", "a@example.com", "--subject", "x"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}
	if resp.OK {
		t.Fatal("resp.OK = true, want false")
	}
	if resp.ErrorCode != ErrorCodeInvalidRequest {
		t.Fatalf("resp.ErrorCode = %q, want %q", resp.ErrorCode, ErrorCodeInvalidRequest)
	}
	if !strings.Contains(resp.Stderr, "readonly") {
		t.Fatalf("resp.Stderr = %q, want readonly detail", resp.Stderr)
	}
	if execCalled {
		t.Fatal("execFn called for readonly write command")
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
