package owa

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactString(t *testing.T) {
	input := "Bearer abc.def.ghi and user@example.com plus 123e4567-e89b-12d3-a456-426614174000"
	output := redactString(input, false)
	if strings.Contains(output, "example.com") {
		t.Fatalf("expected email redacted, got %q", output)
	}
	if strings.Contains(output, "Bearer") && strings.Contains(output, "abc.def.ghi") {
		t.Fatalf("expected bearer redacted, got %q", output)
	}
	if strings.Contains(output, "123e4567-e89b-12d3-a456-426614174000") {
		t.Fatalf("expected guid redacted, got %q", output)
	}
}

func TestRedactJSONBody(t *testing.T) {
	input := `{"Action":"GetTimeZone","AccessToken":"supersecretvalue","Email":"user@example.com"}`
	output := redactBody(input, "application/json", false)
	var payload map[string]string
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("expected JSON, got error: %v", err)
	}
	if payload["Action"] != "GetTimeZone" {
		t.Fatalf("expected Action preserved, got %q", payload["Action"])
	}
	if strings.Contains(payload["AccessToken"], "supersecretvalue") {
		t.Fatalf("expected AccessToken redacted, got %q", payload["AccessToken"])
	}
	if strings.Contains(payload["Email"], "example.com") {
		t.Fatalf("expected Email redacted, got %q", payload["Email"])
	}
}

func TestNormalizeBodyBinary(t *testing.T) {
	body, truncated := normalizeBody([]byte{0xff, 0xfe, 0xfd}, 64)
	if body != binaryPlaceholder {
		t.Fatalf("expected binary placeholder, got %q", body)
	}
	if truncated {
		t.Fatalf("unexpected truncation")
	}
}
