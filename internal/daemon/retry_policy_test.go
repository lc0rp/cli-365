package daemon

import (
	"context"
	"testing"
	"time"
)

func TestExecuteTaskRetriesTransientReadFailures(t *testing.T) {
	opts := testOptions(t)
	opts.AuthRecoveryTimeout = time.Second

	var calls int
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		calls++
		if calls < 3 {
			return ExecResult{
				ExitCode: 1,
				Stderr:   "search failed with status 503: service unavailable",
			}
		}
		return ExecResult{
			ExitCode: 0,
			Stdout:   "ok\n",
		}
	})

	resp := srv.executeTask(context.Background(), queuedExec{
		req: Request{
			RequestID:   "retry-read",
			CommandPath: "mail search",
		},
		argv:       []string{"mail", "search", "invoice"},
		timeout:    2 * time.Second,
		enqueuedAt: time.Now().UTC(),
		respCh:     make(chan Response, 1),
	})

	if !resp.OK {
		t.Fatalf("response not ok: code=%q stderr=%q", resp.ErrorCode, resp.Stderr)
	}
	if calls != 3 {
		t.Fatalf("exec calls = %d, want 3", calls)
	}
}

func TestExecuteTaskDoesNotRetryWriteFailures(t *testing.T) {
	opts := testOptions(t)

	var calls int
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		calls++
		return ExecResult{
			ExitCode: 1,
			Stderr:   "send failed with status 503: service unavailable",
		}
	})

	resp := srv.executeTask(context.Background(), queuedExec{
		req: Request{
			RequestID:   "retry-write",
			CommandPath: "mail send",
		},
		argv:       []string{"mail", "send", "--to", "a@example.com", "--subject", "s"},
		timeout:    2 * time.Second,
		enqueuedAt: time.Now().UTC(),
		respCh:     make(chan Response, 1),
	})

	if resp.OK {
		t.Fatal("response ok, want failure")
	}
	if calls != 1 {
		t.Fatalf("exec calls = %d, want 1", calls)
	}
}

func TestExecuteTaskStopsAfterReadRetryBudget(t *testing.T) {
	opts := testOptions(t)

	var calls int
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		calls++
		return ExecResult{
			ExitCode: 1,
			Stderr:   "search failed with status 503: service unavailable",
		}
	})

	resp := srv.executeTask(context.Background(), queuedExec{
		req: Request{
			RequestID:   "retry-budget",
			CommandPath: "mail search",
		},
		argv:       []string{"mail", "search", "invoice"},
		timeout:    3 * time.Second,
		enqueuedAt: time.Now().UTC(),
		respCh:     make(chan Response, 1),
	})

	if resp.OK {
		t.Fatal("response ok, want failure")
	}
	if calls != maxReadRetries+1 {
		t.Fatalf("exec calls = %d, want %d", calls, maxReadRetries+1)
	}
}
