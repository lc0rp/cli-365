package daemon

import (
	"context"
	"testing"
	"time"
)

func TestServerQueueFullErrorCodeOverIPC(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}
	opts.MaxQueueSize = 1
	opts.RejectNewWhilePaused = false

	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		return ExecResult{ExitCode: 0, Stdout: "ok\n"}
	})
	srv.setPaused(true)

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
	firstDone := make(chan callResult, 1)
	go func() {
		resp, err := Call(opts.SocketPath, Request{
			RequestID:   "queue-full-1",
			Command:     CommandExec,
			CommandPath: "mail draft create",
			Argv:        []string{"mail", "draft", "create", "--subject", "a"},
		}, 2*time.Second)
		firstDone <- callResult{resp: resp, err: err}
	}()

	deadline := time.Now().Add(400 * time.Millisecond)
	for len(srv.execQ) != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(srv.execQ) != 1 {
		t.Fatalf("queue depth = %d, want 1", len(srv.execQ))
	}

	second, err := Call(opts.SocketPath, Request{
		RequestID:   "queue-full-2",
		Command:     CommandExec,
		CommandPath: "mail draft create",
		Argv:        []string{"mail", "draft", "create", "--subject", "b"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("second Call() error: %v", err)
	}
	if second.OK {
		t.Fatal("second response OK, want QUEUE_FULL")
	}
	if second.ErrorCode != ErrorCodeQueueFull {
		t.Fatalf("second ErrorCode = %q, want %q", second.ErrorCode, ErrorCodeQueueFull)
	}

	srv.setPaused(false)
	select {
	case result := <-firstDone:
		if result.err != nil {
			t.Fatalf("first Call() error: %v", result.err)
		}
		if !result.resp.OK {
			t.Fatalf("first response not OK: code=%q stderr=%q", result.resp.ErrorCode, result.resp.Stderr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first request response")
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

func TestServerDaemonUnavailableDrainsPendingOnStop(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}
	opts.RejectNewWhilePaused = false

	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		return ExecResult{ExitCode: 0}
	})
	srv.setPaused(true)

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
	pendingDone := make(chan callResult, 1)
	go func() {
		resp, err := Call(opts.SocketPath, Request{
			RequestID:   "pending-stop",
			Command:     CommandExec,
			CommandPath: "mail draft create",
			Argv:        []string{"mail", "draft", "create", "--subject", "a"},
		}, 3*time.Second)
		pendingDone <- callResult{resp: resp, err: err}
	}()

	deadline := time.Now().Add(400 * time.Millisecond)
	for len(srv.execQ) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(srv.execQ) == 0 {
		t.Fatal("request did not enqueue before stop")
	}

	if err := Stop(opts.SocketPath, 2*time.Second); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	select {
	case result := <-pendingDone:
		if result.err != nil {
			t.Fatalf("pending Call() error: %v", result.err)
		}
		if result.resp.OK {
			t.Fatal("pending response OK, want DAEMON_UNAVAILABLE")
		}
		if result.resp.ErrorCode != ErrorCodeDaemonUnavailable {
			t.Fatalf("pending ErrorCode = %q, want %q", result.resp.ErrorCode, ErrorCodeDaemonUnavailable)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for pending response")
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

func TestServerRequestTimeoutErrorCodeOverIPC(t *testing.T) {
	opts := testOptions(t)
	opts.Allowlist = []string{"mail"}

	srv := NewServer(opts, func(ctx context.Context, _ []string, _ time.Duration) ExecResult {
		<-ctx.Done()
		return ExecResult{
			ExitCode: 1,
			Err:      ctx.Err(),
		}
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()
	waitForPing(t, opts.SocketPath, 3*time.Second)

	resp, err := Call(opts.SocketPath, Request{
		RequestID:   "timeout-ipc",
		Command:     CommandExec,
		CommandPath: "mail search",
		Argv:        []string{"mail", "search", "invoice"},
		TimeoutMS:   50,
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}
	if resp.OK {
		t.Fatal("response OK, want timeout error")
	}
	if resp.ErrorCode != ErrorCodeRequestTimeout {
		t.Fatalf("ErrorCode = %q, want %q", resp.ErrorCode, ErrorCodeRequestTimeout)
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
