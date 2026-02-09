package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lc0rp/cli-365/internal/owa"
)

func TestParseIndexArg(t *testing.T) {
	idx, err := parseIndexArg("#3")
	if err != nil {
		t.Fatalf("parseIndexArg error: %v", err)
	}
	if idx != 3 {
		t.Fatalf("index = %d, want 3", idx)
	}
	if _, err := parseIndexArg("3"); err == nil {
		t.Fatalf("expected error for missing #")
	}
}

func TestCachedSearchRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	result := &owa.SearchResult{
		Messages: []owa.Message{
			{ID: "id-1", ConversationID: "conv-1", ParentFolderId: "folder-1", Subject: "One", From: &owa.EmailAddress{Address: "a@example.com"}},
			{ID: "id-2", ConversationID: "conv-2", ParentFolderId: "folder-2", Subject: "Two", From: &owa.EmailAddress{Name: "Bob", Address: "b@example.com"}},
		},
	}
	if err := saveLastSearch("test", result); err != nil {
		t.Fatalf("saveLastSearch error: %v", err)
	}
	path := lastSearchPath()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
	id, err := resolveCachedMessageID(2)
	if err != nil {
		t.Fatalf("resolveCachedMessageID error: %v", err)
	}
	if id != "id-2" {
		t.Fatalf("id = %q, want id-2", id)
	}
	msg, err := resolveCachedMessage(1)
	if err != nil {
		t.Fatalf("resolveCachedMessage error: %v", err)
	}
	if msg.ConversationID != "conv-1" {
		t.Fatalf("ConversationID = %q, want conv-1", msg.ConversationID)
	}
	if msg.ParentFolderID != "folder-1" {
		t.Fatalf("ParentFolderID = %q, want folder-1", msg.ParentFolderID)
	}
	if filepath.Dir(path) == "" {
		t.Fatalf("cache path should include directory")
	}
}

func TestParseTrailingIntFlag(t *testing.T) {
	value, ok := parseTrailingIntFlag([]string{"test", "--limit", "5"}, []string{"--limit", "-n"})
	if !ok || value != 5 {
		t.Fatalf("parseTrailingIntFlag value=%d ok=%v", value, ok)
	}
	value, ok = parseTrailingIntFlag([]string{"test", "-n", "7"}, []string{"--limit", "-n"})
	if !ok || value != 7 {
		t.Fatalf("parseTrailingIntFlag short value=%d ok=%v", value, ok)
	}
	value, ok = parseTrailingIntFlag([]string{"test", "--limit=9"}, []string{"--limit", "-n"})
	if !ok || value != 9 {
		t.Fatalf("parseTrailingIntFlag eq value=%d ok=%v", value, ok)
	}
	if _, ok = parseTrailingIntFlag([]string{"test", "--limit", "x"}, []string{"--limit", "-n"}); ok {
		t.Fatalf("expected parse failure for non-int")
	}
	if _, ok = parseTrailingIntFlag([]string{"test", "--limit"}, []string{"--limit", "-n"}); ok {
		t.Fatalf("expected parse failure for missing value")
	}
}
