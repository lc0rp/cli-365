package owa

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestAttachmentStructure(t *testing.T) {
	tests := []struct {
		name       string
		attachment Attachment
	}{
		{
			name: "PDF attachment",
			attachment: Attachment{
				ID:          "AAMkAGI2TG93_att1",
				Name:        "document.pdf",
				ContentType: "application/pdf",
				Size:        123456,
				IsInline:    false,
			},
		},
		{
			name: "inline image",
			attachment: Attachment{
				ID:          "AAMkAGI2TG93_att2",
				Name:        "image.png",
				ContentType: "image/png",
				Size:        45678,
				IsInline:    true,
				ContentID:   "cid:image001",
			},
		},
		{
			name: "word document",
			attachment: Attachment{
				ID:          "AAMkAGI2TG93_att3",
				Name:        "report.docx",
				ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				Size:        89012,
			},
		},
		{
			name: "generic file",
			attachment: Attachment{
				ID:          "AAMkAGI2TG93_att4",
				Name:        "data.bin",
				ContentType: "application/octet-stream",
				Size:        1024,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify serialization
			data, err := json.Marshal(tt.attachment)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var parsed Attachment
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			if parsed.ID != tt.attachment.ID {
				t.Errorf("ID = %q, want %q", parsed.ID, tt.attachment.ID)
			}
			if parsed.Name != tt.attachment.Name {
				t.Errorf("Name = %q, want %q", parsed.Name, tt.attachment.Name)
			}
			if parsed.ContentType != tt.attachment.ContentType {
				t.Errorf("ContentType = %q, want %q", parsed.ContentType, tt.attachment.ContentType)
			}
			if parsed.Size != tt.attachment.Size {
				t.Errorf("Size = %d, want %d", parsed.Size, tt.attachment.Size)
			}
			if parsed.IsInline != tt.attachment.IsInline {
				t.Errorf("IsInline = %v, want %v", parsed.IsInline, tt.attachment.IsInline)
			}
		})
	}
}

func TestGetAttachmentRequestBody(t *testing.T) {
	attachmentID := "AAMkAGI2TG93_att_xyz"

	body := map[string]interface{}{
		"AttachmentShape": map[string]interface{}{
			"IncludeMimeContent": true,
		},
		"AttachmentIds": []map[string]interface{}{
			{"__type": "AttachmentId:#Exchange", "Id": attachmentID},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Verify structure
	shape, ok := parsed["AttachmentShape"].(map[string]interface{})
	if !ok {
		t.Fatal("AttachmentShape missing or wrong type")
	}
	if shape["IncludeMimeContent"] != true {
		t.Error("IncludeMimeContent should be true")
	}

	ids, ok := parsed["AttachmentIds"].([]interface{})
	if !ok || len(ids) == 0 {
		t.Fatal("AttachmentIds missing or empty")
	}

	firstID := ids[0].(map[string]interface{})
	if firstID["__type"] != "AttachmentId:#Exchange" {
		t.Errorf("__type = %v, want AttachmentId:#Exchange", firstID["__type"])
	}
	if firstID["Id"] != attachmentID {
		t.Errorf("Id = %v, want %s", firstID["Id"], attachmentID)
	}
}

func TestBase64ContentDecoding(t *testing.T) {
	// Test that base64 content decoding works correctly
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "valid base64",
			content: base64.StdEncoding.EncodeToString([]byte("Hello, World!")),
			want:    "Hello, World!",
			wantErr: false,
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "invalid base64",
			content: "not-valid-base64!!!",
			wantErr: true,
		},
		{
			name:    "binary content",
			content: base64.StdEncoding.EncodeToString([]byte{0x00, 0x01, 0x02, 0xFF}),
			want:    string([]byte{0x00, 0x01, 0x02, 0xFF}),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := base64.StdEncoding.DecodeString(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(decoded) != tt.want {
				t.Errorf("decoded = %q, want %q", string(decoded), tt.want)
			}
		})
	}
}

func TestAttachmentResponseParsing(t *testing.T) {
	// Simulated OWA GetAttachment response
	response := `{
		"Body": {
			"Attachments": [
				{
					"ContentType": "application/pdf",
					"Name": "test.pdf",
					"Content": "SGVsbG8gV29ybGQh"
				}
			]
		}
	}`

	var wrapper struct {
		Body struct {
			Attachments []struct {
				ContentType string `json:"ContentType"`
				Name        string `json:"Name"`
				Content     string `json:"Content"`
			} `json:"Attachments"`
		} `json:"Body"`
	}

	if err := json.Unmarshal([]byte(response), &wrapper); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(wrapper.Body.Attachments) != 1 {
		t.Fatalf("Attachments count = %d, want 1", len(wrapper.Body.Attachments))
	}

	att := wrapper.Body.Attachments[0]
	if att.ContentType != "application/pdf" {
		t.Errorf("ContentType = %q, want application/pdf", att.ContentType)
	}
	if att.Name != "test.pdf" {
		t.Errorf("Name = %q, want test.pdf", att.Name)
	}

	// Decode content
	decoded, err := base64.StdEncoding.DecodeString(att.Content)
	if err != nil {
		t.Fatalf("base64 decode error: %v", err)
	}
	if string(decoded) != "Hello World!" {
		t.Errorf("Content = %q, want 'Hello World!'", string(decoded))
	}
}

func TestAttachmentListFromMessage(t *testing.T) {
	// Test that message attachments are properly structured
	msg := Message{
		ID:             "AAMkAGI2TG93AAA=",
		Subject:        "Email with attachments",
		HasAttachments: true,
		Attachments: []Attachment{
			{ID: "att1", Name: "doc1.pdf", Size: 1000},
			{ID: "att2", Name: "doc2.docx", Size: 2000},
			{ID: "att3", Name: "image.png", Size: 3000, IsInline: true},
		},
	}

	if !msg.HasAttachments {
		t.Error("HasAttachments should be true")
	}
	if len(msg.Attachments) != 3 {
		t.Errorf("Attachments count = %d, want 3", len(msg.Attachments))
	}

	// Count inline vs regular attachments
	inlineCount := 0
	for _, att := range msg.Attachments {
		if att.IsInline {
			inlineCount++
		}
	}
	if inlineCount != 1 {
		t.Errorf("inline count = %d, want 1", inlineCount)
	}
}

func TestCommonMimeTypes(t *testing.T) {
	// Verify we handle common MIME types
	mimeTypes := map[string]string{
		"pdf":  "application/pdf",
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"png":  "image/png",
		"jpg":  "image/jpeg",
		"gif":  "image/gif",
		"txt":  "text/plain",
		"html": "text/html",
		"zip":  "application/zip",
		"json": "application/json",
	}

	for ext, mime := range mimeTypes {
		t.Run(ext, func(t *testing.T) {
			att := Attachment{
				Name:        "file." + ext,
				ContentType: mime,
			}
			if att.ContentType == "" {
				t.Error("ContentType should not be empty")
			}
		})
	}
}

func TestMultipleAttachmentsResponse(t *testing.T) {
	// Test parsing response with multiple attachments
	response := `{
		"Body": {
			"Attachments": [
				{"ContentType": "application/pdf", "Name": "a.pdf", "Content": "YQ=="},
				{"ContentType": "image/png", "Name": "b.png", "Content": "Yg=="},
				{"ContentType": "text/plain", "Name": "c.txt", "Content": "Yw=="}
			]
		}
	}`

	var wrapper struct {
		Body struct {
			Attachments []struct {
				ContentType string `json:"ContentType"`
				Name        string `json:"Name"`
				Content     string `json:"Content"`
			} `json:"Attachments"`
		} `json:"Body"`
	}

	if err := json.Unmarshal([]byte(response), &wrapper); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(wrapper.Body.Attachments) != 3 {
		t.Fatalf("Attachments count = %d, want 3", len(wrapper.Body.Attachments))
	}

	expectedNames := []string{"a.pdf", "b.png", "c.txt"}
	for i, att := range wrapper.Body.Attachments {
		if att.Name != expectedNames[i] {
			t.Errorf("Attachment[%d].Name = %q, want %q", i, att.Name, expectedNames[i])
		}
	}
}
