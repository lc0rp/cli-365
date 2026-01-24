package owa

import (
	"encoding/json"
	"testing"
)

func TestFetchRequest(t *testing.T) {
	req := FetchRequest{
		URL:    "https://outlook.office.com/owa/0/service.svc?action=Test",
		Method: "POST",
		Headers: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		},
		Body: map[string]interface{}{
			"test": "value",
		},
	}

	// Verify serialization
	if req.URL == "" {
		t.Error("URL should not be empty")
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST", req.Method)
	}
	if req.Headers["Accept"] != "application/json" {
		t.Error("Accept header not set correctly")
	}
}

func TestOWAEndpoint(t *testing.T) {
	tests := []struct {
		action string
		want   string
	}{
		{"FindItem", "https://outlook.office.com/owa/0/service.svc?action=FindItem&app=Mail&n=0"},
		{"GetItem", "https://outlook.office.com/owa/0/service.svc?action=GetItem&app=Mail&n=0"},
		{"CreateItem", "https://outlook.office.com/owa/0/service.svc?action=CreateItem&app=Mail&n=0"},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := OWAEndpoint(tt.action)
			if got != tt.want {
				t.Errorf("OWAEndpoint(%q) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

func TestOWAEndpointForURL(t *testing.T) {
	tests := []struct {
		pageURL string
		action  string
		want    string
	}{
		{
			pageURL: "https://outlook.cloud.microsoft/mail/",
			action:  "FindItem",
			want:    "https://outlook.cloud.microsoft/owa/service.svc?action=FindItem&app=Mail&n=0",
		},
		{
			pageURL: "https://outlook.office.com/mail/",
			action:  "FindItem",
			want:    "https://outlook.office.com/owa/0/service.svc?action=FindItem&app=Mail&n=0",
		},
		{
			pageURL: "https://outlook.office365.com/mail/",
			action:  "FindItem",
			want:    "https://outlook.office365.com/owa/0/service.svc?action=FindItem&app=Mail&n=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.pageURL, func(t *testing.T) {
			got := OWAEndpointForURL(tt.pageURL, tt.action)
			if got != tt.want {
				t.Errorf("OWAEndpointForURL(%q) = %q, want %q", tt.pageURL, got, tt.want)
			}
		})
	}
}

func TestNewOWARequest(t *testing.T) {
	body := map[string]string{"test": "value"}
	req := NewOWARequest(body)

	if req.Header.RequestServerVersion != "Exchange2016" {
		t.Errorf("RequestServerVersion = %q, want Exchange2016", req.Header.RequestServerVersion)
	}
	if req.Body == nil {
		t.Error("Body should not be nil")
	}
}

func TestNewOWARequestSerialization(t *testing.T) {
	body := map[string]interface{}{
		"ItemShape": map[string]interface{}{
			"BaseShape": "IdOnly",
		},
	}
	req := NewOWARequest(body)

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	if len(data) == 0 {
		t.Error("Serialized request is empty")
	}

	// Verify it contains expected fields
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if _, ok := parsed["Header"]; !ok {
		t.Error("Serialized request missing Header")
	}
	if _, ok := parsed["Body"]; !ok {
		t.Error("Serialized request missing Body")
	}
}

func TestFetchResponseParsing(t *testing.T) {
	respJSON := `{
		"status": 200,
		"statusText": "OK",
		"headers": {"content-type": "application/json"},
		"body": {"Items": []}
	}`

	var resp FetchResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("Status = %d, want 200", resp.Status)
	}
	if resp.StatusText != "OK" {
		t.Errorf("StatusText = %q, want OK", resp.StatusText)
	}
}

func TestOWATimeZone(t *testing.T) {
	tz := OWATimeZone{}
	tz.TimeZoneDefinition.Id = "Eastern Standard Time"

	data, err := json.Marshal(tz)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed OWATimeZone
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.TimeZoneDefinition.Id != "Eastern Standard Time" {
		t.Errorf("TimeZone Id = %q, want Eastern Standard Time", parsed.TimeZoneDefinition.Id)
	}
}
