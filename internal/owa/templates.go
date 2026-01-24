package owa

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/go-rod/rod"
)

// TemplateDiscovery captures template-like values discovered in the OWA page.
type TemplateDiscovery struct {
	URL             string                       `json:"url"`
	ExtractedAt     time.Time                    `json:"extracted_at"`
	WindowTemplates map[string]string            `json:"window_templates,omitempty"`
	StateTemplates  map[string]map[string]string `json:"state_templates,omitempty"`
}

// TemplateStat summarizes a single template value.
type TemplateStat struct {
	Key        string   `json:"key"`
	Kind       string   `json:"kind"`
	SizeBytes  int      `json:"size_bytes"`
	KeyCount   int      `json:"key_count,omitempty"`
	ItemCount  int      `json:"item_count,omitempty"`
	SampleKeys []string `json:"sample_keys,omitempty"`
	ParseError string   `json:"parse_error,omitempty"`
}

// StateTemplateSummary groups template stats by state/config bucket.
type StateTemplateSummary struct {
	StateKey  string         `json:"state_key"`
	Templates []TemplateStat `json:"templates"`
}

// TemplateSummary provides a lightweight analysis of discovered templates.
type TemplateSummary struct {
	URL         string                 `json:"url"`
	ExtractedAt time.Time              `json:"extracted_at"`
	Window      []TemplateStat         `json:"window,omitempty"`
	State       []StateTemplateSummary `json:"state,omitempty"`
}

type templateDiscoveryJS struct {
	URL             string                       `json:"url"`
	WindowTemplates map[string]string            `json:"windowTemplates"`
	StateTemplates  map[string]map[string]string `json:"stateTemplates"`
}

// DiscoverTemplates scans the OWA page for template-like values.
func DiscoverTemplates(page *rod.Page) (*TemplateDiscovery, error) {
	if page == nil {
		return nil, errors.New("page is nil")
	}

	result, err := page.Eval(`() => {
		const isTemplateKey = (key) => /template/i.test(key);
		const isStateKey = (key) => /INITIAL_STATE|STATE|CONFIG/i.test(key);
		const seen = new WeakSet();
		const safeStringify = (value) => {
			try {
				return JSON.stringify(value, (k, v) => {
					if (typeof v === "function") return "[Function " + (v.name || "anonymous") + "]";
					if (typeof v === "bigint") return v.toString();
					if (v && typeof v === "object") {
						if (seen.has(v)) return "[Circular]";
						seen.add(v);
					}
					return v;
				});
			} catch {
				return null;
			}
		};

		const windowTemplates = {};
		const stateTemplates = {};
		const keys = Object.keys(window);

		for (const key of keys) {
			if (!isTemplateKey(key)) continue;
			const json = safeStringify(window[key]);
			if (json !== null) windowTemplates[key] = json;
		}

		for (const key of keys) {
			if (!isStateKey(key)) continue;
			const value = window[key];
			if (!value || typeof value !== "object") continue;
			const inner = {};
			for (const innerKey of Object.keys(value)) {
				if (!isTemplateKey(innerKey)) continue;
				const json = safeStringify(value[innerKey]);
				if (json !== null) inner[innerKey] = json;
			}
			if (Object.keys(inner).length > 0) stateTemplates[key] = inner;
		}

		return {
			url: location.href,
			windowTemplates,
			stateTemplates,
		};
	}`)
	if err != nil {
		return nil, err
	}

	var parsed templateDiscoveryJS
	if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &parsed); err != nil {
		return nil, err
	}

	return &TemplateDiscovery{
		URL:             parsed.URL,
		ExtractedAt:     time.Now(),
		WindowTemplates: parsed.WindowTemplates,
		StateTemplates:  parsed.StateTemplates,
	}, nil
}

// AnalyzeTemplates creates a summary for manual inspection and logging.
func AnalyzeTemplates(discovery *TemplateDiscovery) *TemplateSummary {
	if discovery == nil {
		return nil
	}

	summary := &TemplateSummary{
		URL:         discovery.URL,
		ExtractedAt: discovery.ExtractedAt,
	}

	windowKeys := sortedKeys(discovery.WindowTemplates)
	for _, key := range windowKeys {
		stat := summarizeTemplate(key, discovery.WindowTemplates[key])
		summary.Window = append(summary.Window, stat)
	}

	stateKeys := sortedKeysMap(discovery.StateTemplates)
	for _, stateKey := range stateKeys {
		inner := discovery.StateTemplates[stateKey]
		innerKeys := sortedKeys(inner)
		stats := make([]TemplateStat, 0, len(innerKeys))
		for _, key := range innerKeys {
			stats = append(stats, summarizeTemplate(key, inner[key]))
		}
		summary.State = append(summary.State, StateTemplateSummary{
			StateKey:  stateKey,
			Templates: stats,
		})
	}

	return summary
}

func summarizeTemplate(key string, raw string) TemplateStat {
	stat := TemplateStat{
		Key:       key,
		SizeBytes: len(raw),
	}

	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		stat.Kind = "invalid-json"
		stat.ParseError = err.Error()
		return stat
	}

	switch v := value.(type) {
	case map[string]interface{}:
		stat.Kind = "object"
		stat.KeyCount = len(v)
		stat.SampleKeys = sampleKeys(v, 8)
	case []interface{}:
		stat.Kind = "array"
		stat.ItemCount = len(v)
	case string:
		stat.Kind = "string"
	case bool:
		stat.Kind = "bool"
	case float64:
		stat.Kind = "number"
	case nil:
		stat.Kind = "null"
	default:
		stat.Kind = "unknown"
	}

	return stat
}

func sampleKeys(m map[string]interface{}, limit int) []string {
	if len(m) == 0 || limit <= 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return keys
}

func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysMap(m map[string]map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
