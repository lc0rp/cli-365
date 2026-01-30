package owa

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTokensJSONRoundTrip(t *testing.T) {
	original := Tokens{
		Canary:      "test-canary-token-abc123",
		Bearer:      "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
		UserEmail:   "user@example.com",
		ExtractedAt: time.Now().Truncate(time.Second),
		ExpiresAt:   time.Now().Add(time.Hour).Truncate(time.Second),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Tokens
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Canary != original.Canary {
		t.Errorf("Canary = %q, want %q", parsed.Canary, original.Canary)
	}
	if parsed.Bearer != original.Bearer {
		t.Errorf("Bearer = %q, want %q", parsed.Bearer, original.Bearer)
	}
	if parsed.UserEmail != original.UserEmail {
		t.Errorf("UserEmail = %q, want %q", parsed.UserEmail, original.UserEmail)
	}
	if !parsed.ExtractedAt.Equal(original.ExtractedAt) {
		t.Errorf("ExtractedAt mismatch")
	}
	if !parsed.ExpiresAt.Equal(original.ExpiresAt) {
		t.Errorf("ExpiresAt mismatch")
	}
}

func TestContainsExtended(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"outlook.office.com/mail", "mail", true},
		{"outlook.office.com/mail", "outlook", true},
		{"outlook.office.com/mail", "calendar", false},
		{"outlook.office.com/mail/inbox/AAMk", "AAMk", true},
		{"https://outlook.office365.com/mail/", "office365", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestFindSubstringExtended(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   int
	}{
		{"abcabc", "bc", 1},
		{"canary=abc123;other", "canary=", 0},
		{"canary=abc123;other", ";", 13},
		{"X-OWA-CANARY=value", "CANARY", 6},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := findSubstring(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("findSubstring(%q, %q) = %d, want %d", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestIsOWAURLExtended(t *testing.T) {
	// Additional edge cases beyond the main test
	tests := []struct {
		url  string
		want bool
	}{
		// With paths
		{"https://outlook.office.com/mail/inbox", true},
		{"https://outlook.office.com/mail/drafts", true},
		{"https://outlook.office.com/mail/sentitems", true},
		{"https://outlook.office.com/owa/", true},
		{"https://outlook.office.com/calendar/", true},
		// Case variations (URL matching is case-sensitive in impl)
		{"https://OUTLOOK.OFFICE.COM/mail/", false}, // Case matters
		// Near misses
		{"https://outlook.office.com/people/", false},
		{"https://outlook.office.com/", false},
		// Various protocols
		{"http://outlook.office.com/mail/", true},
		// Invalid URLs
		{"not-a-url", false},
		{"javascript:alert(1)", false},
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

func TestClientTokensAfterMultipleSets(t *testing.T) {
	client := NewClient(nil)

	tokens1 := &Tokens{Canary: "first"}
	tokens2 := &Tokens{Canary: "second"}
	tokens3 := &Tokens{Canary: "third"}

	client.SetTokens(tokens1)
	if client.Tokens().Canary != "first" {
		t.Error("first set failed")
	}

	client.SetTokens(tokens2)
	if client.Tokens().Canary != "second" {
		t.Error("second set failed")
	}

	client.SetTokens(tokens3)
	if client.Tokens().Canary != "third" {
		t.Error("third set failed")
	}
}

func TestTokensFieldAccess(t *testing.T) {
	tokens := &Tokens{
		Canary:      "canary-value",
		Bearer:      "bearer-value",
		UserEmail:   "user@test.com",
		ExtractedAt: time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	// Verify all fields are accessible
	if tokens.Canary == "" {
		t.Error("Canary should not be empty")
	}
	if tokens.Bearer == "" {
		t.Error("Bearer should not be empty")
	}
	if tokens.UserEmail == "" {
		t.Error("UserEmail should not be empty")
	}
	if tokens.ExtractedAt.IsZero() {
		t.Error("ExtractedAt should not be zero")
	}
	if tokens.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should not be zero")
	}
}

func TestTokensWithEmptyCanary(t *testing.T) {
	tokens := &Tokens{
		Canary:      "",
		Bearer:      "has-bearer",
		ExtractedAt: time.Now(),
	}

	if tokens.Canary != "" {
		t.Error("Empty canary should remain empty")
	}
	if tokens.Bearer == "" {
		t.Error("Bearer should not be empty")
	}
}
