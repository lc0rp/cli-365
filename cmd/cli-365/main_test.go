package main

import "testing"

func TestDaemonFlagEnabled(t *testing.T) {
	tests := []struct {
		name          string
		flagSet       bool
		flagValue     bool
		configEnabled bool
		want          bool
	}{
		{
			name:          "config true when flag not set",
			flagSet:       false,
			flagValue:     false,
			configEnabled: true,
			want:          true,
		},
		{
			name:          "config false when flag not set",
			flagSet:       false,
			flagValue:     false,
			configEnabled: false,
			want:          false,
		},
		{
			name:          "explicit flag true overrides config false",
			flagSet:       true,
			flagValue:     true,
			configEnabled: false,
			want:          true,
		},
		{
			name:          "explicit flag false overrides config true",
			flagSet:       true,
			flagValue:     false,
			configEnabled: true,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := daemonFlagEnabled(tt.flagSet, tt.flagValue, tt.configEnabled)
			if got != tt.want {
				t.Fatalf("daemonFlagEnabled(%v, %v, %v) = %v, want %v", tt.flagSet, tt.flagValue, tt.configEnabled, got, tt.want)
			}
		})
	}
}

func TestBuildSearchQuery(t *testing.T) {
	got, err := buildSearchQuery(
		"alpha",
		"alice@example.com",
		"bob@example.com",
		"carol@example.com",
		"dave@example.com",
		"report",
		true,
		true,
		false,
		"2026-01-01",
		"2026-01-31",
	)
	if err != nil {
		t.Fatalf("buildSearchQuery error: %v", err)
	}
	want := "alpha from:\"alice@example.com\" to:\"bob@example.com\" cc:\"carol@example.com\" bcc:\"dave@example.com\" subject:\"report\" hasattachment:true isread:false received>=2026-01-01 received<=2026-01-31"
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestBuildSearchQueryReadFlags(t *testing.T) {
	_, err := buildSearchQuery("", "", "", "", "", "", false, true, true, "", "")
	if err == nil {
		t.Fatalf("expected error when unread and is-read are both set")
	}
}

func TestNormalizeDateTime(t *testing.T) {
	got, err := normalizeDateTime("2026-01-15T10:30:00Z")
	if err != nil {
		t.Fatalf("normalizeDateTime error: %v", err)
	}
	if got != "2026-01-15T10:30:00Z" {
		t.Fatalf("date = %q, want 2026-01-15T10:30:00Z", got)
	}
	got, err = normalizeDateTime("2026-01-15")
	if err != nil {
		t.Fatalf("normalizeDateTime error: %v", err)
	}
	if got != "2026-01-15" {
		t.Fatalf("date = %q, want 2026-01-15", got)
	}
}

func TestParseTrailingIntFlagShortCombined(t *testing.T) {
	value, ok := parseTrailingIntFlag([]string{"report", "-n5"}, []string{"--limit", "-n"})
	if !ok {
		t.Fatalf("expected match for -n5")
	}
	if value != 5 {
		t.Fatalf("value = %d, want 5", value)
	}
}

func TestParseTrailingIntFlagShortSeparate(t *testing.T) {
	value, ok := parseTrailingIntFlag([]string{"report", "-n", "7"}, []string{"--limit", "-n"})
	if !ok {
		t.Fatalf("expected match for -n 7")
	}
	if value != 7 {
		t.Fatalf("value = %d, want 7", value)
	}
}

func TestParseTrailingIntFlagIgnoresNonNumeric(t *testing.T) {
	_, ok := parseTrailingIntFlag([]string{"report", "-not"}, []string{"--limit", "-n"})
	if ok {
		t.Fatalf("expected -not to be ignored")
	}
}
