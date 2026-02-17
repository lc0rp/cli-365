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
	if len(lines) < 8 {
		t.Fatalf("expected args lines, got %v", lines)
	}
	wantPrefix := []string{"message", "send", "--channel", "discord", "--target", "ops-room", "--message"}
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
		"secure_input_url=",
		"secure_input_expires_at=",
		"secure_input_expires_in=",
		"queue_depth=7",
	} {
		if !strings.Contains(msg, needle) {
			t.Fatalf("message missing %q: %q", needle, msg)
		}
	}
}

func TestDefaultNotifyAuthIncludesSecureInputExpiry(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	scriptPath := filepath.Join(dir, "fake-openclaw.sh")
	script := "#!/usr/bin/env bash\nout=\"$1\"; shift; printf '%s\\n' \"$@\" > \"$out\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}

	notifier := defaultNotifyAuth(
		"openclaw-cli",
		"bash "+scriptPath+" "+argsFile,
		"discord",
		"ops-room",
	)

	at := time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC)
	expiresAt := at.Add(5 * time.Minute)
	err := notifier(context.Background(), AuthNotification{
		Severity:             "warning",
		Reason:               "secure_input_url",
		LoginURL:             "https://outlook.office.com/mail/",
		SecureInputURL:       "https://100.64.0.1:38000/token",
		SecureInputExpiresAt: expiresAt,
		QueueDepth:           2,
		At:                   at,
	})
	if err != nil {
		t.Fatalf("notify error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	msg := lines[len(lines)-1]
	for _, needle := range []string{
		"reason=secure_input_url",
		"secure_input_url=https://100.64.0.1:38000/token",
		"secure_input_expires_at=2026-02-16T10:05:00Z",
		"secure_input_expires_in=5m0s",
	} {
		if !strings.Contains(msg, needle) {
			t.Fatalf("message missing %q: %q", needle, msg)
		}
	}
}

func TestDefaultNotifyAuthIncludesDetailWhenProvided(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	scriptPath := filepath.Join(dir, "fake-openclaw.sh")
	script := "#!/usr/bin/env bash\nout=\"$1\"; shift; printf '%s\\n' \"$@\" > \"$out\"\n"
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
		Reason:     "mfa_number_challenge",
		LoginURL:   "https://outlook.office.com/mail/",
		QueueDepth: 1,
		Detail:     "authenticator number: 42",
		At:         time.Date(2026, 2, 17, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("notify error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	msg := lines[len(lines)-1]
	if !strings.Contains(msg, "detail=authenticator number: 42") {
		t.Fatalf("message missing detail field: %q", msg)
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
