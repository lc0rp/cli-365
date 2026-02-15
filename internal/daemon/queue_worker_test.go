package daemon

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTryEnqueueQueueFull(t *testing.T) {
	opts := testOptions(t)
	opts.MaxQueueSize = 1
	srv := NewServer(opts, nil)

	if code := srv.tryEnqueue(queuedExec{
		req:    Request{RequestID: "one"},
		respCh: make(chan Response, 1),
	}); code != "" {
		t.Fatalf("tryEnqueue first code = %q, want empty", code)
	}

	if code := srv.tryEnqueue(queuedExec{
		req:    Request{RequestID: "two"},
		respCh: make(chan Response, 1),
	}); code != ErrorCodeQueueFull {
		t.Fatalf("tryEnqueue second code = %q, want %q", code, ErrorCodeQueueFull)
	}
}

func TestTryEnqueueAuthPaused(t *testing.T) {
	opts := testOptions(t)
	srv := NewServer(opts, nil)
	srv.setPaused(true)

	if code := srv.tryEnqueue(queuedExec{
		req:    Request{RequestID: "paused"},
		respCh: make(chan Response, 1),
	}); code != ErrorCodeAuthPaused {
		t.Fatalf("tryEnqueue code = %q, want %q", code, ErrorCodeAuthPaused)
	}
}

func TestRunWorkerFIFO(t *testing.T) {
	opts := testOptions(t)
	opts.MaxQueueSize = 4

	var (
		mu    sync.Mutex
		order []string
	)
	execFn := func(_ context.Context, argv []string, _ time.Duration) ExecResult {
		mu.Lock()
		order = append(order, argv[0])
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		return ExecResult{ExitCode: 0}
	}

	srv := NewServer(opts, execFn)
	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		srv.runWorker(ctx)
	}()

	tasks := []queuedExec{
		{
			req:        Request{RequestID: "one"},
			argv:       []string{"one"},
			timeout:    time.Second,
			enqueuedAt: time.Now().UTC(),
			respCh:     make(chan Response, 1),
		},
		{
			req:        Request{RequestID: "two"},
			argv:       []string{"two"},
			timeout:    time.Second,
			enqueuedAt: time.Now().UTC(),
			respCh:     make(chan Response, 1),
		},
		{
			req:        Request{RequestID: "three"},
			argv:       []string{"three"},
			timeout:    time.Second,
			enqueuedAt: time.Now().UTC(),
			respCh:     make(chan Response, 1),
		},
	}

	for i := range tasks {
		if code := srv.tryEnqueue(tasks[i]); code != "" {
			t.Fatalf("tryEnqueue[%d] code = %q, want empty", i, code)
		}
	}

	for i := range tasks {
		select {
		case resp := <-tasks[i].respCh:
			if !resp.OK {
				t.Fatalf("resp[%d] not OK: code=%q stderr=%q", i, resp.ErrorCode, resp.Stderr)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for resp[%d]", i)
		}
	}

	cancel()
	select {
	case <-workerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker shutdown")
	}

	if !reflect.DeepEqual(order, []string{"one", "two", "three"}) {
		t.Fatalf("worker order = %v, want [one two three]", order)
	}
}

func TestDrainPendingFailsQueuedRequests(t *testing.T) {
	opts := testOptions(t)
	srv := NewServer(opts, nil)

	tasks := []queuedExec{
		{
			req:        Request{RequestID: "one"},
			enqueuedAt: time.Now().UTC(),
			respCh:     make(chan Response, 1),
		},
		{
			req:        Request{RequestID: "two"},
			enqueuedAt: time.Now().UTC(),
			respCh:     make(chan Response, 1),
		},
	}

	for i := range tasks {
		if code := srv.tryEnqueue(tasks[i]); code != "" {
			t.Fatalf("tryEnqueue[%d] code = %q, want empty", i, code)
		}
	}

	srv.drainPending(ErrorCodeAuthTimeout, "auth recovery timeout")

	for i := range tasks {
		select {
		case resp := <-tasks[i].respCh:
			if resp.ErrorCode != ErrorCodeAuthTimeout {
				t.Fatalf("resp[%d].ErrorCode = %q, want %q", i, resp.ErrorCode, ErrorCodeAuthTimeout)
			}
			if resp.RequestID != tasks[i].req.RequestID {
				t.Fatalf("resp[%d].RequestID = %q, want %q", i, resp.RequestID, tasks[i].req.RequestID)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for drained resp[%d]", i)
		}
	}
}

func TestRunWorkerPauseResume(t *testing.T) {
	opts := testOptions(t)
	opts.RejectNewWhilePaused = false

	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		return ExecResult{ExitCode: 0}
	})
	srv.setPaused(true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		srv.runWorker(ctx)
	}()

	task := queuedExec{
		req:        Request{RequestID: "pause-resume"},
		argv:       []string{"auth", "status"},
		timeout:    time.Second,
		enqueuedAt: time.Now().UTC(),
		respCh:     make(chan Response, 1),
	}
	if code := srv.tryEnqueue(task); code != "" {
		t.Fatalf("tryEnqueue code = %q, want empty", code)
	}

	select {
	case <-task.respCh:
		t.Fatal("worker executed task while paused")
	case <-time.After(150 * time.Millisecond):
	}

	srv.setPaused(false)

	select {
	case resp := <-task.respCh:
		if !resp.OK {
			t.Fatalf("resp.OK = false, code=%q stderr=%q", resp.ErrorCode, resp.Stderr)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for resumed task execution")
	}

	cancel()
	select {
	case <-workerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker shutdown")
	}
}

func TestExecuteTaskTimeoutCode(t *testing.T) {
	opts := testOptions(t)
	srv := NewServer(opts, func(ctx context.Context, _ []string, _ time.Duration) ExecResult {
		<-ctx.Done()
		return ExecResult{
			ExitCode: 1,
			Stderr:   ctx.Err().Error(),
			Err:      ctx.Err(),
		}
	})

	resp := srv.executeTask(context.Background(), queuedExec{
		req:        Request{RequestID: "timeout"},
		argv:       []string{"auth", "status"},
		timeout:    30 * time.Millisecond,
		enqueuedAt: time.Now().Add(-10 * time.Millisecond).UTC(),
		respCh:     make(chan Response, 1),
	})

	if resp.ErrorCode != ErrorCodeRequestTimeout {
		t.Fatalf("ErrorCode = %q, want %q", resp.ErrorCode, ErrorCodeRequestTimeout)
	}
	if resp.ExecMS <= 0 {
		t.Fatalf("ExecMS = %d, want > 0", resp.ExecMS)
	}
}

func TestServerPanicGuard(t *testing.T) {
	opts := testOptions(t)
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		panic("boom")
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()

	waitForPing(t, opts.SocketPath, 3*time.Second)

	resp, err := Call(opts.SocketPath, Request{
		RequestID: "panic",
		Command:   CommandExec,
		Argv:      []string{"auth", "status"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}
	if resp.OK {
		t.Fatal("resp.OK = true, want false")
	}
	if resp.ErrorCode != ErrorCodeExecFailed {
		t.Fatalf("resp.ErrorCode = %q, want %q", resp.ErrorCode, ErrorCodeExecFailed)
	}
	if !strings.Contains(resp.Stderr, "panic") {
		t.Fatalf("resp.Stderr = %q, want panic detail", resp.Stderr)
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
