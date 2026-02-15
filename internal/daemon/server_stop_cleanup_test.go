package daemon

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lc0rp/cli-365/internal/paths"
)

func TestServerRunInvokesStopCleanupHook(t *testing.T) {
	opts := testOptions(t)
	srv := NewServer(opts, func(_ context.Context, _ []string, _ time.Duration) ExecResult {
		return ExecResult{ExitCode: 0}
	})

	var calls int32
	srv.stopCleanup = func() error {
		atomic.AddInt32(&calls, 1)
		return nil
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(runCtx)
	}()

	waitForPing(t, opts.SocketPath, 3*time.Second)

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

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("stopCleanup calls = %d, want 1", got)
	}
}

func TestServerShouldCleanupManagedBrowser(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	runtimeDir := filepath.Dir(paths.RuntimePath())

	opts := testOptions(t)
	opts.StateDir = runtimeDir

	srv := NewServer(opts, nil)
	if !srv.shouldCleanupManagedBrowser() {
		t.Fatalf("shouldCleanupManagedBrowser() = false, want true for daemon runtime dir %q", runtimeDir)
	}

	opts.StateDir = filepath.Join(t.TempDir(), "daemon")
	srv = NewServer(opts, nil)
	if srv.shouldCleanupManagedBrowser() {
		t.Fatalf("shouldCleanupManagedBrowser() = true, want false for non-runtime state dir %q", opts.StateDir)
	}
}
