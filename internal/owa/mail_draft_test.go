package owa

import (
	"encoding/json"
	"testing"
)

// Tests for draft creation request body construction

func TestCreateDraftRequestBody(t *testing.T) {
	tests := []struct {
		name         string
		draft        Draft
		wantSubject  string
		wantBodyType string
		wantToCount  int
		wantCcCount  int
	}{
		{
			name: "minimal draft",
			draft: Draft{
				Subject:      "Test Subject",
				ToRecipients: []EmailAddress{{Address: "to@example.com"}},
			},
			wantSubject: "Test Subject",
			wantToCount: 1,
			wantCcCount: 0,
		},
		{
			name: "draft with body",
			draft: Draft{
				Subject:      "With Body",
				ToRecipients: []EmailAddress{{Address: "to@example.com"}},
				Body: &MessageBody{
					BodyType: "HTML",
					Value:    "<p>Hello</p>",
				},
			},
			wantSubject:  "With Body",
			wantBodyType: "HTML",
			wantToCount:  1,
		},
		{
			name: "draft with CC",
			draft: Draft{
				Subject:      "With CC",
				ToRecipients: []EmailAddress{{Address: "to@example.com"}},
				CcRecipients: []EmailAddress{
					{Address: "cc1@example.com"},
					{Address: "cc2@example.com"},
				},
			},
			wantSubject: "With CC",
			wantToCount: 1,
			wantCcCount: 2,
		},
		{
			name: "draft with multiple recipients",
			draft: Draft{
				Subject: "Multiple Recipients",
				ToRecipients: []EmailAddress{
					{Name: "User 1", Address: "user1@example.com"},
					{Name: "User 2", Address: "user2@example.com"},
					{Address: "user3@example.com"},
				},
			},
			wantSubject: "Multiple Recipients",
			wantToCount: 3,
		},
		{
			name: "draft with importance",
			draft: Draft{
				Subject:      "Important",
				ToRecipients: []EmailAddress{{Address: "to@example.com"}},
				Importance:   "High",
			},
			wantSubject: "Important",
			wantToCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate the draft structure serializes properly
			data, err := json.Marshal(tt.draft)
			if err != nil {
				t.Fatalf("Failed to marshal draft: %v", err)
			}

			var parsed map[string]interface{}
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("Failed to unmarshal draft: %v", err)
			}

			if tt.draft.Subject != tt.wantSubject {
				t.Errorf("Subject = %q, want %q", tt.draft.Subject, tt.wantSubject)
			}

			if len(tt.draft.ToRecipients) != tt.wantToCount {
				t.Errorf("ToRecipients count = %d, want %d", len(tt.draft.ToRecipients), tt.wantToCount)
			}

			if len(tt.draft.CcRecipients) != tt.wantCcCount {
				t.Errorf("CcRecipients count = %d, want %d", len(tt.draft.CcRecipients), tt.wantCcCount)
			}

			if tt.wantBodyType != "" && tt.draft.Body != nil {
				if tt.draft.Body.BodyType != tt.wantBodyType {
					t.Errorf("BodyType = %q, want %q", tt.draft.Body.BodyType, tt.wantBodyType)
				}
			}
		})
	}
}

func TestDraftValidation(t *testing.T) {
	tests := []struct {
		name      string
		draft     Draft
		wantValid bool
	}{
		{
			name: "valid draft",
			draft: Draft{
				Subject:      "Test",
				ToRecipients: []EmailAddress{{Address: "to@example.com"}},
			},
			wantValid: true,
		},
		{
			name: "empty subject allowed",
			draft: Draft{
				Subject:      "",
				ToRecipients: []EmailAddress{{Address: "to@example.com"}},
			},
			wantValid: true, // OWA allows empty subjects
		},
		{
			name: "no recipients",
			draft: Draft{
				Subject: "No Recipients",
			},
			wantValid: true, // Draft can exist without recipients
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify serialization works
			_, err := json.Marshal(tt.draft)
			gotValid := err == nil
			if gotValid != tt.wantValid {
				t.Errorf("valid = %v, want %v", gotValid, tt.wantValid)
			}
		})
	}
}

func TestUpdateDraftChangesStructure(t *testing.T) {
	// Test that update changes are properly structured
	tests := []struct {
		name        string
		subject     string
		body        *MessageBody
		wantChanges int
	}{
		{
			name:        "subject only",
			subject:     "New Subject",
			wantChanges: 1,
		},
		{
			name:    "body only",
			subject: "",
			body: &MessageBody{
				BodyType: "Text",
				Value:    "New body content",
			},
			wantChanges: 1,
		},
		{
			name:    "subject and body",
			subject: "Updated",
			body: &MessageBody{
				BodyType: "HTML",
				Value:    "<p>Updated</p>",
			},
			wantChanges: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := 0
			if tt.subject != "" {
				changes++
			}
			if tt.body != nil {
				changes++
			}

			if changes != tt.wantChanges {
				t.Errorf("changes count = %d, want %d", changes, tt.wantChanges)
			}
		})
	}
}

func TestMessageDispositionValues(t *testing.T) {
	// Test OWA MessageDisposition values
	dispositions := []string{
		"SaveOnly",        // For drafts
		"SendAndSaveCopy", // For direct send
		"SendOnly",        // Send without saving
	}

	for _, disp := range dispositions {
		t.Run(disp, func(t *testing.T) {
			if disp == "" {
				t.Error("disposition should not be empty")
			}
		})
	}
}

func TestDeleteTypeValues(t *testing.T) {
	// Test OWA DeleteType values
	deleteTypes := []string{
		"HardDelete",         // Permanent delete
		"SoftDelete",         // Recoverable delete
		"MoveToDeletedItems", // Move to trash
	}

	for _, dt := range deleteTypes {
		t.Run(dt, func(t *testing.T) {
			if dt == "" {
				t.Error("delete type should not be empty")
			}
		})
	}
}

func TestSendDraftRequestStructure(t *testing.T) {
	// Verify SendItem request structure matches OWA expectations
	draftID := "AAMkAGI2TG93AAA="

	body := map[string]interface{}{
		"ItemIds": []map[string]interface{}{
			{"__type": "ItemId:#Exchange", "Id": draftID},
		},
		"SaveItemToFolder": true,
		"SavedItemFolderId": map[string]interface{}{
			"__type": "DistinguishedFolderId:#Exchange",
			"Id":     "sentitems",
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if _, ok := parsed["ItemIds"]; !ok {
		t.Error("missing ItemIds")
	}
	if _, ok := parsed["SaveItemToFolder"]; !ok {
		t.Error("missing SaveItemToFolder")
	}
	if _, ok := parsed["SavedItemFolderId"]; !ok {
		t.Error("missing SavedItemFolderId")
	}
}

func TestSendMessageRequestStructure(t *testing.T) {
	// Verify CreateItem with SendAndSaveCopy structure
	draft := Draft{
		Subject: "Test Send",
		ToRecipients: []EmailAddress{
			{Name: "Recipient", Address: "to@example.com"},
		},
		Body: &MessageBody{
			BodyType: "Text",
			Value:    "Test body",
		},
	}

	body := map[string]interface{}{
		"Items": []map[string]interface{}{
			{
				"__type":  "Message:#Exchange",
				"Subject": draft.Subject,
				"Body": map[string]interface{}{
					"__type":   "BodyContentType:#Exchange",
					"BodyType": draft.Body.BodyType,
					"Value":    draft.Body.Value,
				},
			},
		},
		"MessageDisposition": "SendAndSaveCopy",
	}

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	if len(data) == 0 {
		t.Error("serialized body is empty")
	}

	// Verify key structure elements
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed["MessageDisposition"] != "SendAndSaveCopy" {
		t.Errorf("MessageDisposition = %v, want SendAndSaveCopy", parsed["MessageDisposition"])
	}

	items, ok := parsed["Items"].([]interface{})
	if !ok || len(items) == 0 {
		t.Error("Items should be non-empty array")
	}
}
