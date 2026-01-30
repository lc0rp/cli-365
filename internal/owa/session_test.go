package owa

import "testing"

func TestMergeSessionHeaders(t *testing.T) {
	base := SessionHeaders{SessionID: "a", AnchorMailbox: "user@example.com"}
	other := SessionHeaders{
		SessionID:     "b",
		TenantID:      "tenant",
		Prefer:        "IdType=\"ImmutableId\"",
		OwaAppID:      "app",
		ClientID:      "client",
		ClientFlights: "flights",
		RoutingKey:    "routing",
		MSAppName:     "appname",
		SearchGriffin: "GWSv2",
	}
	merged := MergeSessionHeaders(base, other)
	if merged.SessionID != "b" {
		t.Fatalf("expected SessionID b, got %q", merged.SessionID)
	}
	if merged.AnchorMailbox != "user@example.com" {
		t.Fatalf("expected AnchorMailbox preserved, got %q", merged.AnchorMailbox)
	}
	if merged.TenantID != "tenant" {
		t.Fatalf("expected TenantID set, got %q", merged.TenantID)
	}
	if merged.Prefer != "IdType=\"ImmutableId\"" {
		t.Fatalf("expected Prefer set, got %q", merged.Prefer)
	}
	if merged.OwaAppID != "app" {
		t.Fatalf("expected OwaAppID set, got %q", merged.OwaAppID)
	}
	if merged.ClientID != "client" {
		t.Fatalf("expected ClientID set, got %q", merged.ClientID)
	}
	if merged.ClientFlights != "flights" {
		t.Fatalf("expected ClientFlights set, got %q", merged.ClientFlights)
	}
	if merged.RoutingKey != "routing" {
		t.Fatalf("expected RoutingKey set, got %q", merged.RoutingKey)
	}
	if merged.MSAppName != "appname" {
		t.Fatalf("expected MSAppName set, got %q", merged.MSAppName)
	}
	if merged.SearchGriffin != "GWSv2" {
		t.Fatalf("expected SearchGriffin set, got %q", merged.SearchGriffin)
	}
}
