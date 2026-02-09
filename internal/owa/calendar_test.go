package owa

import (
	"encoding/json"
	"testing"
)

func TestCalendarLocationUnmarshal(t *testing.T) {
	var loc CalendarLocation
	if err := json.Unmarshal([]byte(`"Board Room"`), &loc); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if loc.DisplayName != "Board Room" {
		t.Fatalf("DisplayName = %q", loc.DisplayName)
	}

	loc = CalendarLocation{}
	if err := json.Unmarshal([]byte(`{"DisplayName":"Suite 5"}`), &loc); err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}
	if loc.DisplayName != "Suite 5" {
		t.Fatalf("DisplayName = %q", loc.DisplayName)
	}
}

func TestCalendarEventUnmarshalItemID(t *testing.T) {
	payload := `{"ItemId":{"Id":"event-1","ChangeKey":"ck-1"},"Subject":"Standup","Location":{"DisplayName":"Zoom"}}`
	var ev CalendarEvent
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if ev.ID != "event-1" {
		t.Fatalf("ID = %q", ev.ID)
	}
	if ev.ChangeKey != "ck-1" {
		t.Fatalf("ChangeKey = %q", ev.ChangeKey)
	}
	if ev.Location == nil || ev.Location.DisplayName != "Zoom" {
		t.Fatalf("Location = %+v", ev.Location)
	}
}

func TestBuildCalendarViewRequest(t *testing.T) {
	if _, err := buildCalendarViewRequest("", "", 10, ""); err == nil {
		t.Fatalf("expected error for missing dates")
	}
	body, err := buildCalendarViewRequest("2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z", 10, "")
	if err != nil {
		t.Fatalf("buildCalendarViewRequest: %v", err)
	}
	if body["CalendarView"] == nil {
		t.Fatalf("CalendarView missing")
	}
	folders, ok := body["ParentFolderIds"].([]map[string]interface{})
	if !ok || len(folders) == 0 {
		t.Fatalf("ParentFolderIds missing")
	}
}

func TestBuildCalendarViewJsonRequest(t *testing.T) {
	req, err := buildCalendarViewJsonRequest("2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z", 10, "")
	if err != nil {
		t.Fatalf("buildCalendarViewJsonRequest: %v", err)
	}
	if req["__type"] != "FindItemJsonRequest:#Exchange" {
		t.Fatalf("unexpected type: %v", req["__type"])
	}
	if req["Header"] == nil {
		t.Fatalf("Header missing")
	}
	if req["Body"] == nil {
		t.Fatalf("Body missing")
	}
}

func TestShouldRetryCalendarView(t *testing.T) {
	resp := &FetchResponse{Status: 500, Body: json.RawMessage(`{"Body":{"ExceptionName":"OwaSerializationException","MessageText":"oops"}}`)}
	if !shouldRetryCalendarView(resp) {
		t.Fatalf("expected retry for serialization exception")
	}
	resp = &FetchResponse{Status: 500, Body: json.RawMessage(`{"Body":{"ExceptionName":"MemberAccessException","ResponseCode":"ErrorInternalServerError"}}`)}
	if !shouldRetryCalendarView(resp) {
		t.Fatalf("expected retry for member access exception")
	}
	resp = &FetchResponse{Status: 400, Body: json.RawMessage(`{"Body":{"ExceptionName":"OtherException"}}`)}
	if shouldRetryCalendarView(resp) {
		t.Fatalf("unexpected retry for non-500 response")
	}
}

func TestBuildCreateCalendarEventRequest(t *testing.T) {
	if _, err := buildCreateCalendarEventRequest(nil); err == nil {
		t.Fatalf("expected error for nil draft")
	}
	if _, err := buildCreateCalendarEventRequest(&CalendarEventDraft{}); err == nil {
		t.Fatalf("expected error for missing fields")
	}
	req, err := buildCreateCalendarEventRequest(&CalendarEventDraft{
		Subject: "Planning",
		Start:   "2026-01-01T09:00:00Z",
		End:     "2026-01-01T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req["__type"] != "CreateItemJsonRequest:#Exchange" {
		t.Fatalf("unexpected type: %v", req["__type"])
	}
}

func TestBuildUpdateCalendarEventRequest(t *testing.T) {
	if _, err := buildUpdateCalendarEventRequest("", &CalendarEventUpdate{}); err == nil {
		t.Fatalf("expected error for missing ID")
	}
	if _, err := buildUpdateCalendarEventRequest("event-1", &CalendarEventUpdate{}); err == nil {
		t.Fatalf("expected error for empty update")
	}
	subject := "Updated"
	req, err := buildUpdateCalendarEventRequest("event-1", &CalendarEventUpdate{Subject: &subject})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req["__type"] != "UpdateItemJsonRequest:#Exchange" {
		t.Fatalf("unexpected type: %v", req["__type"])
	}
}
