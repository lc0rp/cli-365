package daemon

import (
	"reflect"
	"testing"
)

func TestShouldMaintainPrimaryTab(t *testing.T) {
	tests := []struct {
		name        string
		commandPath string
		argv        []string
		want        bool
	}{
		{name: "mail search", commandPath: "mail search", want: true},
		{name: "calendar list", commandPath: "calendar list", want: true},
		{name: "auth login", commandPath: "auth login", want: true},
		{name: "debug discover", commandPath: "debug discover", want: true},
		{name: "auth status", commandPath: "auth status", want: false},
		{name: "browser status", commandPath: "browser status", want: false},
		{name: "help", commandPath: "help", want: false},
		{name: "infer from argv", commandPath: "", argv: []string{"mail", "search", "invoice"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldMaintainPrimaryTab(tt.commandPath, tt.argv)
			if got != tt.want {
				t.Fatalf("shouldMaintainPrimaryTab(%q, %v) = %v, want %v", tt.commandPath, tt.argv, got, tt.want)
			}
		})
	}
}

func TestPlanPrimaryTab_KeepExistingAndCloseExtras(t *testing.T) {
	pages := []tabSnapshot{
		{ID: "owa-a", URL: "https://outlook.office.com/mail/"},
		{ID: "blank-1", URL: "about:blank"},
		{ID: "owa-b", URL: "https://outlook.office.com/mail/"},
		{ID: "other", URL: "https://example.com"},
	}

	got := planPrimaryTab("owa-b", pages)
	if got.PrimaryID != "owa-b" {
		t.Fatalf("PrimaryID = %q, want owa-b", got.PrimaryID)
	}
	wantClose := []string{"owa-a", "blank-1"}
	if !reflect.DeepEqual(got.CloseIDs, wantClose) {
		t.Fatalf("CloseIDs = %v, want %v", got.CloseIDs, wantClose)
	}
}

func TestPlanPrimaryTab_SelectFirstOWAWhenExistingMissing(t *testing.T) {
	pages := []tabSnapshot{
		{ID: "blank-1", URL: "about:blank"},
		{ID: "owa-a", URL: "https://outlook.office.com/mail/"},
		{ID: "owa-b", URL: "https://outlook.office.com/calendar/"},
	}

	got := planPrimaryTab("missing", pages)
	if got.PrimaryID != "owa-a" {
		t.Fatalf("PrimaryID = %q, want owa-a", got.PrimaryID)
	}
	wantClose := []string{"blank-1", "owa-b"}
	if !reflect.DeepEqual(got.CloseIDs, wantClose) {
		t.Fatalf("CloseIDs = %v, want %v", got.CloseIDs, wantClose)
	}
}

func TestPlanPrimaryTab_NoOWAPagesNoPrimary(t *testing.T) {
	pages := []tabSnapshot{
		{ID: "blank-1", URL: "about:blank"},
		{ID: "other", URL: "https://example.com"},
	}

	got := planPrimaryTab("none", pages)
	if got.PrimaryID != "" {
		t.Fatalf("PrimaryID = %q, want empty", got.PrimaryID)
	}
	if len(got.CloseIDs) != 0 {
		t.Fatalf("CloseIDs = %v, want empty", got.CloseIDs)
	}
}
