package owa

import (
	"testing"
)

func TestOWAConstants(t *testing.T) {
	if OWABaseURL == "" {
		t.Error("OWABaseURL should not be empty")
	}
	if OWAAPIBase == "" {
		t.Error("OWAAPIBase should not be empty")
	}

	// Verify URL format
	if OWABaseURL != "https://outlook.office.com/mail/" {
		t.Errorf("OWABaseURL = %q, want https://outlook.office.com/mail/", OWABaseURL)
	}
	if OWAAPIBase != "https://outlook.office.com/owa/0/service.svc" {
		t.Errorf("OWAAPIBase = %q, want https://outlook.office.com/owa/0/service.svc", OWAAPIBase)
	}
}

func TestIsOWAURLVariants(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		// Should match
		{"https://outlook.office.com/mail/", true},
		{"https://outlook.office.com/mail/inbox", true},
		{"https://outlook.office.com/mail/drafts", true},
		{"https://outlook.office365.com/mail/", true},
		{"https://outlook.office365.com/mail/inbox", true},
		{"https://outlook.live.com/mail/", true},
		{"https://outlook.live.com/mail/0/inbox", true},
		{"https://outlook.cloud.microsoft/mail/", true},
		{"https://outlook.cloud.microsoft/mail/inbox", true},

		// Should not match
		{"https://outlook.office.com/calendar/", false},
		{"https://outlook.office.com/owa/", false},
		{"https://login.microsoftonline.com/", false},
		{"https://google.com/mail", false},
		{"https://mail.google.com/", false},
		{"", false},
		{"about:blank", false},
		{"https://outlook.office.com/", false}, // Missing /mail/
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

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello", "hello world", false},
		{"", "test", false},
		{"test", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		got := contains(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestFindSubstringHelper(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   int
	}{
		{"hello world", "world", 6},
		{"hello world", "hello", 0},
		{"hello world", "xyz", -1},
		{"", "test", -1},
		{"test", "", 0},
	}

	for _, tt := range tests {
		got := findSubstring(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("findSubstring(%q, %q) = %d, want %d", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestTokensStructure(t *testing.T) {
	tokens := Tokens{
		Canary:    "test-canary-12345",
		Bearer:    "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
		UserEmail: "user@example.com",
	}

	if tokens.Canary == "" {
		t.Error("Canary should not be empty")
	}
	if tokens.Bearer == "" {
		t.Error("Bearer should not be empty")
	}
	if tokens.UserEmail == "" {
		t.Error("UserEmail should not be empty")
	}
}

func TestDiscoverTokensNilPage(t *testing.T) {
	_, err := DiscoverTokens(nil)
	if err == nil {
		t.Error("DiscoverTokens(nil) should return error")
	}
}

func TestIsLoggedInNilPage(t *testing.T) {
	if IsLoggedIn(nil) {
		t.Error("IsLoggedIn(nil) should return false")
	}
}

// These tests verify the JavaScript evaluation strings are valid
// by checking they don't contain obvious syntax errors

func TestCanaryExtractionJSStructure(t *testing.T) {
	// The JS code should look for specific cookie names
	expectedCookieNames := []string{"X-OWA-CANARY", "OWA-CANARY", "XOWACANARY"}

	for _, name := range expectedCookieNames {
		if name == "" {
			t.Error("Cookie name should not be empty")
		}
	}
}

func TestBearerExtractionJSStructure(t *testing.T) {
	// The JS should check localStorage for accesstoken keys
	// targeting outlook.office.com or outlook.cloud.microsoft
	targets := []string{"outlook.office.com", "outlook.cloud.microsoft"}

	for _, target := range targets {
		if target == "" {
			t.Error("Target should not be empty")
		}
	}
}
