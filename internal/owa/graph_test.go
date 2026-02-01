package owa

import (
	"net/url"
	"testing"
)

func TestBuildGraphConversationURL(t *testing.T) {
	urlStr, err := buildGraphConversationURL("conv-graph", 5)
	if err != nil {
		t.Fatalf("buildGraphConversationURL error: %v", err)
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("url.Parse error: %v", err)
	}
	query := parsed.Query()
	if query.Get("$top") != "5" {
		t.Fatalf("$top = %q", query.Get("$top"))
	}
	wantFilter := "isDraft eq false and conversationId eq 'conv-graph'"
	if query.Get("$filter") != wantFilter {
		t.Fatalf("$filter = %q", query.Get("$filter"))
	}
}
