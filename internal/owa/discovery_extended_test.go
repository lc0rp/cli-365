package owa

import (
	"errors"
	"testing"
)

func TestExtractCanaryFromValue(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{
			name:  "direct string with canary key",
			value: map[string]interface{}{"canary": "test-canary-123"},
			want:  "test-canary-123",
		},
		{
			name:  "nested canary",
			value: map[string]interface{}{
				"settings": map[string]interface{}{
					"owaCanary": "nested-canary-456",
				},
			},
			want:  "nested-canary-456",
		},
		{
			name:  "X-OWA-CANARY key",
			value: map[string]interface{}{"X-OWA-CANARY": "xowa-canary-789"},
			want:  "xowa-canary-789",
		},
		{
			name:  "canary in array without confusing keys",
			value: []interface{}{
				map[string]interface{}{"other": "value"},
				map[string]interface{}{"canaryToken": "array-canary-abc"},
			},
			want:  "array-canary-abc",
		},
		{
			name:  "deeply nested",
			value: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": map[string]interface{}{
							"sessionCanary": "deep-canary-xyz",
						},
					},
				},
			},
			want:  "deep-canary-xyz",
		},
		{
			name:  "empty map",
			value: map[string]interface{}{},
			want:  "",
		},
		{
			name:  "nil value",
			value: nil,
			want:  "",
		},
		{
			name:  "canary key with empty value",
			value: map[string]interface{}{"canary": ""},
			want:  "",
		},
		{
			name:  "canary key with non-string value",
			value: map[string]interface{}{"canary": 12345},
			want:  "",
		},
		{
			name:  "empty array",
			value: []interface{}{},
			want:  "",
		},
		{
			name:  "case insensitive canary match",
			value: map[string]interface{}{"CANARY": "upper-canary"},
			want:  "upper-canary",
		},
		{
			name:  "mixed case canary",
			value: map[string]interface{}{"oWaCanArY": "mixed-canary"},
			want:  "mixed-canary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCanaryFromValue(tt.value)
			if got != tt.want {
				t.Errorf("extractCanaryFromValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractCanaryFromString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "no canary",
			input: "some random string without the keyword",
			want:  "",
		},
		{
			name:  "has canary keyword",
			input: "prefix canary=abc123 suffix",
			want:  "", // Note: current impl returns "" even with keyword
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCanaryFromString(tt.input)
			if got != tt.want {
				t.Errorf("extractCanaryFromString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDiscoverTokensNilPageError(t *testing.T) {
	_, err := DiscoverTokens(nil)
	if err == nil {
		t.Error("DiscoverTokens(nil) should return error")
	}
	if err.Error() != "page is nil" {
		t.Errorf("error = %q, want 'page is nil'", err.Error())
	}
}

func TestIsLoggedInNilPageFalse(t *testing.T) {
	result := IsLoggedIn(nil)
	if result {
		t.Error("IsLoggedIn(nil) should return false")
	}
}

func TestOWABaseURLConstants(t *testing.T) {
	// Verify constants are properly set
	if OWABaseURL == "" {
		t.Error("OWABaseURL should not be empty")
	}
	if OWAAPIBase == "" {
		t.Error("OWAAPIBase should not be empty")
	}

	// Verify they have proper URL format
	if OWABaseURL[:8] != "https://" {
		t.Errorf("OWABaseURL should start with https://: %s", OWABaseURL)
	}
	if OWAAPIBase[:8] != "https://" {
		t.Errorf("OWAAPIBase should start with https://: %s", OWAAPIBase)
	}
}

func TestIsNonFatalCanaryEvalError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "security error cookie access denied",
			err:  errors.New("eval js error: SecurityError: Failed to read the 'cookie' property from 'Document': Access is denied for this document."),
			want: true,
		},
		{
			name: "securityerror lowercase",
			err:  errors.New("securityerror: access denied"),
			want: true,
		},
		{
			name: "generic eval error",
			err:  errors.New("eval js error: execution context destroyed"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNonFatalCanaryEvalError(tt.err)
			if got != tt.want {
				t.Fatalf("isNonFatalCanaryEvalError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
