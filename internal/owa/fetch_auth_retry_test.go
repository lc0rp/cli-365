package owa

import (
	"encoding/json"
	"testing"

	"github.com/go-rod/rod"
)

func TestCallOWAAction_Bearer401_FallsBackToCanary(t *testing.T) {
	oldFetch := fetchFn
	oldFeatures := sessionFeatures
	t.Cleanup(func() {
		fetchFn = oldFetch
		sessionFeatures = oldFeatures
	})
	sessionFeatures = NewFeatureState(DefaultFeatureCatalog())

	var calls int
	fetchFn = func(_ *rod.Page, req FetchRequest) (*FetchResponse, error) {
		calls++
		if req.Headers == nil {
			return &FetchResponse{Status: 500, Body: json.RawMessage(`{}`)}, nil
		}
		if req.Headers["Authorization"] != "" {
			return &FetchResponse{Status: 401, StatusText: "Unauthorized", Body: json.RawMessage(`{}`)}, nil
		}
		if req.Headers["X-OWA-CANARY"] != "" {
			return &FetchResponse{Status: 200, StatusText: "OK", Body: json.RawMessage(`{}`)}, nil
		}
		return &FetchResponse{Status: 401, StatusText: "Unauthorized", Body: json.RawMessage(`{}`)}, nil
	}

	tokens := &Tokens{
		Canary: "canary-ok",
		Bearer: "Bearer stale",
	}
	resp, err := callOWAAction(nil, tokens, "FindItem", map[string]interface{}{"k": "v"}, "", "Mail")
	if err != nil {
		t.Fatalf("callOWAAction error: %v", err)
	}
	if resp == nil {
		t.Fatalf("callOWAAction resp is nil")
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200", resp.Status)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}
