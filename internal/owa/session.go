package owa

import "sync"

// SessionHeaders captures session-related headers used by OWA.
type SessionHeaders struct {
	SessionID     string `json:"session_id,omitempty"`
	AnchorMailbox string `json:"anchor_mailbox,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`
	Prefer        string `json:"prefer,omitempty"`
	OwaAppID      string `json:"owa_app_id,omitempty"`
	ClientID      string `json:"client_id,omitempty"`
	ClientFlights string `json:"client_flights,omitempty"`
	RoutingKey    string `json:"routing_key,omitempty"`
	MSAppName     string `json:"ms_app_name,omitempty"`
	SearchGriffin string `json:"search_griffin,omitempty"`
}

// IsZero reports whether the session headers are empty.
func (h SessionHeaders) IsZero() bool {
	return h.SessionID == "" && h.AnchorMailbox == "" && h.TenantID == "" && h.Prefer == "" &&
		h.OwaAppID == "" && h.ClientID == "" && h.ClientFlights == "" && h.RoutingKey == "" &&
		h.MSAppName == "" && h.SearchGriffin == ""
}

// MergeSessionHeaders merges non-empty fields from src into dst.
func MergeSessionHeaders(dst, src SessionHeaders) SessionHeaders {
	if src.SessionID != "" {
		dst.SessionID = src.SessionID
	}
	if src.AnchorMailbox != "" {
		dst.AnchorMailbox = src.AnchorMailbox
	}
	if src.TenantID != "" {
		dst.TenantID = src.TenantID
	}
	if src.Prefer != "" {
		dst.Prefer = src.Prefer
	}
	if src.OwaAppID != "" {
		dst.OwaAppID = src.OwaAppID
	}
	if src.ClientID != "" {
		dst.ClientID = src.ClientID
	}
	if src.ClientFlights != "" {
		dst.ClientFlights = src.ClientFlights
	}
	if src.RoutingKey != "" {
		dst.RoutingKey = src.RoutingKey
	}
	if src.MSAppName != "" {
		dst.MSAppName = src.MSAppName
	}
	if src.SearchGriffin != "" {
		dst.SearchGriffin = src.SearchGriffin
	}
	return dst
}

type sessionState struct {
	mu      sync.RWMutex
	headers SessionHeaders
}

var globalSession = &sessionState{}

// SetSessionHeaders updates the process-local session headers.
func SetSessionHeaders(headers SessionHeaders) {
	if headers.IsZero() {
		return
	}
	globalSession.mu.Lock()
	globalSession.headers = MergeSessionHeaders(globalSession.headers, headers)
	globalSession.mu.Unlock()
}

// CurrentSessionHeaders returns the current process-local session headers.
func CurrentSessionHeaders() SessionHeaders {
	globalSession.mu.RLock()
	defer globalSession.mu.RUnlock()
	return globalSession.headers
}
