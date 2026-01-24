package owa

import (
	"testing"
	"time"
)

func TestAnalyzeTemplates(t *testing.T) {
	discovery := &TemplateDiscovery{
		URL:         "https://outlook.office.com/mail/",
		ExtractedAt: time.Now(),
		WindowTemplates: map[string]string{
			"AlphaTemplate": `{"a":1,"b":2}`,
			"BetaTemplate":  `["x","y","z"]`,
			"BadTemplate":   `{"a":`,
		},
		StateTemplates: map[string]map[string]string{
			"__INITIAL_STATE__": {
				"InnerTemplate": `{"x":true}`,
			},
		},
	}

	summary := AnalyzeTemplates(discovery)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.URL != discovery.URL {
		t.Fatalf("summary URL = %q, want %q", summary.URL, discovery.URL)
	}
	if len(summary.Window) != 3 {
		t.Fatalf("summary.Window len = %d, want 3", len(summary.Window))
	}

	byKey := map[string]TemplateStat{}
	for _, stat := range summary.Window {
		byKey[stat.Key] = stat
	}

	if byKey["AlphaTemplate"].Kind != "object" {
		t.Fatalf("AlphaTemplate kind = %q, want object", byKey["AlphaTemplate"].Kind)
	}
	if byKey["BetaTemplate"].Kind != "array" {
		t.Fatalf("BetaTemplate kind = %q, want array", byKey["BetaTemplate"].Kind)
	}
	if byKey["BadTemplate"].Kind != "invalid-json" {
		t.Fatalf("BadTemplate kind = %q, want invalid-json", byKey["BadTemplate"].Kind)
	}

	if len(summary.State) != 1 {
		t.Fatalf("summary.State len = %d, want 1", len(summary.State))
	}
	if summary.State[0].StateKey != "__INITIAL_STATE__" {
		t.Fatalf("state key = %q, want __INITIAL_STATE__", summary.State[0].StateKey)
	}
	if len(summary.State[0].Templates) != 1 {
		t.Fatalf("state templates len = %d, want 1", len(summary.State[0].Templates))
	}
	if summary.State[0].Templates[0].Kind != "object" {
		t.Fatalf("state template kind = %q, want object", summary.State[0].Templates[0].Kind)
	}
}
