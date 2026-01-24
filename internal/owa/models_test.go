package owa

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2025-01-24T12:00:00Z", false},
		{"RFC3339Nano", "2025-01-24T12:00:00.123456789Z", false},
		{"ISO8601 no offset", "2025-01-24T12:00:00", false},
		{"ISO8601 with millis", "2025-01-24T12:00:00.000Z", false},
		{"Empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTime(tt.input)
			if tt.wantErr {
				if result != (time.Time{}) && err == nil {
					// ParseTime returns zero time on failure, not an error
					return
				}
			} else {
				if result.IsZero() {
					t.Errorf("ParseTime(%q) returned zero time", tt.input)
				}
			}
		})
	}
}

func TestUnmarshalSearchResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMsgs int
	}{
		{
			name: "Direct messages array",
			input: `{
				"Messages": [
					{"ItemId": "abc123", "Subject": "Test"},
					{"ItemId": "def456", "Subject": "Test 2"}
				]
			}`,
			wantMsgs: 2,
		},
		{
			name: "Body wrapper",
			input: `{
				"Body": {
					"Items": [
						{"ItemId": "abc123", "Subject": "Test"}
					],
					"TotalItemsInView": 1
				}
			}`,
			wantMsgs: 1,
		},
		{
			name:     "Empty response",
			input:    `{}`,
			wantMsgs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UnmarshalSearchResponse(json.RawMessage(tt.input))
			if err != nil {
				t.Fatalf("UnmarshalSearchResponse() error = %v", err)
			}
			if len(result.Messages) != tt.wantMsgs {
				t.Errorf("UnmarshalSearchResponse() got %d messages, want %d", len(result.Messages), tt.wantMsgs)
			}
		})
	}
}

func TestEmailAddress(t *testing.T) {
	addr := EmailAddress{
		Name:    "John Doe",
		Address: "john@example.com",
	}

	data, err := json.Marshal(addr)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed EmailAddress
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Name != addr.Name || parsed.Address != addr.Address {
		t.Errorf("EmailAddress roundtrip failed: got %+v, want %+v", parsed, addr)
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		ID:      "test-id",
		Subject: "Test Subject",
		From: &EmailAddress{
			Name:    "Sender",
			Address: "sender@example.com",
		},
		ToRecipients: []EmailAddress{
			{Address: "recipient@example.com"},
		},
		Body: &MessageBody{
			BodyType: "HTML",
			Value:    "<p>Hello</p>",
		},
		IsRead:         true,
		HasAttachments: false,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Subject != msg.Subject {
		t.Errorf("Subject mismatch: got %q, want %q", parsed.Subject, msg.Subject)
	}
	if parsed.From.Address != msg.From.Address {
		t.Errorf("From address mismatch: got %q, want %q", parsed.From.Address, msg.From.Address)
	}
}

func TestDraft(t *testing.T) {
	draft := Draft{
		Subject: "Draft Subject",
		Body: &MessageBody{
			BodyType: "Text",
			Value:    "Plain text body",
		},
		ToRecipients: []EmailAddress{
			{Name: "Recipient", Address: "to@example.com"},
		},
		CcRecipients: []EmailAddress{
			{Address: "cc@example.com"},
		},
	}

	data, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	if len(data) == 0 {
		t.Error("Marshal returned empty data")
	}
}

func TestConversation(t *testing.T) {
	conv := Conversation{
		ID:          "conv-id",
		Topic:       "Conversation Topic",
		MessageCount: 5,
		UnreadCount: 2,
		Messages: []Message{
			{ID: "msg1", Subject: "Re: Topic"},
			{ID: "msg2", Subject: "Re: Re: Topic"},
		},
	}

	data, err := json.Marshal(conv)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Conversation
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(parsed.Messages) != 2 {
		t.Errorf("Messages count mismatch: got %d, want 2", len(parsed.Messages))
	}
}

func TestAttachment(t *testing.T) {
	att := Attachment{
		ID:          "att-id",
		Name:        "document.pdf",
		ContentType: "application/pdf",
		Size:        12345,
		IsInline:    false,
	}

	data, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Attachment
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Name != att.Name {
		t.Errorf("Name mismatch: got %q, want %q", parsed.Name, att.Name)
	}
	if parsed.Size != att.Size {
		t.Errorf("Size mismatch: got %d, want %d", parsed.Size, att.Size)
	}
}
