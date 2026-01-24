package owa

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUnmarshalSearchResponseVariants(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		wantMessages  int
		wantConvs     int
		wantTotal     int
	}{
		{
			name: "body wrapper with items",
			json: `{
				"Body": {
					"Items": [
						{"ItemId": "msg1", "Subject": "Message 1"},
						{"ItemId": "msg2", "Subject": "Message 2"},
						{"ItemId": "msg3", "Subject": "Message 3"}
					],
					"TotalItemsInView": 100
				}
			}`,
			wantMessages: 3,
			wantConvs:    0,
			wantTotal:    100,
		},
		{
			name: "body wrapper with conversations",
			json: `{
				"Body": {
					"Conversations": [
						{"ConversationId": "conv1", "ConversationTopic": "Topic 1"},
						{"ConversationId": "conv2", "ConversationTopic": "Topic 2"}
					],
					"TotalItemsInView": 50
				}
			}`,
			wantMessages: 0,
			wantConvs:    2,
			wantTotal:    50,
		},
		{
			name: "direct search result with messages",
			json: `{
				"Messages": [{"ItemId": "msg1", "Subject": "Test"}],
				"TotalCount": 25
			}`,
			wantMessages: 1,
			wantConvs:    0,
			wantTotal:    25,
		},
		{
			name: "nested search result",
			json: `{
				"Body": {
					"SearchResult": {
						"Messages": [{"ItemId": "msg1"}, {"ItemId": "msg2"}],
						"TotalCount": 42
					}
				}
			}`,
			wantMessages: 2,
			wantConvs:    0,
			wantTotal:    42,
		},
		{
			name:          "empty response",
			json:          `{}`,
			wantMessages:  0,
			wantConvs:     0,
			wantTotal:     0,
		},
		{
			name: "empty body",
			json: `{"Body": {}}`,
			wantMessages:  0,
			wantConvs:     0,
			wantTotal:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UnmarshalSearchResponse(json.RawMessage(tt.json))
			if err != nil {
				t.Fatalf("UnmarshalSearchResponse failed: %v", err)
			}

			if len(result.Messages) != tt.wantMessages {
				t.Errorf("Messages count = %d, want %d", len(result.Messages), tt.wantMessages)
			}
			if len(result.Conversations) != tt.wantConvs {
				t.Errorf("Conversations count = %d, want %d", len(result.Conversations), tt.wantConvs)
			}
			if result.TotalCount != tt.wantTotal {
				t.Errorf("TotalCount = %d, want %d", result.TotalCount, tt.wantTotal)
			}
		})
	}
}

func TestParseTimeFormats(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
		check   func(time.Time) bool
	}{
		{
			input:   "2025-01-24T12:30:45Z",
			wantErr: false,
			check: func(t time.Time) bool {
				return t.Year() == 2025 && t.Month() == 1 && t.Day() == 24
			},
		},
		{
			input:   "2025-01-24T12:30:45",
			wantErr: false,
			check: func(t time.Time) bool {
				return t.Hour() == 12 && t.Minute() == 30
			},
		},
		{
			input:   "2025-01-24T12:30:45.123Z",
			wantErr: false,
			check: func(t time.Time) bool {
				return t.Second() == 45
			},
		},
		{
			input:   "2025-01-24T08:00:00-05:00",
			wantErr: false,
			check: func(t time.Time) bool {
				return !t.IsZero()
			},
		},
		{
			input:   "",
			wantErr: false, // Returns zero time
			check: func(t time.Time) bool {
				return t.IsZero()
			},
		},
		{
			input:   "invalid",
			wantErr: false, // Returns zero time, no error
			check: func(t time.Time) bool {
				return t.IsZero()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseTime(tt.input)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.check != nil && !tt.check(result) {
				t.Errorf("time check failed for input %q, got %v", tt.input, result)
			}
		})
	}
}

func TestMessageJSONRoundTrip(t *testing.T) {
	original := Message{
		ID:      "AAMkAGI2TG93AAA=",
		Subject: "Test Subject",
		From: &EmailAddress{
			Name:    "Sender",
			Address: "sender@example.com",
		},
		ToRecipients: []EmailAddress{
			{Name: "Recipient 1", Address: "r1@example.com"},
			{Name: "Recipient 2", Address: "r2@example.com"},
		},
		CcRecipients: []EmailAddress{
			{Address: "cc@example.com"},
		},
		Body: &MessageBody{
			BodyType: "HTML",
			Value:    "<p>Hello World</p>",
		},
		IsRead:         true,
		HasAttachments: true,
		Importance:     "High",
		DateTimeSent:   "2025-01-24T12:00:00Z",
		Attachments: []Attachment{
			{ID: "att1", Name: "file.pdf", ContentType: "application/pdf", Size: 12345},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Message
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", parsed.ID, original.ID)
	}
	if parsed.Subject != original.Subject {
		t.Errorf("Subject mismatch: got %q, want %q", parsed.Subject, original.Subject)
	}
	if parsed.From == nil || parsed.From.Address != original.From.Address {
		t.Error("From address mismatch")
	}
	if len(parsed.ToRecipients) != 2 {
		t.Errorf("ToRecipients count = %d, want 2", len(parsed.ToRecipients))
	}
	if len(parsed.Attachments) != 1 {
		t.Errorf("Attachments count = %d, want 1", len(parsed.Attachments))
	}
	if parsed.Body == nil || parsed.Body.BodyType != "HTML" {
		t.Error("Body mismatch")
	}
}

func TestConversationJSONRoundTrip(t *testing.T) {
	original := Conversation{
		ID:               "conv-123",
		Topic:            "Discussion Thread",
		MessageCount:     5,
		UnreadCount:      2,
		HasAttachments:   true,
		LastDeliveryTime: "2025-01-24T15:00:00Z",
		UniqueRecipients: []string{"user1@example.com", "user2@example.com"},
		UniqueSenders:    []string{"sender@example.com"},
		Messages: []Message{
			{ID: "msg1", Subject: "First"},
			{ID: "msg2", Subject: "Re: First"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Conversation
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.ID != original.ID {
		t.Errorf("ID mismatch")
	}
	if parsed.Topic != original.Topic {
		t.Errorf("Topic mismatch")
	}
	if parsed.MessageCount != original.MessageCount {
		t.Errorf("MessageCount = %d, want %d", parsed.MessageCount, original.MessageCount)
	}
	if len(parsed.Messages) != 2 {
		t.Errorf("Messages count = %d, want 2", len(parsed.Messages))
	}
}

func TestDraftJSONSerialization(t *testing.T) {
	draft := Draft{
		Subject: "Test Draft",
		Body: &MessageBody{
			BodyType: "Text",
			Value:    "Plain text body content",
		},
		ToRecipients: []EmailAddress{
			{Name: "To User", Address: "to@example.com"},
		},
		CcRecipients: []EmailAddress{
			{Address: "cc@example.com"},
		},
		BccRecipients: []EmailAddress{
			{Address: "bcc@example.com"},
		},
		Importance:      "Normal",
		SaveToSentItems: true,
	}

	data, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["Subject"] != "Test Draft" {
		t.Errorf("Subject = %v, want Test Draft", parsed["Subject"])
	}
	if parsed["Importance"] != "Normal" {
		t.Errorf("Importance = %v, want Normal", parsed["Importance"])
	}
}

func TestFolderJSONRoundTrip(t *testing.T) {
	folder := Folder{
		ID:               "folder-abc",
		DisplayName:      "Important",
		ParentFolderID:   "parent-123",
		ChildFolderCount: 3,
		TotalCount:       150,
		UnreadCount:      12,
		FolderClass:      "IPF.Note",
	}

	data, err := json.Marshal(folder)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Folder
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.ID != folder.ID {
		t.Errorf("ID mismatch")
	}
	if parsed.DisplayName != folder.DisplayName {
		t.Errorf("DisplayName mismatch")
	}
	if parsed.TotalCount != folder.TotalCount {
		t.Errorf("TotalCount = %d, want %d", parsed.TotalCount, folder.TotalCount)
	}
}

func TestAttachmentJSONRoundTrip(t *testing.T) {
	attachment := Attachment{
		ID:          "att-xyz",
		Name:        "document.docx",
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Size:        98765,
		IsInline:    false,
		ContentID:   "cid:12345",
	}

	data, err := json.Marshal(attachment)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Attachment
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.ID != attachment.ID {
		t.Errorf("ID mismatch")
	}
	if parsed.Size != attachment.Size {
		t.Errorf("Size = %d, want %d", parsed.Size, attachment.Size)
	}
}

func TestSearchResultFromRealOWAResponse(t *testing.T) {
	// Simulated realistic OWA FindItem response
	// Note: ItemId is a string in our model, not an object
	owaResponse := `{
		"Body": {
			"Items": [
				{
					"ItemId": "AAMkAGI2TG93AAA=",
					"Subject": "Meeting Reminder",
					"From": {
						"Name": "John Doe",
						"EmailAddress": "john@example.com",
						"RoutingType": "SMTP"
					},
					"DateTimeReceived": "2025-01-24T10:30:00Z",
					"IsRead": false,
					"HasAttachments": true,
					"Importance": "High"
				},
				{
					"ItemId": "AAMkAGI2TG93BBB=",
					"Subject": "Weekly Report",
					"From": {
						"Name": "Jane Smith",
						"EmailAddress": "jane@example.com"
					},
					"DateTimeReceived": "2025-01-24T09:15:00Z",
					"IsRead": true,
					"HasAttachments": false
				}
			],
			"TotalItemsInView": 42
		}
	}`

	result, err := UnmarshalSearchResponse(json.RawMessage(owaResponse))
	if err != nil {
		t.Fatalf("UnmarshalSearchResponse failed: %v", err)
	}

	if result.TotalCount != 42 {
		t.Errorf("TotalCount = %d, want 42", result.TotalCount)
	}
	if len(result.Messages) != 2 {
		t.Errorf("Messages count = %d, want 2", len(result.Messages))
	}
}
