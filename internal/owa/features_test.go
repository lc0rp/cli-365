package owa

import "testing"

func TestExtractNetlogFeatures(t *testing.T) {
	netlog := NetworkLog{
		Entries: []NetworkLogEntry{
			{URL: "https://outlook.cloud.microsoft/owa/service.svc?action=GetTimeZone&app=Mail&n=1", Status: 200},
			{URL: "https://outlook.cloud.microsoft/owa/service.svc?app=Mail", RequestHeaders: map[string]string{"Action": "GetFolderChangeDigest"}, Status: 200},
			{URL: "https://outlook.cloud.microsoft/owa/service.svc?app=Mail", RequestBody: `{"Action":"GetOwaUserConfiguration"}`, Status: 401},
			{URL: "https://outlook.cloud.microsoft/owa/service.svc?app=Mail", RequestBody: `{"__type":"FindItemRequest:#Exchange"}`, Status: 500},
		},
	}

	summary := ExtractNetlogFeatures(netlog)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if len(summary.ServiceActions) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(summary.ServiceActions))
	}
	found := map[string]bool{}
	for _, action := range summary.ServiceActions {
		found[action.Name] = true
		if action.Count == 0 {
			t.Fatalf("action %q has zero count", action.Name)
		}
	}
	for _, name := range []string{"GetTimeZone", "GetFolderChangeDigest", "GetOwaUserConfiguration", "FindItem"} {
		if !found[name] {
			t.Fatalf("missing action %q", name)
		}
	}
}
