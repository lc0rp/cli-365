package owa

import (
	"encoding/json"
	"testing"
)

func TestSearchServiceToResultMapsMessagesAndConversations(t *testing.T) {
	raw := []byte(`{
		"EntitySets":[
			{"ResultSets":[
				{"ResultCount":1,"Total":1,"Results":[
					{"Type":"Conversation","Source":{
						"ItemId":{"Id":"msg-1"},
						"ConversationId":{"Id":"conv-1"},
						"ConversationTopic":"Hello",
						"LastDeliveryTime":"2026-01-01T00:00:00Z",
						"HasAttachments":true,
						"Importance":"High",
						"Preview":"Preview text",
						"ParentFolderId":{"Id":"folder-1"},
						"From":{"EmailAddress":{"Name":"Alice","Address":"a@example.com"}}
					}}
				]}
			]}
		]
	}`)

	var payload searchServiceResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	result := searchServiceToResult(payload)
	if result.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, want 1", result.TotalCount)
	}
	if len(result.Conversations) != 1 {
		t.Fatalf("Conversations length = %d, want 1", len(result.Conversations))
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(result.Messages))
	}
	msg := result.Messages[0]
	if msg.ID != "msg-1" {
		t.Fatalf("Message ID = %q, want msg-1", msg.ID)
	}
	if msg.ConversationID != "conv-1" {
		t.Fatalf("ConversationID = %q, want conv-1", msg.ConversationID)
	}
	if msg.Subject != "Hello" {
		t.Fatalf("Subject = %q, want Hello", msg.Subject)
	}
	if msg.ParentFolderId != "folder-1" {
		t.Fatalf("ParentFolderId = %q, want folder-1", msg.ParentFolderId)
	}
	if msg.From == nil || msg.From.Address != "a@example.com" {
		t.Fatalf("From address = %v, want a@example.com", msg.From)
	}
}

func TestBuildSearchServiceBodyDefaultFilter(t *testing.T) {
	body := buildSearchServiceBody("hello", 10, nil)
	reqs, ok := body["EntityRequests"].([]map[string]interface{})
	if !ok || len(reqs) != 1 {
		t.Fatalf("EntityRequests missing or invalid")
	}
	filter, ok := reqs[0]["Filter"].(map[string]interface{})
	if !ok {
		t.Fatalf("Filter missing")
	}
	orList, ok := filter["Or"].([]map[string]interface{})
	if !ok || len(orList) < 2 {
		t.Fatalf("default filter should include msgfolderroot and DeletedItems")
	}
}

func TestBuildSearchServiceBodyFolderFilter(t *testing.T) {
	filter := buildSearchServiceFilter("inbox")
	body := buildSearchServiceBody("hello", 10, filter)
	reqs := body["EntityRequests"].([]map[string]interface{})
	f := reqs[0]["Filter"].(map[string]interface{})
	orList := f["Or"].([]map[string]interface{})
	term := orList[0]["Term"].(map[string]interface{})
	if term["DistinguishedFolderName"] != "Inbox" {
		t.Fatalf("DistinguishedFolderName = %v, want Inbox", term["DistinguishedFolderName"])
	}
	filter = buildSearchServiceFilter("folder-123")
	body = buildSearchServiceBody("hello", 10, filter)
	reqs = body["EntityRequests"].([]map[string]interface{})
	f = reqs[0]["Filter"].(map[string]interface{})
	orList = f["Or"].([]map[string]interface{})
	term = orList[0]["Term"].(map[string]interface{})
	if term["ParentFolderId"] != "folder-123" {
		t.Fatalf("ParentFolderId = %v, want folder-123", term["ParentFolderId"])
	}
}
