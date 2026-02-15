package daemon

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lc0rp/cli-365/internal/paths"
)

func TestRunPrimaryMaintenanceUsesHook(t *testing.T) {
	srv := NewServer(testOptions(t), nil)

	var calls int32
	srv.maintainPrimaryFn = func() {
		atomic.AddInt32(&calls, 1)
	}

	srv.runPrimaryMaintenance()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("runPrimaryMaintenance calls = %d, want 1", got)
	}
}

func TestRunSessionMaintenanceTicksWhenManagedStateDir(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	runtimeDir := filepath.Dir(paths.RuntimePath())

	opts := testOptions(t)
	opts.StateDir = runtimeDir

	srv := NewServer(opts, nil)
	srv.maintenanceInterval = 5 * time.Millisecond

	done := make(chan struct{})
	var calls int32
	srv.maintainPrimaryFn = func() {
		if atomic.AddInt32(&calls, 1) == 2 {
			close(done)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.runSessionMaintenance(ctx)

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for session maintenance ticks")
	}

	if got := atomic.LoadInt32(&calls); got < 2 {
		t.Fatalf("maintenance calls = %d, want >= 2", got)
	}
}
