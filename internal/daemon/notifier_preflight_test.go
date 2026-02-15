package daemon

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestNotifierCommandName(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		command  string
		want     string
	}{
		{
			name:     "openclaw with args",
			provider: "openclaw-cli",
			command:  "openclaw message send",
			want:     "openclaw",
		},
		{
			name:     "provider casing",
			provider: "OPENCLAW-CLI",
			command:  "  openclaw  ",
			want:     "openclaw",
		},
		{
			name:     "unsupported provider",
			provider: "none",
			command:  "openclaw",
			want:     "",
		},
		{
			name:     "empty command",
			provider: "openclaw-cli",
			command:  "   ",
			want:     "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := notifierCommandName(tt.provider, tt.command)
			if got != tt.want {
				t.Fatalf("notifierCommandName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLogNotifierAvailabilityLogsMissingCommand(t *testing.T) {
	opts := testOptions(t)
	opts.NotifyProvider = "openclaw-cli"
	opts.NotifyOpenClawCmd = "missing-openclaw message send"

	srv := NewServer(opts, nil)
	srv.lookupPath = func(file string) (string, error) {
		if file != "missing-openclaw" {
			t.Fatalf("lookupPath file = %q, want missing-openclaw", file)
		}
		return "", errors.New("not found")
	}

	var buf bytes.Buffer
	srv.SetLogWriter(&buf)
	srv.logNotifierAvailability()

	out := buf.String()
	if !strings.Contains(out, "notifier_unavailable") {
		t.Fatalf("log output missing notifier_unavailable event: %q", out)
	}
	if !strings.Contains(out, "missing-openclaw") {
		t.Fatalf("log output missing command: %q", out)
	}
}

func TestLogNotifierAvailabilitySkipsWhenAvailable(t *testing.T) {
	opts := testOptions(t)
	opts.NotifyProvider = "openclaw-cli"
	opts.NotifyOpenClawCmd = "openclaw"

	srv := NewServer(opts, nil)
	srv.lookupPath = func(file string) (string, error) {
		if file != "openclaw" {
			t.Fatalf("lookupPath file = %q, want openclaw", file)
		}
		return "/usr/bin/openclaw", nil
	}

	var buf bytes.Buffer
	srv.SetLogWriter(&buf)
	srv.logNotifierAvailability()

	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Fatalf("expected no logs, got %q", got)
	}
}
