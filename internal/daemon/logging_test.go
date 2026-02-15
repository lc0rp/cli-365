package daemon

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLogEventStructuredAndRedacted(t *testing.T) {
	srv := NewServer(testOptions(t), nil)

	var buf bytes.Buffer
	srv.logWriter = &buf

	rawBearer := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc.def"
	rawCanary := "X-OWA-CANARY=abc123SECRET"
	rawToken := "token=super-secret-value"

	srv.logEvent("INFO", "request_complete", map[string]interface{}{
		"request_id": "r-1",
		"stderr":     "status 401 unauthorized " + rawBearer + " " + rawCanary + " " + rawToken,
		"token":      "plain-secret-token",
		"nested": map[string]interface{}{
			"authorization": "Bearer should-not-leak",
			"message":       "safe text",
		},
	})

	output := strings.TrimSpace(buf.String())
	if output == "" {
		t.Fatal("expected log output")
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("invalid JSON log output: %v", err)
	}

	if entry["event"] != "request_complete" {
		t.Fatalf("event = %v, want request_complete", entry["event"])
	}
	if entry["level"] != "info" {
		t.Fatalf("level = %v, want info", entry["level"])
	}
	if _, ok := entry["ts"]; !ok {
		t.Fatal("missing ts field")
	}

	if strings.Contains(output, rawBearer) {
		t.Fatal("log leaked raw bearer token")
	}
	if strings.Contains(output, rawCanary) {
		t.Fatal("log leaked raw canary token")
	}
	if strings.Contains(output, rawToken) {
		t.Fatal("log leaked raw token value")
	}
	if strings.Contains(output, "plain-secret-token") {
		t.Fatal("log leaked sensitive token field")
	}
	if strings.Contains(output, "should-not-leak") {
		t.Fatal("log leaked nested authorization field")
	}
	if !strings.Contains(output, "\\u003credacted") && !strings.Contains(output, "<redacted") {
		t.Fatal("expected redaction placeholders in output")
	}
}
