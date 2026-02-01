package owa

import (
	"net/url"
	"testing"
)

func TestBuildSubstrateConversationURL(t *testing.T) {
	urlStr, err := buildSubstrateConversationURL("conv-1", 10)
	if err != nil {
		t.Fatalf("buildSubstrateConversationURL error: %v", err)
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("url.Parse error: %v", err)
	}
	query := parsed.Query()
	if query.Get("$top") != "10" {
		t.Fatalf("$top = %q", query.Get("$top"))
	}
	if query.Get("$select") != "sender,sentDateTime,toRecipients" {
		t.Fatalf("$select = %q", query.Get("$select"))
	}
	wantFilter := "(isDraft eq false and conversationId eq 'conv-1')"
	if query.Get("$filter") != wantFilter {
		t.Fatalf("$filter = %q", query.Get("$filter"))
	}
}

func TestParseSubstrateMessageIDs(t *testing.T) {
	body := []byte(`{"value":[{"Id":"id-1"},{"Id":"id-2"}]}`)
	ids, err := parseSubstrateMessageIDs(body)
	if err != nil {
		t.Fatalf("parseSubstrateMessageIDs error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("ids = %d, want 2", len(ids))
	}
	if ids[0] != "id-1" || ids[1] != "id-2" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestParseSubstrateMessageIDsLowercase(t *testing.T) {
	body := []byte(`{"value":[{"id":"id-3"}]}`)
	ids, err := parseSubstrateMessageIDs(body)
	if err != nil {
		t.Fatalf("parseSubstrateMessageIDs error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "id-3" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestParseSubstrateMessageIDsEmpty(t *testing.T) {
	if _, err := parseSubstrateMessageIDs(nil); err == nil {
		t.Fatalf("expected error")
	}
}
