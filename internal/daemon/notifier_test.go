package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultNotifyAuthInvokesOpenClawCLI(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	scriptPath := filepath.Join(dir, "fake-openclaw.sh")
	script := "#!/usr/bin/env bash\nout=\"$1\"; shift; printf '%s\\n' \"$@\" > \"$out\"\n"
	// shell wrapper form: "bash <script> <argsFile>"
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}

	notifier := defaultNotifyAuth(
		"openclaw-cli",
		"bash "+scriptPath+" "+argsFile,
		"discord",
		"ops-room",
	)

	err := notifier(context.Background(), AuthNotification{
		Severity:   "warning",
		Reason:     "auth_required",
		LoginURL:   "https://outlook.office.com/mail/",
		QueueDepth: 7,
		At:         time.Date(2026, 2, 15, 16, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("notify error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 7 {
		t.Fatalf("expected args lines, got %v", lines)
	}
	wantPrefix := []string{"message", "send", "--channel", "discord", "--target", "ops-room"}
	for i, want := range wantPrefix {
		if lines[i] != want {
			t.Fatalf("arg[%d] = %q, want %q", i, lines[i], want)
		}
	}
	msg := lines[len(lines)-1]
	for _, needle := range []string{
		"service=cli-365",
		"severity=warning",
		"reason=auth_required",
		"login_url=https://outlook.office.com/mail/",
		"queue_depth=7",
	} {
		if !strings.Contains(msg, needle) {
			t.Fatalf("message missing %q: %q", needle, msg)
		}
	}
}

func TestDefaultNotifyAuthNoopForUnsupportedProvider(t *testing.T) {
	notifier := defaultNotifyAuth("noop", "openclaw", "discord", "ops")
	if err := notifier(context.Background(), AuthNotification{
		Severity: "error",
		Reason:   "auth_timeout",
	}); err != nil {
		t.Fatalf("unexpected error for unsupported provider: %v", err)
	}
}
