package owa

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTemplateDiscoveryStructure(t *testing.T) {
	discovery := &TemplateDiscovery{
		URL:         "https://outlook.office.com/mail/",
		ExtractedAt: time.Now(),
		WindowTemplates: map[string]string{
			"TestTemplate": `{"key": "value"}`,
		},
		StateTemplates: map[string]map[string]string{
			"__INITIAL_STATE__": {
				"innerTemplate": `{"inner": true}`,
			},
		},
	}

	if discovery.URL == "" {
		t.Error("URL should not be empty")
	}
	if discovery.ExtractedAt.IsZero() {
		t.Error("ExtractedAt should be set")
	}
	if len(discovery.WindowTemplates) != 1 {
		t.Errorf("WindowTemplates len = %d, want 1", len(discovery.WindowTemplates))
	}
	if len(discovery.StateTemplates) != 1 {
		t.Errorf("StateTemplates len = %d, want 1", len(discovery.StateTemplates))
	}
}

func TestTemplateStatStructure(t *testing.T) {
	stat := TemplateStat{
		Key:        "TestTemplate",
		Kind:       "object",
		SizeBytes:  1024,
		KeyCount:   5,
		SampleKeys: []string{"a", "b", "c"},
	}

	if stat.Key == "" {
		t.Error("Key should not be empty")
	}
	if stat.Kind != "object" {
		t.Errorf("Kind = %q, want object", stat.Kind)
	}
	if stat.SizeBytes != 1024 {
		t.Errorf("SizeBytes = %d, want 1024", stat.SizeBytes)
	}
}

func TestSummarizeTemplateTypes(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantKind string
	}{
		{"object", `{"a": 1}`, "object"},
		{"array", `[1, 2, 3]`, "array"},
		{"string", `"hello"`, "string"},
		{"number", `42`, "number"},
		{"bool", `true`, "bool"},
		{"null", `null`, "null"},
		{"invalid", `{broken`, "invalid-json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stat := summarizeTemplate("test", tt.raw)
			if stat.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", stat.Kind, tt.wantKind)
			}
		})
	}
}

func TestSummarizeTemplateObjectSampleKeys(t *testing.T) {
	raw := `{"alpha": 1, "beta": 2, "gamma": 3, "delta": 4}`
	stat := summarizeTemplate("test", raw)

	if stat.Kind != "object" {
		t.Fatalf("Kind = %q, want object", stat.Kind)
	}
	if stat.KeyCount != 4 {
		t.Errorf("KeyCount = %d, want 4", stat.KeyCount)
	}
	if len(stat.SampleKeys) == 0 {
		t.Error("SampleKeys should not be empty")
	}
}

func TestSummarizeTemplateArrayItemCount(t *testing.T) {
	raw := `[1, 2, 3, 4, 5]`
	stat := summarizeTemplate("test", raw)

	if stat.Kind != "array" {
		t.Fatalf("Kind = %q, want array", stat.Kind)
	}
	if stat.ItemCount != 5 {
		t.Errorf("ItemCount = %d, want 5", stat.ItemCount)
	}
}

func TestSummarizeTemplateParseError(t *testing.T) {
	stat := summarizeTemplate("test", `{not valid json`)

	if stat.Kind != "invalid-json" {
		t.Errorf("Kind = %q, want invalid-json", stat.Kind)
	}
	if stat.ParseError == "" {
		t.Error("ParseError should be set for invalid JSON")
	}
}

func TestSampleKeysLimit(t *testing.T) {
	// Create map with more than 8 keys
	m := make(map[string]interface{})
	for i := 0; i < 20; i++ {
		m[string(rune('a'+i))] = i
	}

	keys := sampleKeys(m, 8)
	if len(keys) != 8 {
		t.Errorf("sampleKeys returned %d keys, want 8", len(keys))
	}
}

func TestSampleKeysEmpty(t *testing.T) {
	keys := sampleKeys(nil, 8)
	if keys != nil {
		t.Errorf("sampleKeys(nil) = %v, want nil", keys)
	}

	keys = sampleKeys(map[string]interface{}{}, 8)
	if keys != nil {
		t.Errorf("sampleKeys({}) = %v, want nil", keys)
	}
}

func TestSampleKeysZeroLimit(t *testing.T) {
	m := map[string]interface{}{"a": 1}
	keys := sampleKeys(m, 0)
	if keys != nil {
		t.Errorf("sampleKeys with limit 0 = %v, want nil", keys)
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]string{
		"zebra":  "z",
		"alpha":  "a",
		"middle": "m",
	}

	keys := sortedKeys(m)
	if len(keys) != 3 {
		t.Fatalf("sortedKeys len = %d, want 3", len(keys))
	}
	if keys[0] != "alpha" {
		t.Errorf("first key = %q, want alpha", keys[0])
	}
	if keys[2] != "zebra" {
		t.Errorf("last key = %q, want zebra", keys[2])
	}
}

func TestSortedKeysNil(t *testing.T) {
	keys := sortedKeys(nil)
	if keys != nil {
		t.Errorf("sortedKeys(nil) = %v, want nil", keys)
	}
}

func TestSortedKeysMap(t *testing.T) {
	m := map[string]map[string]string{
		"STATE_B": {"t1": "v1"},
		"STATE_A": {"t2": "v2"},
	}

	keys := sortedKeysMap(m)
	if len(keys) != 2 {
		t.Fatalf("sortedKeysMap len = %d, want 2", len(keys))
	}
	if keys[0] != "STATE_A" {
		t.Errorf("first key = %q, want STATE_A", keys[0])
	}
}

func TestSortedKeysMapNil(t *testing.T) {
	keys := sortedKeysMap(nil)
	if keys != nil {
		t.Errorf("sortedKeysMap(nil) = %v, want nil", keys)
	}
}

func TestDiscoverTemplatesNilPage(t *testing.T) {
	_, err := DiscoverTemplates(nil)
	if err == nil {
		t.Error("DiscoverTemplates(nil) should return error")
	}
}

func TestAnalyzeTemplatesNil(t *testing.T) {
	summary := AnalyzeTemplates(nil)
	if summary != nil {
		t.Errorf("AnalyzeTemplates(nil) = %v, want nil", summary)
	}
}

func TestTemplateSummarySerialization(t *testing.T) {
	summary := &TemplateSummary{
		URL:         "https://example.com",
		ExtractedAt: time.Now(),
		Window: []TemplateStat{
			{Key: "test", Kind: "object", SizeBytes: 100},
		},
		State: []StateTemplateSummary{
			{
				StateKey: "__STATE__",
				Templates: []TemplateStat{
					{Key: "inner", Kind: "string", SizeBytes: 50},
				},
			},
		},
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed TemplateSummary
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.URL != summary.URL {
		t.Errorf("URL mismatch: got %q, want %q", parsed.URL, summary.URL)
	}
	if len(parsed.Window) != 1 {
		t.Errorf("Window len = %d, want 1", len(parsed.Window))
	}
	if len(parsed.State) != 1 {
		t.Errorf("State len = %d, want 1", len(parsed.State))
	}
}

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
