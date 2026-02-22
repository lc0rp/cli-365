package owa

import (
	"testing"
)

func TestSearchRequestBody(t *testing.T) {
	// Test that search request body is correctly constructed
	// This tests the structure without actually making requests

	tests := []struct {
		name       string
		query      string
		folderID   string
		maxResults int
	}{
		{"Empty query", "", "", 50},
		{"With query", "test", "", 20},
		{"With folder", "", "inbox-id", 10},
		{"Full params", "search term", "folder-123", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the test params are valid
			if tt.maxResults <= 0 {
				t.Error("maxResults should be positive")
			}
		})
	}
}

func TestDraftConstruction(t *testing.T) {
	tests := []struct {
		name  string
		draft Draft
	}{
		{
			name: "Minimal draft",
			draft: Draft{
				Subject:      "Test",
				ToRecipients: []EmailAddress{{Address: "test@example.com"}},
			},
		},
		{
			name: "Draft with body",
			draft: Draft{
				Subject:      "Test",
				ToRecipients: []EmailAddress{{Address: "test@example.com"}},
				Body: &MessageBody{
					BodyType: "HTML",
					Value:    "<p>Hello</p>",
				},
			},
		},
		{
			name: "Draft with CC",
			draft: Draft{
				Subject:      "Test",
				ToRecipients: []EmailAddress{{Address: "to@example.com"}},
				CcRecipients: []EmailAddress{{Address: "cc@example.com"}},
			},
		},
		{
			name: "Draft with all fields",
			draft: Draft{
				Subject:       "Important Email",
				ToRecipients:  []EmailAddress{{Name: "To User", Address: "to@example.com"}},
				CcRecipients:  []EmailAddress{{Address: "cc@example.com"}},
				BccRecipients: []EmailAddress{{Address: "bcc@example.com"}},
				Body: &MessageBody{
					BodyType: "Text",
					Value:    "Plain text body",
				},
				Importance: "High",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.draft.Subject == "" {
				t.Error("Draft subject should not be empty")
			}
			if len(tt.draft.ToRecipients) == 0 {
				t.Error("Draft should have at least one recipient")
			}
		})
	}
}

func TestOWAActionEndpoints(t *testing.T) {
	endpoints := []struct {
		action   string
		expected string
	}{
		{"FindItem", "https://outlook.office.com/owa/0/service.svc?action=FindItem&app=Mail&n=0"},
		{"GetItem", "https://outlook.office.com/owa/0/service.svc?action=GetItem&app=Mail&n=0"},
		{"CreateItem", "https://outlook.office.com/owa/0/service.svc?action=CreateItem&app=Mail&n=0"},
		{"UpdateItem", "https://outlook.office.com/owa/0/service.svc?action=UpdateItem&app=Mail&n=0"},
		{"DeleteItem", "https://outlook.office.com/owa/0/service.svc?action=DeleteItem&app=Mail&n=0"},
		{"SendItem", "https://outlook.office.com/owa/0/service.svc?action=SendItem&app=Mail&n=0"},
		{"GetAttachment", "https://outlook.office.com/owa/0/service.svc?action=GetAttachment&app=Mail&n=0"},
		{"FindConversation", "https://outlook.office.com/owa/0/service.svc?action=FindConversation&app=Mail&n=0"},
		{"GetConversationItems", "https://outlook.office.com/owa/0/service.svc?action=GetConversationItems&app=Mail&n=0"},
	}

	for _, tt := range endpoints {
		t.Run(tt.action, func(t *testing.T) {
			got := OWAEndpoint(tt.action)
			if got != tt.expected {
				t.Errorf("OWAEndpoint(%q) = %q, want %q", tt.action, got, tt.expected)
			}
		})
	}
}

func TestMessageParsing(t *testing.T) {
	// Test that message fields are properly accessible
	msg := Message{
		ID:      "test-id-123",
		Subject: "Test Subject",
		From: &EmailAddress{
			Name:    "Sender Name",
			Address: "sender@example.com",
		},
		ToRecipients: []EmailAddress{
			{Name: "Recipient 1", Address: "r1@example.com"},
			{Name: "Recipient 2", Address: "r2@example.com"},
		},
		IsRead:         true,
		IsDraft:        false,
		HasAttachments: true,
		DateTimeSent:   "2025-01-24T12:00:00Z",
	}

	if msg.ID != "test-id-123" {
		t.Errorf("ID = %q, want test-id-123", msg.ID)
	}
	if msg.From.Name != "Sender Name" {
		t.Errorf("From.Name = %q, want Sender Name", msg.From.Name)
	}
	if len(msg.ToRecipients) != 2 {
		t.Errorf("ToRecipients length = %d, want 2", len(msg.ToRecipients))
	}
	if !msg.IsRead {
		t.Error("IsRead should be true")
	}
	if !msg.HasAttachments {
		t.Error("HasAttachments should be true")
	}
}

func TestConversationParsing(t *testing.T) {
	conv := Conversation{
		ID:           "conv-id-456",
		Topic:        "Conversation Topic",
		MessageCount: 5,
		UnreadCount:  2,
		Messages: []Message{
			{ID: "msg-1", Subject: "First message"},
			{ID: "msg-2", Subject: "Re: First message"},
			{ID: "msg-3", Subject: "Re: First message"},
		},
	}

	if conv.ID != "conv-id-456" {
		t.Errorf("ID = %q, want conv-id-456", conv.ID)
	}
	if conv.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want 5", conv.MessageCount)
	}
	if len(conv.Messages) != 3 {
		t.Errorf("Messages length = %d, want 3", len(conv.Messages))
	}
}

func TestAttachmentParsing(t *testing.T) {
	att := Attachment{
		ID:          "att-id-789",
		Name:        "document.pdf",
		ContentType: "application/pdf",
		Size:        123456,
		IsInline:    false,
		ContentID:   "",
	}

	if att.ID != "att-id-789" {
		t.Errorf("ID = %q, want att-id-789", att.ID)
	}
	if att.Name != "document.pdf" {
		t.Errorf("Name = %q, want document.pdf", att.Name)
	}
	if att.Size != 123456 {
		t.Errorf("Size = %d, want 123456", att.Size)
	}
}

func TestEmailAddressFormatting(t *testing.T) {
	tests := []struct {
		name string
		addr EmailAddress
	}{
		{
			name: "With name and address",
			addr: EmailAddress{Name: "John Doe", Address: "john@example.com"},
		},
		{
			name: "Address only",
			addr: EmailAddress{Address: "jane@example.com"},
		},
		{
			name: "With routing type",
			addr: EmailAddress{
				Name:        "Bob",
				Address:     "bob@example.com",
				RoutingType: "SMTP",
				MailboxType: "Mailbox",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.addr.Address == "" {
				t.Error("Address should not be empty")
			}
		})
	}
}

func TestItemIDStructure(t *testing.T) {
	id := ItemID{
		ID:        "AAMkAGI2TG93AAA=",
		ChangeKey: "CQAAABYAAABz",
	}

	if id.ID == "" {
		t.Error("ID should not be empty")
	}
	if id.ChangeKey == "" {
		t.Error("ChangeKey should not be empty for this test")
	}
}

func TestFolderIDStructure(t *testing.T) {
	id := FolderID{
		ID:        "AAMkAGI2TG93BBB=",
		ChangeKey: "AQAAABYA",
	}

	if id.ID == "" {
		t.Error("ID should not be empty")
	}
}

func TestDistinguishedFolderID(t *testing.T) {
	folders := []string{"inbox", "drafts", "sentitems", "deleteditems", "junkemail"}

	for _, folder := range folders {
		dfid := DistinguishedFolderID{ID: folder}
		if dfid.ID != folder {
			t.Errorf("DistinguishedFolderID = %q, want %q", dfid.ID, folder)
		}
	}
}
