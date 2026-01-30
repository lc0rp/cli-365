package owa

import "testing"

func TestFeatureStateDisable(t *testing.T) {
	state := NewFeatureState(DefaultFeatureCatalog())
	action := "FindItem"
	if err := state.Check(action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state.Disable(action, "not supported")
	if err := state.Check(action); err == nil {
		t.Fatalf("expected disabled error")
	}
}

func TestParseOWAError(t *testing.T) {
	payload := []byte(`{"Body":{"ResponseCode":"ErrorNotSupported","MessageText":"Feature not supported","ExceptionName":"NotSupportedException"}}`)
	info := parseOWAError(payload)
	if info.Code != "ErrorNotSupported" {
		t.Fatalf("expected code, got %q", info.Code)
	}
	if info.Exception != "NotSupportedException" {
		t.Fatalf("expected exception, got %q", info.Exception)
	}
	if info.Message == "" {
		t.Fatalf("expected message")
	}
}
