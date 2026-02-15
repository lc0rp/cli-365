package daemon

import (
	"testing"
	"time"

	"github.com/lc0rp/cli-365/internal/config"
)

func TestResolveOptionsAuthRecoveryDefaults(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.SecureInput = ""
	cfg.Daemon.AuthRecoveryTimeout = 0
	cfg.Daemon.Notify.Provider = ""
	cfg.Daemon.Notify.OpenClawCmd = ""
	cfg.Daemon.Notify.Channel = ""

	opts := ResolveOptions(cfg)
	if opts.AuthRecoveryTimeout != 5*time.Minute {
		t.Fatalf("AuthRecoveryTimeout = %s, want 5m", opts.AuthRecoveryTimeout)
	}
	if opts.AuthProbeInterval != 2*time.Second {
		t.Fatalf("AuthProbeInterval = %s, want 2s", opts.AuthProbeInterval)
	}
	if opts.SecureInputCommand != "secure-targeted-input" {
		t.Fatalf("SecureInputCommand = %q, want secure-targeted-input", opts.SecureInputCommand)
	}
	if opts.NotifyProvider != "openclaw-cli" {
		t.Fatalf("NotifyProvider = %q, want openclaw-cli", opts.NotifyProvider)
	}
	if opts.NotifyOpenClawCmd != "openclaw" {
		t.Fatalf("NotifyOpenClawCmd = %q, want openclaw", opts.NotifyOpenClawCmd)
	}
	if opts.NotifyChannel != "discord" {
		t.Fatalf("NotifyChannel = %q, want discord", opts.NotifyChannel)
	}
	if opts.LoginURL != "https://outlook.office.com/mail/" {
		t.Fatalf("LoginURL = %q, want OWA URL", opts.LoginURL)
	}
}

func TestResolveOptionsAuthRecoveryOverrides(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.SecureInput = "/usr/local/bin/secure-targeted-input --focus login"
	cfg.Daemon.AuthRecoveryTimeout = 3 * time.Minute
	cfg.Daemon.Notify.Provider = "openclaw-cli"
	cfg.Daemon.Notify.OpenClawCmd = "/usr/local/bin/openclaw"
	cfg.Daemon.Notify.Channel = "whatsapp"
	cfg.Daemon.Notify.Target = "ops-team"

	opts := ResolveOptions(cfg)
	if opts.AuthRecoveryTimeout != 3*time.Minute {
		t.Fatalf("AuthRecoveryTimeout = %s, want 3m", opts.AuthRecoveryTimeout)
	}
	if opts.SecureInputCommand != cfg.Auth.SecureInput {
		t.Fatalf("SecureInputCommand = %q, want %q", opts.SecureInputCommand, cfg.Auth.SecureInput)
	}
	if opts.NotifyOpenClawCmd != cfg.Daemon.Notify.OpenClawCmd {
		t.Fatalf("NotifyOpenClawCmd = %q, want %q", opts.NotifyOpenClawCmd, cfg.Daemon.Notify.OpenClawCmd)
	}
	if opts.NotifyChannel != "whatsapp" {
		t.Fatalf("NotifyChannel = %q, want whatsapp", opts.NotifyChannel)
	}
	if opts.NotifyTarget != "ops-team" {
		t.Fatalf("NotifyTarget = %q, want ops-team", opts.NotifyTarget)
	}
}
