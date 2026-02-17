package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.ProfileDir == "" {
		t.Error("ProfileDir should not be empty")
	}
	if cfg.Auth.Tenant != "common" {
		t.Errorf("Auth.Tenant = %q, want common", cfg.Auth.Tenant)
	}
	if cfg.Security.Keyring != "os" {
		t.Errorf("Security.Keyring = %q, want os", cfg.Security.Keyring)
	}
	if len(cfg.Auth.Scopes) == 0 {
		t.Error("Auth.Scopes should not be empty")
	}
	if cfg.Auth.SecureInput != "secure-targeted-input" {
		t.Errorf("Auth.SecureInput = %q, want secure-targeted-input", cfg.Auth.SecureInput)
	}
	if cfg.Daemon.MaxQueueSize != 64 {
		t.Errorf("Daemon.MaxQueueSize = %d, want 64", cfg.Daemon.MaxQueueSize)
	}
	if !cfg.Daemon.Enabled {
		t.Error("Daemon.Enabled should be true by default")
	}
	if cfg.Daemon.MaxRequestBytes != 1024*1024 {
		t.Errorf("Daemon.MaxRequestBytes = %d, want 1048576", cfg.Daemon.MaxRequestBytes)
	}
	if cfg.Daemon.MaxResponseBytes != 1024*1024 {
		t.Errorf("Daemon.MaxResponseBytes = %d, want 1048576", cfg.Daemon.MaxResponseBytes)
	}
	if cfg.Daemon.DefaultCommandTimeout <= 0 {
		t.Errorf("Daemon.DefaultCommandTimeout = %s, want > 0", cfg.Daemon.DefaultCommandTimeout)
	}
	if !cfg.Daemon.RejectNewWhileAuthPaused {
		t.Error("Daemon.RejectNewWhileAuthPaused should be true")
	}
	if !cfg.Daemon.CoalesceIdenticalReads {
		t.Error("Daemon.CoalesceIdenticalReads should be true")
	}
	if cfg.Daemon.DuplicateWriteWindowMail != 12*time.Hour {
		t.Errorf("Daemon.DuplicateWriteWindowMail = %s, want 12h", cfg.Daemon.DuplicateWriteWindowMail)
	}
	if cfg.Daemon.DuplicateWriteWindowCalendar != time.Hour {
		t.Errorf("Daemon.DuplicateWriteWindowCalendar = %s, want 1h", cfg.Daemon.DuplicateWriteWindowCalendar)
	}
	if cfg.Daemon.WriteRateLimitPerMinute != 20 {
		t.Errorf("Daemon.WriteRateLimitPerMinute = %d, want 20", cfg.Daemon.WriteRateLimitPerMinute)
	}
	if cfg.Daemon.RecipientWriteRateLimitPerMinute != 6 {
		t.Errorf("Daemon.RecipientWriteRateLimitPerMinute = %d, want 6", cfg.Daemon.RecipientWriteRateLimitPerMinute)
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent", "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load should not error for non-existent file: %v", err)
	}

	// Should return defaults
	if cfg.Auth.Tenant != "common" {
		t.Errorf("Auth.Tenant = %q, want common", cfg.Auth.Tenant)
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	// Create custom config
	cfg := Default()
	cfg.Auth.Tenant = "test-tenant"
	cfg.Browser.Headless = false
	cfg.Browser.NoSandbox = true
	cfg.Browser.CDPPort = 9222

	// Save
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Load
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if loaded.Auth.Tenant != cfg.Auth.Tenant {
		t.Errorf("Auth.Tenant = %q, want %q", loaded.Auth.Tenant, cfg.Auth.Tenant)
	}
	if loaded.Browser.Headless != cfg.Browser.Headless {
		t.Errorf("Browser.Headless = %v, want %v", loaded.Browser.Headless, cfg.Browser.Headless)
	}
	if loaded.Browser.NoSandbox != cfg.Browser.NoSandbox {
		t.Errorf("Browser.NoSandbox = %v, want %v", loaded.Browser.NoSandbox, cfg.Browser.NoSandbox)
	}
	if loaded.Browser.CDPPort != cfg.Browser.CDPPort {
		t.Errorf("Browser.CDPPort = %v, want %v", loaded.Browser.CDPPort, cfg.Browser.CDPPort)
	}
}

func TestResolvePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Empty returns default", ""},
		{"Absolute path", "/tmp/test.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolvePath(tt.input)
			if result == "" {
				t.Error("ResolvePath returned empty string")
			}
		})
	}
}

func TestLoadWithTildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config with tilde path
	cfg := Default()
	cfg.ProfileDir = "~/test-profile"

	// Override HOME for testing
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	path := filepath.Join(tmpDir, "config.yaml")
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// ProfileDir should be expanded
	if loaded.ProfileDir == "~/test-profile" {
		t.Error("ProfileDir tilde was not expanded")
	}
}

func TestBrowserConfig(t *testing.T) {
	cfg := BrowserConfig{
		Headless:    true,
		NoSandbox:   true,
		CDPEndpoint: "ws://localhost:9222",
		CDPPort:     9222,
	}

	if !cfg.Headless {
		t.Error("Headless should be true")
	}
	if !cfg.NoSandbox {
		t.Error("NoSandbox should be true")
	}
	if cfg.CDPEndpoint == "" {
		t.Error("CDPEndpoint should not be empty")
	}
	if cfg.CDPPort == 0 {
		t.Error("CDPPort should not be zero")
	}
}

func TestAuthConfig(t *testing.T) {
	cfg := AuthConfig{
		Tenant:      "mytenant",
		AccountHint: "user@example.com",
		Readonly:    true,
		SecureInput: "secure-targeted-input",
		Scopes:      []string{"mail.read"},
	}

	if cfg.Tenant != "mytenant" {
		t.Errorf("Tenant = %q, want mytenant", cfg.Tenant)
	}
	if !cfg.Readonly {
		t.Error("Readonly should be true")
	}
	if len(cfg.Scopes) != 1 {
		t.Errorf("Scopes length = %d, want 1", len(cfg.Scopes))
	}
	if cfg.SecureInput == "" {
		t.Error("SecureInput should not be empty")
	}
}

func TestSecurityConfig(t *testing.T) {
	cfg := SecurityConfig{
		Allowlist: []string{"mail", "auth"},
		Keyring:   "encrypted-file",
	}

	if len(cfg.Allowlist) != 2 {
		t.Errorf("Allowlist length = %d, want 2", len(cfg.Allowlist))
	}
	if cfg.Keyring != "encrypted-file" {
		t.Errorf("Keyring = %q, want encrypted-file", cfg.Keyring)
	}
}
