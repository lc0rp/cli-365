package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoveStaleSocketPathRejectsDirectory(t *testing.T) {
	socketDir := filepath.Join(t.TempDir(), "daemon.sock")
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		t.Fatalf("mkdir socket dir: %v", err)
	}

	err := removeStaleSocketPath(socketDir)
	if err == nil {
		t.Fatal("removeStaleSocketPath() error = nil, want rejection for directory")
	}
	if _, statErr := os.Stat(socketDir); statErr != nil {
		t.Fatalf("socket dir should remain after rejection: %v", statErr)
	}
}

func TestRemoveStaleSocketPathRemovesRegularFile(t *testing.T) {
	socketFile := filepath.Join(t.TempDir(), "daemon.sock")
	if err := os.WriteFile(socketFile, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale socket file: %v", err)
	}

	if err := removeStaleSocketPath(socketFile); err != nil {
		t.Fatalf("removeStaleSocketPath() error: %v", err)
	}
	if _, err := os.Stat(socketFile); !os.IsNotExist(err) {
		t.Fatalf("socket file should be removed, stat err = %v", err)
	}
}

func TestRemoveStaleSocketPathRejectsSymlink(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}

	socketPath := filepath.Join(tmp, "daemon.sock")
	if err := os.Symlink(target, socketPath); err != nil {
		t.Fatalf("create socket symlink: %v", err)
	}

	err := removeStaleSocketPath(socketPath)
	if err == nil {
		t.Fatal("removeStaleSocketPath() error = nil, want rejection for symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("removeStaleSocketPath() error = %q, want symlink rejection message", err.Error())
	}
	if _, statErr := os.Lstat(socketPath); statErr != nil {
		t.Fatalf("socket symlink should remain after rejection: %v", statErr)
	}
}

func TestCheckWriteRateLimitsPrunesStaleRecipientEntries(t *testing.T) {
	opts := testOptions(t)
	opts.WriteRateLimitPerMinute = 10
	opts.RecipientWriteRateLimitPerMinute = 1

	srv := NewServer(opts, nil)
	srv.recipientWriteSeen["stale@example.com"] = []time.Time{time.Now().Add(-2 * time.Minute)}

	code, msg := srv.checkWriteRateLimits("mail send", []string{"mail", "send", "--to", "fresh@example.com"})
	if code != "" || msg != "" {
		t.Fatalf("checkWriteRateLimits() = (%q, %q), want success", code, msg)
	}
	if _, ok := srv.recipientWriteSeen["stale@example.com"]; ok {
		t.Fatal("stale recipient entry should be pruned")
	}
	if len(srv.recipientWriteSeen["fresh@example.com"]) != 1 {
		t.Fatalf("fresh recipient events = %d, want 1", len(srv.recipientWriteSeen["fresh@example.com"]))
	}
}

func TestCheckWriteRateLimitsThrottleAfterPrunedRecipientHistory(t *testing.T) {
	opts := testOptions(t)
	opts.WriteRateLimitPerMinute = 10
	opts.RecipientWriteRateLimitPerMinute = 1

	srv := NewServer(opts, nil)
	srv.recipientWriteSeen["fresh@example.com"] = []time.Time{time.Now().Add(-2 * time.Minute)}

	code, msg := srv.checkWriteRateLimits("mail send", []string{"mail", "send", "--to", "fresh@example.com"})
	if code != "" || msg != "" {
		t.Fatalf("first checkWriteRateLimits() = (%q, %q), want success", code, msg)
	}

	code, msg = srv.checkWriteRateLimits("mail send", []string{"mail", "send", "--to", "fresh@example.com"})
	if code != ErrorCodeWriteThrottled {
		t.Fatalf("second checkWriteRateLimits() code = %q, want %q", code, ErrorCodeWriteThrottled)
	}
	if !strings.Contains(strings.ToLower(msg), "recipient") {
		t.Fatalf("second checkWriteRateLimits() msg = %q, want recipient throttle detail", msg)
	}
}
