package owa

import (
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// NetlogFeatureSummary captures service actions and endpoints from network logs.
type NetlogFeatureSummary struct {
	ExtractedAt      time.Time          `json:"extracted_at"`
	ServiceActions   []NetlogActionStat `json:"service_actions,omitempty"`
	ServiceEndpoints []string           `json:"service_endpoints,omitempty"`
}

// NetlogActionStat summarizes a service.svc action.
type NetlogActionStat struct {
	Name       string         `json:"name"`
	Count      int            `json:"count"`
	Statuses   map[string]int `json:"statuses,omitempty"`
	SampleURLs []string       `json:"sample_urls,omitempty"`
}

// ExtractNetlogFeatures derives action names and endpoints from the network log.
func ExtractNetlogFeatures(netlog NetworkLog) *NetlogFeatureSummary {
	summary := &NetlogFeatureSummary{ExtractedAt: time.Now()}
	if len(netlog.Entries) == 0 {
		return summary
	}

	actions := make(map[string]*NetlogActionStat)
	endpoints := map[string]struct{}{}

	for _, entry := range netlog.Entries {
		action := actionFromEntry(entry)
		if action == "" {
			continue
		}
		stat := actions[action]
		if stat == nil {
			stat = &NetlogActionStat{Name: action, Statuses: map[string]int{}}
			actions[action] = stat
		}
		stat.Count++
		if entry.Status != 0 {
			stat.Statuses[itoa(entry.Status)]++
		}
		if len(stat.SampleURLs) < 3 && entry.URL != "" {
			stat.SampleURLs = append(stat.SampleURLs, entry.URL)
		}

		if endpoint := serviceEndpoint(entry.URL); endpoint != "" {
			endpoints[endpoint] = struct{}{}
		}
	}

	if len(actions) == 0 {
		return summary
	}

	names := make([]string, 0, len(actions))
	for name := range actions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		summary.ServiceActions = append(summary.ServiceActions, *actions[name])
	}

	if len(endpoints) > 0 {
		list := make([]string, 0, len(endpoints))
		for endpoint := range endpoints {
			list = append(list, endpoint)
		}
		sort.Strings(list)
		summary.ServiceEndpoints = list
	}

	return summary
}

func actionFromEntry(entry NetworkLogEntry) string {
	if entry.URL == "" {
		return actionFromHeaders(entry.RequestHeaders, entry.RequestBody)
	}
	if action := actionFromURL(entry.URL); action != "" {
		return action
	}
	if action := actionFromHeaders(entry.RequestHeaders, entry.RequestBody); action != "" {
		return action
	}
	return ""
}

func actionFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if !strings.Contains(strings.ToLower(parsed.Path), "/owa/service.svc") {
		return ""
	}
	query := parsed.Query()
	if action := query.Get("action"); action != "" {
		return action
	}
	for key, values := range query {
		if strings.EqualFold(key, "action") && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func actionFromHeaders(headers map[string]string, body string) string {
	if action := headerValue(headers, "action"); action != "" {
		return action
	}
	if action := actionFromBody(body); action != "" {
		return action
	}
	return ""
}

func actionFromBody(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" || trimmed == binaryPlaceholder {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return ""
	}
	if action, ok := payload["Action"].(string); ok && action != "" {
		return action
	}
	if action, ok := payload["action"].(string); ok && action != "" {
		return action
	}
	if typ, ok := payload["__type"].(string); ok && typ != "" {
		return normalizeTypeName(typ)
	}
	return ""
}

func normalizeTypeName(value string) string {
	trimmed := value
	if idx := strings.Index(trimmed, ":"); idx > 0 {
		trimmed = trimmed[:idx]
	}
	trimmed = strings.TrimSpace(trimmed)
	if strings.HasSuffix(trimmed, "Request") {
		trimmed = strings.TrimSuffix(trimmed, "Request")
	}
	return trimmed
}

func serviceEndpoint(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if !strings.Contains(strings.ToLower(parsed.Path), "/owa/service.svc") {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
