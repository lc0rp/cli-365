package owa

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenCachePath(t *testing.T) {
	path := TokenCachePath()
	if path == "" {
		t.Error("TokenCachePath() returned empty string")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("TokenCachePath() returned relative path: %s", path)
	}
}

func TestSaveAndLoadTokens(t *testing.T) {
	// Create a temp directory for testing
	tmpDir := t.TempDir()

	// Override the state dir for testing
	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	tokens := &Tokens{
		Canary:      "test-canary-token",
		Bearer:      "Bearer test-bearer-token",
		UserEmail:   "test@example.com",
		ExtractedAt: time.Now().Truncate(time.Second),
	}

	// Save tokens
	if err := SaveTokens(tokens); err != nil {
		t.Fatalf("SaveTokens() error = %v", err)
	}

	// Load tokens back
	loaded, err := LoadTokens()
	if err != nil {
		t.Fatalf("LoadTokens() error = %v", err)
	}

	if loaded.Canary != tokens.Canary {
		t.Errorf("Canary mismatch: got %q, want %q", loaded.Canary, tokens.Canary)
	}
	if loaded.Bearer != tokens.Bearer {
		t.Errorf("Bearer mismatch: got %q, want %q", loaded.Bearer, tokens.Bearer)
	}
	if loaded.UserEmail != tokens.UserEmail {
		t.Errorf("UserEmail mismatch: got %q, want %q", loaded.UserEmail, tokens.UserEmail)
	}
}

func TestLoadOrDiscoverTokensWithCache(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	tokens := &Tokens{
		Canary:      "cached-canary",
		Bearer:      "Bearer cached-token",
		UserEmail:   "cached@example.com",
		ExtractedAt: time.Now().Truncate(time.Second),
	}

	if err := SaveTokens(tokens); err != nil {
		t.Fatalf("SaveTokens() error = %v", err)
	}

	loaded, err := LoadOrDiscoverTokens(nil)
	if err != nil {
		t.Fatalf("LoadOrDiscoverTokens() error = %v", err)
	}
	if loaded.Canary != tokens.Canary {
		t.Errorf("Canary mismatch: got %q, want %q", loaded.Canary, tokens.Canary)
	}
}

func TestLoadOrDiscoverTokensWithoutCache(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	_, err := LoadOrDiscoverTokens(nil)
	if err == nil {
		t.Error("LoadOrDiscoverTokens() should return error without cache and nil page")
	}
}

func TestClearTokens(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	tokens := &Tokens{
		Canary:      "test-canary",
		ExtractedAt: time.Now(),
	}

	// Save first
	if err := SaveTokens(tokens); err != nil {
		t.Fatalf("SaveTokens() error = %v", err)
	}

	// Clear
	if err := ClearTokens(); err != nil {
		t.Fatalf("ClearTokens() error = %v", err)
	}

	// Verify cleared
	_, err := LoadTokens()
	if err == nil {
		t.Error("LoadTokens() should return error after ClearTokens()")
	}
}

func TestClearTokensNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	// Clear without saving first should not error
	if err := ClearTokens(); err != nil {
		t.Errorf("ClearTokens() should not error for non-existent file: %v", err)
	}
}

func TestIsOWAURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://outlook.office.com/mail/inbox", true},
		{"https://outlook.office365.com/mail/", true},
		{"https://outlook.live.com/mail/", true},
		{"https://outlook.cloud.microsoft/mail/", true},
		{"https://google.com", false},
		{"https://mail.google.com", false},
		{"", false},
		{"about:blank", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := isOWAURL(tt.url)
			if got != tt.want {
				t.Errorf("isOWAURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	// NewClient should not panic with nil browser
	client := NewClient(nil)
	if client == nil {
		t.Error("NewClient() returned nil")
	}
	if client.Page() != nil {
		t.Error("NewClient() should have nil page initially")
	}
	if client.Tokens() != nil {
		t.Error("NewClient() should have nil tokens initially")
	}
}

func TestClientSetTokens(t *testing.T) {
	client := NewClient(nil)

	tokens := &Tokens{
		Canary:    "test-canary",
		UserEmail: "test@example.com",
	}

	client.SetTokens(tokens)

	got := client.Tokens()
	if got == nil {
		t.Fatal("Tokens() returned nil after SetTokens()")
	}
	if got.Canary != tokens.Canary {
		t.Errorf("Canary mismatch: got %q, want %q", got.Canary, tokens.Canary)
	}
}

func TestClientConnectWithoutBrowser(t *testing.T) {
	client := NewClient(nil)
	err := client.Connect()
	if err == nil {
		t.Error("Connect() should error when browser is nil")
	}
}
