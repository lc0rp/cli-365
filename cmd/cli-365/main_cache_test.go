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
			{ID: "id-1", Subject: "One", From: &owa.EmailAddress{Address: "a@example.com"}},
			{ID: "id-2", Subject: "Two", From: &owa.EmailAddress{Name: "Bob", Address: "b@example.com"}},
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
	if filepath.Dir(path) == "" {
		t.Fatalf("cache path should include directory")
	}
}
