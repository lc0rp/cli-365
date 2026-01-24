package owa

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestNetworkLogEntry(t *testing.T) {
	entry := NetworkLogEntry{
		RequestID:    "123",
		URL:          "https://outlook.office.com/owa/0/service.svc?action=FindItem",
		Method:       "POST",
		ResourceType: "XHR",
		Status:       200,
		StatusText:   "OK",
		MimeType:     "application/json",
	}

	if entry.RequestID != "123" {
		t.Errorf("RequestID = %q, want 123", entry.RequestID)
	}
	if entry.URL == "" {
		t.Error("URL should not be empty")
	}
	if entry.Status != 200 {
		t.Errorf("Status = %d, want 200", entry.Status)
	}
}

func TestNetworkLogEntryFailed(t *testing.T) {
	entry := NetworkLogEntry{
		RequestID:    "456",
		URL:          "https://example.com/fail",
		Method:       "GET",
		ResourceType: "Fetch",
		Failed:       true,
		ErrorText:    "net::ERR_CONNECTION_REFUSED",
	}

	if !entry.Failed {
		t.Error("Failed should be true")
	}
	if entry.ErrorText == "" {
		t.Error("ErrorText should not be empty for failed requests")
	}
}

func TestNetworkLogSnapshot(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now().Add(-5 * time.Second),
		endedAt:   time.Now(),
		max:       100,
		dropped:   0,
		index:     make(map[proto.NetworkRequestID]int),
		entries: []NetworkLogEntry{
			{RequestID: "1", URL: "https://example.com/1", Status: 200},
			{RequestID: "2", URL: "https://example.com/2", Status: 404},
		},
	}

	snapshot := logger.Snapshot()

	if len(snapshot.Entries) != 2 {
		t.Errorf("Entries count = %d, want 2", len(snapshot.Entries))
	}
	if snapshot.Dropped != 0 {
		t.Errorf("Dropped = %d, want 0", snapshot.Dropped)
	}
	if snapshot.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
}

func TestNetworkLogSnapshotCopy(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       100,
		index:     make(map[proto.NetworkRequestID]int),
		entries: []NetworkLogEntry{
			{RequestID: "1", URL: "https://example.com"},
		},
	}

	snapshot1 := logger.Snapshot()

	// Modify original logger
	logger.entries = append(logger.entries, NetworkLogEntry{RequestID: "2"})

	snapshot2 := logger.Snapshot()

	// Snapshot1 should not be affected
	if len(snapshot1.Entries) != 1 {
		t.Errorf("snapshot1 entries = %d, want 1 (should be isolated copy)", len(snapshot1.Entries))
	}
	if len(snapshot2.Entries) != 2 {
		t.Errorf("snapshot2 entries = %d, want 2", len(snapshot2.Entries))
	}
}

func TestNetworkLoggerOnRequest(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       100,
		index:     make(map[proto.NetworkRequestID]int),
	}

	event := &proto.NetworkRequestWillBeSent{
		RequestID: "test-req-1",
		Request:   &proto.NetworkRequest{URL: "https://example.com/api", Method: "POST"},
		Type:      proto.NetworkResourceTypeXHR,
	}

	logger.onRequest(event)

	if len(logger.entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(logger.entries))
	}
	if logger.entries[0].RequestID != "test-req-1" {
		t.Errorf("RequestID = %q, want test-req-1", logger.entries[0].RequestID)
	}
	if logger.entries[0].Method != "POST" {
		t.Errorf("Method = %q, want POST", logger.entries[0].Method)
	}
}

func TestNetworkLoggerOnRequestNil(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       100,
		index:     make(map[proto.NetworkRequestID]int),
	}

	// Should not panic on nil event
	logger.onRequest(nil)

	if len(logger.entries) != 0 {
		t.Errorf("entries count = %d, want 0", len(logger.entries))
	}
}

func TestNetworkLoggerOnRequestOverflow(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       2,
		index:     make(map[proto.NetworkRequestID]int),
	}

	for i := 0; i < 5; i++ {
		event := &proto.NetworkRequestWillBeSent{
			RequestID: proto.NetworkRequestID(string(rune('a' + i))),
			Request:   &proto.NetworkRequest{URL: "https://example.com"},
		}
		logger.onRequest(event)
	}

	if len(logger.entries) != 2 {
		t.Errorf("entries count = %d, want 2 (max)", len(logger.entries))
	}
	if logger.dropped != 3 {
		t.Errorf("dropped = %d, want 3", logger.dropped)
	}
}

func TestNetworkLoggerOnResponse(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       100,
		index:     make(map[proto.NetworkRequestID]int),
	}

	// First add a request
	reqEvent := &proto.NetworkRequestWillBeSent{
		RequestID: "req-1",
		Request:   &proto.NetworkRequest{URL: "https://example.com", Method: "GET"},
	}
	logger.onRequest(reqEvent)

	// Then add response
	respEvent := &proto.NetworkResponseReceived{
		RequestID: "req-1",
		Response: &proto.NetworkResponse{
			URL:               "https://example.com",
			Status:            200,
			StatusText:        "OK",
			MIMEType:          "application/json",
			FromDiskCache:     false,
			FromServiceWorker: false,
		},
	}
	logger.onResponse(respEvent)

	if len(logger.entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(logger.entries))
	}
	if logger.entries[0].Status != 200 {
		t.Errorf("Status = %d, want 200", logger.entries[0].Status)
	}
	if logger.entries[0].MimeType != "application/json" {
		t.Errorf("MimeType = %q, want application/json", logger.entries[0].MimeType)
	}
}

func TestNetworkLoggerOnResponseWithoutRequest(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       100,
		index:     make(map[proto.NetworkRequestID]int),
	}

	// Add response without matching request
	respEvent := &proto.NetworkResponseReceived{
		RequestID: "orphan-req",
		Response: &proto.NetworkResponse{
			URL:        "https://example.com/orphan",
			Status:     200,
			StatusText: "OK",
		},
		Type: proto.NetworkResourceTypeFetch,
	}
	logger.onResponse(respEvent)

	if len(logger.entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(logger.entries))
	}
	if logger.entries[0].URL != "https://example.com/orphan" {
		t.Errorf("URL = %q, want https://example.com/orphan", logger.entries[0].URL)
	}
}

func TestNetworkLoggerOnResponseNil(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       100,
		index:     make(map[proto.NetworkRequestID]int),
	}

	// Should not panic
	logger.onResponse(nil)

	if len(logger.entries) != 0 {
		t.Errorf("entries count = %d, want 0", len(logger.entries))
	}
}

func TestNetworkLoggerOnFailed(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       100,
		index:     make(map[proto.NetworkRequestID]int),
	}

	// First add a request
	reqEvent := &proto.NetworkRequestWillBeSent{
		RequestID: "fail-req",
		Request:   &proto.NetworkRequest{URL: "https://example.com/fail", Method: "GET"},
	}
	logger.onRequest(reqEvent)

	// Then mark as failed
	failEvent := &proto.NetworkLoadingFailed{
		RequestID: "fail-req",
		ErrorText: "net::ERR_CONNECTION_TIMEOUT",
	}
	logger.onFailed(failEvent)

	if len(logger.entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(logger.entries))
	}
	if !logger.entries[0].Failed {
		t.Error("Failed should be true")
	}
	if logger.entries[0].ErrorText != "net::ERR_CONNECTION_TIMEOUT" {
		t.Errorf("ErrorText = %q, want net::ERR_CONNECTION_TIMEOUT", logger.entries[0].ErrorText)
	}
}

func TestNetworkLoggerOnFailedWithoutRequest(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       100,
		index:     make(map[proto.NetworkRequestID]int),
	}

	failEvent := &proto.NetworkLoadingFailed{
		RequestID: "orphan-fail",
		ErrorText: "net::ERR_NAME_NOT_RESOLVED",
		Type:      proto.NetworkResourceTypeFetch,
	}
	logger.onFailed(failEvent)

	if len(logger.entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(logger.entries))
	}
	if !logger.entries[0].Failed {
		t.Error("Failed should be true")
	}
}

func TestNetworkLoggerOnFailedNil(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       100,
		index:     make(map[proto.NetworkRequestID]int),
	}

	// Should not panic
	logger.onFailed(nil)

	if len(logger.entries) != 0 {
		t.Errorf("entries count = %d, want 0", len(logger.entries))
	}
}

func TestNetworkLoggerFinish(t *testing.T) {
	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       100,
		index:     make(map[proto.NetworkRequestID]int),
	}

	if !logger.endedAt.IsZero() {
		t.Error("endedAt should be zero before finish")
	}

	logger.finish()

	if logger.endedAt.IsZero() {
		t.Error("endedAt should be set after finish")
	}

	// Calling finish again should not change endedAt
	firstEnd := logger.endedAt
	time.Sleep(time.Millisecond)
	logger.finish()

	if logger.endedAt != firstEnd {
		t.Error("finish should not update endedAt if already set")
	}
}

func TestStartNetworkLoggerNilPage(t *testing.T) {
	_, _, err := StartNetworkLogger(nil, NetworkLogOptions{MaxEntries: 100})
	if err == nil {
		t.Error("StartNetworkLogger(nil) should return error")
	}
}

func TestStartNetworkLoggerDefaultMax(t *testing.T) {
	opts := normalizeNetworkLogOptions(NetworkLogOptions{})
	if opts.MaxEntries != 500 {
		t.Errorf("default max = %d, want 500", opts.MaxEntries)
	}
}

func TestNetworkLogStructure(t *testing.T) {
	log := NetworkLog{
		StartedAt: time.Now().Add(-10 * time.Second),
		EndedAt:   time.Now(),
		Dropped:   5,
		Entries: []NetworkLogEntry{
			{RequestID: "1", URL: "https://example.com", Status: 200},
		},
	}

	if log.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
	if log.EndedAt.IsZero() {
		t.Error("EndedAt should be set")
	}
	if log.Dropped != 5 {
		t.Errorf("Dropped = %d, want 5", log.Dropped)
	}
	if len(log.Entries) != 1 {
		t.Errorf("Entries count = %d, want 1", len(log.Entries))
	}
}
