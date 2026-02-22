package owa

import (
	"encoding/json"
	"testing"
)

func TestOWAEndpointAllActions(t *testing.T) {
	// Test all common OWA actions
	actions := []string{
		"FindItem",
		"GetItem",
		"CreateItem",
		"UpdateItem",
		"DeleteItem",
		"SendItem",
		"GetAttachment",
		"CreateAttachment",
		"FindConversation",
		"GetConversationItems",
		"MoveItem",
		"CopyItem",
		"MarkItemsAsRead",
		"GetFolder",
		"FindFolder",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			endpoint := OWAEndpoint(action)
			if endpoint == "" {
				t.Errorf("OWAEndpoint(%q) returned empty string", action)
			}
			// Should contain the action name
			if !contains(endpoint, action) {
				t.Errorf("OWAEndpoint(%q) = %q, doesn't contain action name", action, endpoint)
			}
			// Should contain service.svc
			if !contains(endpoint, "service.svc") {
				t.Errorf("OWAEndpoint(%q) = %q, doesn't contain service.svc", action, endpoint)
			}
		})
	}
}

func TestOWAEndpointForURLAllDomains(t *testing.T) {
	domains := []struct {
		pageURL      string
		expectedHost string
	}{
		{"https://outlook.office.com/mail/", "outlook.office.com"},
		{"https://outlook.office365.com/mail/", "outlook.office365.com"},
		{"https://outlook.live.com/mail/", "outlook.live.com"},
		{"https://outlook.cloud.microsoft/mail/", "outlook.cloud.microsoft"},
	}

	for _, tt := range domains {
		t.Run(tt.pageURL, func(t *testing.T) {
			endpoint := OWAEndpointForURL(tt.pageURL, "FindItem")
			if endpoint == "" {
				t.Errorf("OWAEndpointForURL(%q) returned empty string", tt.pageURL)
			}
			if !contains(endpoint, tt.expectedHost) {
				t.Errorf("OWAEndpointForURL(%q) = %q, doesn't contain %q", tt.pageURL, endpoint, tt.expectedHost)
			}
		})
	}
}

func TestNewOWARequestStructure(t *testing.T) {
	body := map[string]interface{}{
		"ItemShape": map[string]interface{}{
			"BaseShape": "IdOnly",
		},
		"ParentFolderIds": []map[string]interface{}{
			{"Id": "inbox"},
		},
	}

	req := NewOWARequest(body)

	// Check Header
	if req.Header.RequestServerVersion != "Exchange2016" {
		t.Errorf("RequestServerVersion = %q, want Exchange2016", req.Header.RequestServerVersion)
	}

	// TimeZoneContext may be nil (not set by NewOWARequest)
	// Just verify it doesn't panic
	_ = req.Header.TimeZoneContext

	// Check Body exists
	if req.Body == nil {
		t.Error("Body should not be nil")
	}
}

func TestNewOWARequestSerializesToValidJSON(t *testing.T) {
	body := map[string]interface{}{
		"test": "value",
	}

	req := NewOWARequest(body)

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Parse back to verify it's valid
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Check required keys
	if _, ok := parsed["Header"]; !ok {
		t.Error("Serialized request missing Header")
	}
	if _, ok := parsed["Body"]; !ok {
		t.Error("Serialized request missing Body")
	}
}

func TestFetchRequestStructure(t *testing.T) {
	req := FetchRequest{
		URL:    "https://outlook.office.com/owa/0/service.svc?action=FindItem",
		Method: "POST",
		Headers: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
			"X-OWA-CANARY": "test-canary",
		},
		Body: map[string]interface{}{
			"test": "data",
		},
	}

	// Verify fields
	if req.URL == "" {
		t.Error("URL should not be empty")
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST", req.Method)
	}
	if len(req.Headers) != 3 {
		t.Errorf("Headers count = %d, want 3", len(req.Headers))
	}
	if req.Headers["X-OWA-CANARY"] != "test-canary" {
		t.Error("X-OWA-CANARY header not set correctly")
	}
}

func TestFetchResponseStatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantOK   bool
		wantCode int
	}{
		{
			name:     "success response",
			json:     `{"status": 200, "statusText": "OK", "body": {"Items": []}}`,
			wantOK:   true,
			wantCode: 200,
		},
		{
			name:     "error response",
			json:     `{"status": 401, "statusText": "Unauthorized", "body": {}}`,
			wantOK:   false,
			wantCode: 401,
		},
		{
			name:     "server error",
			json:     `{"status": 500, "statusText": "Internal Server Error", "body": {}}`,
			wantOK:   false,
			wantCode: 500,
		},
		{
			name:     "not found",
			json:     `{"status": 404, "statusText": "Not Found", "body": {}}`,
			wantOK:   false,
			wantCode: 404,
		},
		{
			name:     "created",
			json:     `{"status": 201, "statusText": "Created", "body": {}}`,
			wantOK:   false,
			wantCode: 201,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp FetchResponse
			if err := json.Unmarshal([]byte(tt.json), &resp); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			gotOK := resp.Status == 200
			if gotOK != tt.wantOK {
				t.Errorf("response OK = %v, want %v", gotOK, tt.wantOK)
			}
			if resp.Status != tt.wantCode {
				t.Errorf("Status = %d, want %d", resp.Status, tt.wantCode)
			}
		})
	}
}

func TestOWATimeZoneStructure(t *testing.T) {
	tz := OWATimeZone{}
	tz.TimeZoneDefinition.Id = "Pacific Standard Time"

	data, err := json.Marshal(tz)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Should contain TimeZoneDefinition
	if !contains(string(data), "TimeZoneDefinition") {
		t.Error("Serialized TimeZone should contain TimeZoneDefinition")
	}
}

func TestOWAEndpointForURLUnknownDomain(t *testing.T) {
	// Unknown domain should fall back to default
	endpoint := OWAEndpointForURL("https://unknown.domain.com/mail/", "FindItem")

	// Should still return a valid endpoint (falls back to office.com)
	if endpoint == "" {
		t.Error("OWAEndpointForURL should return endpoint even for unknown domain")
	}
}

func TestOWAEndpointForURLEmptyURL(t *testing.T) {
	endpoint := OWAEndpointForURL("", "FindItem")

	// Should fall back to default
	if endpoint == "" {
		t.Error("OWAEndpointForURL('', 'FindItem') should return default endpoint")
	}
}
