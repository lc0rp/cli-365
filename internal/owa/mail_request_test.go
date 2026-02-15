package owa

import "testing"

func TestBuildGetItemRequest(t *testing.T) {
	req, err := buildGetItemRequest("msg-1")
	if err != nil {
		t.Fatalf("buildGetItemRequest error: %v", err)
	}
	if req["__type"] != "GetItemJsonRequest:#Exchange" {
		t.Fatalf("__type = %v", req["__type"])
	}
	body := req["Body"].(map[string]interface{})
	if body["__type"] != "GetItemRequest:#Exchange" {
		t.Fatalf("body __type = %v", body["__type"])
	}
	ids := body["ItemIds"].([]map[string]interface{})
	if ids[0]["Id"] != "msg-1" {
		t.Fatalf("ItemId = %v", ids[0]["Id"])
	}
	shape := body["ItemShape"].(map[string]interface{})
	if shape["BaseShape"] != "Default" {
		t.Fatalf("BaseShape = %v", shape["BaseShape"])
	}
	props := shape["AdditionalProperties"].([]map[string]interface{})
	found := map[string]bool{}
	for _, prop := range props {
		if field, ok := prop["FieldURI"].(string); ok {
			found[field] = true
		}
	}
	for _, field := range []string{"item:ConversationId", "item:ParentFolderId"} {
		if !found[field] {
			t.Fatalf("missing AdditionalProperties %s", field)
		}
	}
}

func TestBuildGetConversationItemsRequest(t *testing.T) {
	req, err := buildGetConversationItemsRequest("conv-1", "", nil)
	if err != nil {
		t.Fatalf("buildGetConversationItemsRequest error: %v", err)
	}
	if req["__type"] != "GetConversationItemsJsonRequest:#Exchange" {
		t.Fatalf("__type = %v", req["__type"])
	}
	body := req["Body"].(map[string]interface{})
	if body["__type"] != "GetConversationItemsRequest:#Exchange" {
		t.Fatalf("body __type = %v", body["__type"])
	}
	convs := body["Conversations"].([]map[string]interface{})
	cid := convs[0]["ConversationId"].(map[string]interface{})
	if cid["__type"] != "ConversationId:#Exchange" {
		t.Fatalf("__type = %v", cid["__type"])
	}
	if cid["Id"] != "conv-1" {
		t.Fatalf("ConversationId = %v", cid["Id"])
	}
}

func TestBuildGetConversationItemsRequestWithFolder(t *testing.T) {
	req, err := buildGetConversationItemsRequest("conv-1", "folder-1", nil)
	if err != nil {
		t.Fatalf("buildGetConversationItemsRequest error: %v", err)
	}
	body := req["Body"].(map[string]interface{})
	convs := body["Conversations"].([]map[string]interface{})
	parent := convs[0]["ParentFolderId"].(map[string]interface{})
	if parent["Id"] != "folder-1" {
		t.Fatalf("ParentFolderId = %v", parent["Id"])
	}
}

func TestBuildGetConversationItemsRequestMailboxInfo(t *testing.T) {
	tokens := &Tokens{UserEmail: "user@example.com"}
	req, err := buildGetConversationItemsRequest("conv-1", "", tokens)
	if err != nil {
		t.Fatalf("buildGetConversationItemsRequest error: %v", err)
	}
	body := req["Body"].(map[string]interface{})
	mailbox := body["MailboxInfo"].(map[string]interface{})
	if mailbox["mailboxSmtpAddress"] != "user@example.com" {
		t.Fatalf("mailboxSmtpAddress = %v", mailbox["mailboxSmtpAddress"])
	}
}

func TestBuildCreateDraftRequest(t *testing.T) {
	draft := &Draft{
		Subject: "Hello",
		Body:    &MessageBody{BodyType: "Text", Value: "Body"},
		ToRecipients: []EmailAddress{
			{Name: "Alice", Address: "alice@example.com"},
		},
	}
	tokens := &Tokens{UserEmail: "me@example.com"}
	req, err := buildCreateDraftRequest(draft, tokens)
	if err != nil {
		t.Fatalf("buildCreateDraftRequest error: %v", err)
	}
	body := req["Body"].(map[string]interface{})
	if body["MessageDisposition"] != "SaveOnly" {
		t.Fatalf("MessageDisposition = %v", body["MessageDisposition"])
	}
	items := body["Items"].([]map[string]interface{})
	if items[0]["Subject"] != "Hello" {
		t.Fatalf("Subject = %v", items[0]["Subject"])
	}
}

func TestBuildUpdateDraftRequest(t *testing.T) {
	req, err := buildUpdateDraftRequest("draft-1", &Draft{Subject: "New"})
	if err != nil {
		t.Fatalf("buildUpdateDraftRequest error: %v", err)
	}
	if req["__type"] != "UpdateItemJsonRequest:#Exchange" {
		t.Fatalf("__type = %v", req["__type"])
	}
	body := req["Body"].(map[string]interface{})
	if body["__type"] != "UpdateItemRequest:#Exchange" {
		t.Fatalf("body __type = %v", body["__type"])
	}
	changes := body["ItemChanges"].([]map[string]interface{})
	itemID := changes[0]["ItemId"].(map[string]interface{})
	if itemID["Id"] != "draft-1" {
		t.Fatalf("ItemId = %v", itemID["Id"])
	}
}

func TestLimitSearchResults(t *testing.T) {
	res := &SearchResult{
		Messages:      []Message{{ID: "1"}, {ID: "2"}, {ID: "3"}},
		Conversations: []Conversation{{ID: "c1"}, {ID: "c2"}},
	}
	limitSearchResults(res, 2)
	if len(res.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(res.Messages))
	}
	if len(res.Conversations) != 2 {
		t.Fatalf("conversations = %d, want 2", len(res.Conversations))
	}
}

func TestExtractMessagesFromResponse_ResponseMessages(t *testing.T) {
	body := []byte(`{"Body":{"ResponseMessages":{"Items":[{"Items":[{"ItemId":"id-1","Subject":"Hi"}]}]}}}`)
	msgs, err := extractMessagesFromResponse(body)
	if err != nil {
		t.Fatalf("extractMessagesFromResponse error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
	if msgs[0].ID != "id-1" {
		t.Fatalf("message ID = %q", msgs[0].ID)
	}
}

func TestExtractMessagesFromResponse_BodyItems(t *testing.T) {
	body := []byte(`{"Body":{"Items":[{"ItemId":"id-2","Subject":"Fallback"}]}}`)
	msgs, err := extractMessagesFromResponse(body)
	if err != nil {
		t.Fatalf("extractMessagesFromResponse error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
	if msgs[0].ID != "id-2" {
		t.Fatalf("message ID = %q", msgs[0].ID)
	}
}

func TestExtractMessagesFromResponse_InvalidJSON(t *testing.T) {
	_, err := extractMessagesFromResponse([]byte(`{"Body":`))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestExtractMessagesFromResponse_Empty(t *testing.T) {
	_, err := extractMessagesFromResponse(nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildGetFolderRequest(t *testing.T) {
	req := buildGetFolderRequest("inbox")
	if req["__type"] != "GetFolderRequest:#Exchange" {
		t.Fatalf("__type = %v", req["__type"])
	}
	shape := req["FolderShape"].(map[string]interface{})
	if shape["BaseShape"] != "IdOnly" {
		t.Fatalf("BaseShape = %v", shape["BaseShape"])
	}
	ids := req["FolderIds"].([]map[string]interface{})
	if got := ids[0]["__type"]; got != "DistinguishedFolderId:#Exchange" {
		t.Fatalf("FolderIds[0].__type = %v", got)
	}
	if got := ids[0]["Id"]; got != "inbox" {
		t.Fatalf("FolderIds[0].Id = %v", got)
	}
}
