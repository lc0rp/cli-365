package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/lc0rp/cli-365/internal/owa"
)

func TestCalendarListHelpIncludesCalendarSelectorFlag(t *testing.T) {
	app := newCLIApp(cliAppOptions{DisableDaemonForwarding: true})
	var out bytes.Buffer
	app.Writer = &out
	app.ErrWriter = &out

	code := runCLIApp(context.Background(), app, []string{"cli-365", "calendar", "list", "--help"})
	if code != 0 {
		t.Fatalf("exit code = %d, output = %s", code, out.String())
	}
	if !strings.Contains(out.String(), "--calendar value") {
		t.Fatalf("help output missing --calendar flag: %s", out.String())
	}
}

func TestResolveCalendarDirectoryIdentity(t *testing.T) {
	tests := []struct {
		name      string
		emailFlag string
		nameFlag  string
		arg       string
		wantEmail string
		wantName  string
		wantErr   bool
	}{
		{
			name:      "email flag",
			emailFlag: "alice@example.com",
			wantEmail: "alice@example.com",
		},
		{
			name:     "name flag",
			nameFlag: "Alice Doe",
			wantName: "Alice Doe",
		},
		{
			name:      "positional email",
			arg:       "bob@example.com",
			wantEmail: "bob@example.com",
		},
		{
			name:     "positional name",
			arg:      "Bob Doe",
			wantName: "Bob Doe",
		},
		{
			name:      "both flags rejected",
			emailFlag: "a@example.com",
			nameFlag:  "Alice",
			wantErr:   true,
		},
		{
			name:      "flag plus arg rejected",
			emailFlag: "a@example.com",
			arg:       "b@example.com",
			wantErr:   true,
		},
		{
			name:    "missing identity rejected",
			wantErr: true,
		},
		{
			name:      "invalid email rejected",
			emailFlag: "alice",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEmail, gotName, err := resolveCalendarDirectoryIdentity(tt.emailFlag, tt.nameFlag, tt.arg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveCalendarDirectoryIdentity() err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if gotEmail != tt.wantEmail {
				t.Fatalf("email = %q, want %q", gotEmail, tt.wantEmail)
			}
			if gotName != tt.wantName {
				t.Fatalf("name = %q, want %q", gotName, tt.wantName)
			}
		})
	}
}

func TestFindExistingAddedDirectoryCalendarMatchesLiveFolderName(t *testing.T) {
	folders := []owa.CalendarFolder{
		{DisplayName: "Example User", FolderID: "folder-sample"},
		{DisplayName: "Calendar", FolderID: "folder-default"},
	}

	record, ok := findExistingAddedDirectoryCalendar(nil, folders, "", "example user")
	if !ok {
		t.Fatal("expected existing calendar match")
	}
	if record.FolderID != "folder-sample" {
		t.Fatalf("FolderID = %q, want folder-sample", record.FolderID)
	}
	if record.CalendarName != "Example User" {
		t.Fatalf("CalendarName = %q, want Example User", record.CalendarName)
	}
}

func TestFindExistingAddedDirectoryCalendarPrefersRegistryByEmail(t *testing.T) {
	folders := []owa.CalendarFolder{
		{DisplayName: "Example User", FolderID: "folder-sample"},
	}
	records := []addedDirectoryCalendarRecord{
		{
			Email:        "sample.user@example.com",
			CalendarName: "Example User",
			FolderID:     "folder-sample",
			CalendarID:   "calendar-sample",
		},
	}

	record, ok := findExistingAddedDirectoryCalendar(records, folders, "sample.user@example.com", "")
	if !ok {
		t.Fatal("expected registry email match")
	}
	if record.CalendarID != "calendar-sample" {
		t.Fatalf("CalendarID = %q, want calendar-sample", record.CalendarID)
	}
}

func TestFindExistingAddedDirectoryCalendarMatchesRegistryWithoutActiveFolder(t *testing.T) {
	folders := []owa.CalendarFolder{
		{DisplayName: "Calendar", FolderID: "folder-default"},
	}
	records := []addedDirectoryCalendarRecord{
		{
			Email:        "user@example.com",
			CalendarName: "Example User",
			FolderID:     "folder-sample-linked",
			CalendarID:   "folder-sample-linked",
		},
	}

	record, ok := findExistingAddedDirectoryCalendar(records, folders, "user@example.com", "")
	if !ok {
		t.Fatal("expected registry email match without active folder")
	}
	if record.FolderID != "folder-sample-linked" {
		t.Fatalf("FolderID = %q, want folder-sample-linked", record.FolderID)
	}
}

func TestMergeCalendarListItemsIncludesRegistryOnly(t *testing.T) {
	folders := []owa.CalendarFolder{
		{DisplayName: "Calendar", FolderID: "folder-default"},
	}
	records := []addedDirectoryCalendarRecord{
		{
			Email:        "user@example.com",
			CalendarName: "Example User",
			FolderID:     "folder-sample-linked",
			CalendarID:   "folder-sample-linked",
		},
	}

	items := mergeCalendarListItems(folders, records)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[1].FolderID != "folder-sample-linked" {
		t.Fatalf("items[1].FolderID = %q, want folder-sample-linked", items[1].FolderID)
	}
	if items[1].Name != "Example User" {
		t.Fatalf("items[1].Name = %q, want Example User", items[1].Name)
	}
	if items[1].Email != "user@example.com" {
		t.Fatalf("items[1].Email = %q, want user@example.com", items[1].Email)
	}
}

func TestResolveCalendarListFolderFromSelectorByEmail(t *testing.T) {
	items := []calendarListItem{
		{Name: "Example User", Email: "user@example.com", FolderID: "folder-sample", CalendarID: "calendar-sample"},
	}
	got, err := resolveCalendarListFolderFromSelector(items, "user@example.com")
	if err != nil {
		t.Fatalf("resolveCalendarListFolderFromSelector() error: %v", err)
	}
	if got != "folder-sample" {
		t.Fatalf("folder = %q, want folder-sample", got)
	}
}

func TestResolveCalendarListFolderFromSelectorByCalendarID(t *testing.T) {
	items := []calendarListItem{
		{Name: "Example User", Email: "user@example.com", FolderID: "folder-sample", CalendarID: "opaque-calendar-id"},
	}
	got, err := resolveCalendarListFolderFromSelector(items, "AAExampleOpaqueCalendarID0123456789")
	if err != nil {
		t.Fatalf("resolveCalendarListFolderFromSelector() error: %v", err)
	}
	if got != "folder-sample" {
		t.Fatalf("folder = %q, want folder-sample", got)
	}
}

func TestResolveCalendarListFolderFromSelectorByName(t *testing.T) {
	items := []calendarListItem{
		{Name: "Example User", FolderID: "folder-sample"},
	}
	got, err := resolveCalendarListFolderFromSelector(items, "example user")
	if err != nil {
		t.Fatalf("resolveCalendarListFolderFromSelector() error: %v", err)
	}
	if got != "folder-sample" {
		t.Fatalf("folder = %q, want folder-sample", got)
	}
}

func TestResolveCalendarListFolderFromSelectorAmbiguousName(t *testing.T) {
	items := []calendarListItem{
		{Name: "Team Calendar", FolderID: "folder-1"},
		{Name: "Team Calendar", FolderID: "folder-2"},
	}
	_, err := resolveCalendarListFolderFromSelector(items, "Team Calendar")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
}

func TestResolveCalendarListFolderFromSelectorNoMatch(t *testing.T) {
	items := []calendarListItem{
		{Name: "Example User", FolderID: "folder-sample"},
	}
	_, err := resolveCalendarListFolderFromSelector(items, "alice@example.com")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no calendar matched") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLooksLikeCalendarOpaqueID(t *testing.T) {
	if !looksLikeCalendarOpaqueID("AAExampleOpaqueCalendarID0123456789") {
		t.Fatal("expected opaque id to match")
	}
	if looksLikeCalendarOpaqueID("user@example.com") {
		t.Fatal("email should not match opaque id")
	}
	if looksLikeCalendarOpaqueID("calendar") {
		t.Fatal("name should not match opaque id")
	}
}
