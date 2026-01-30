package owa

import "testing"

func TestNormalizeFolderName(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"Inbox", "inbox", true},
		{"Sent Items", "sentitems", true},
		{"trash", "deleteditems", true},
		{"Junk-Mail", "junkemail", true},
		{"archive", "archive", true},
		{"custom-folder-id", "", false},
	}

	for _, tt := range tests {
		got, ok := normalizeFolderName(tt.input)
		if ok != tt.ok {
			t.Fatalf("normalizeFolderName(%q) ok=%v, want %v", tt.input, ok, tt.ok)
		}
		if got != tt.want {
			t.Fatalf("normalizeFolderName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveFolderInputUsesCache(t *testing.T) {
	tokens := &Tokens{Folders: map[string]string{"inbox": "inbox-id"}}
	got, err := resolveFolderInput(nil, tokens, "Inbox")
	if err != nil {
		t.Fatalf("resolveFolderInput error: %v", err)
	}
	if got != "inbox-id" {
		t.Fatalf("resolveFolderInput = %q, want inbox-id", got)
	}

	got, err = resolveFolderInput(nil, tokens, "folder-123")
	if err != nil {
		t.Fatalf("resolveFolderInput error: %v", err)
	}
	if got != "folder-123" {
		t.Fatalf("resolveFolderInput passthrough = %q, want folder-123", got)
	}
}
