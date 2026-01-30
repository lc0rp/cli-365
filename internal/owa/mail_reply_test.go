package owa

import "testing"

func TestBuildReplyRequest(t *testing.T) {
	body := &MessageBody{BodyType: "Text", Value: "Thanks"}
	msg := &Message{
		ChangeKey: "ck-1",
		Subject:   "Hello",
		From:      &EmailAddress{Name: "Sender", Address: "sender@example.com"},
	}
	tokens := &Tokens{UserEmail: "me@example.com"}
	req, err := buildReplyRequest("msg-1", msg, tokens, body, false, true)
	if err != nil {
		t.Fatalf("buildReplyRequest error: %v", err)
	}
	if req["__type"] != "CreateItemJsonRequest:#Exchange" {
		t.Fatalf("__type = %v, want CreateItemJsonRequest:#Exchange", req["__type"])
	}
	bodyMap := req["Body"].(map[string]interface{})
	items := bodyMap["Items"].([]map[string]interface{})
	item := items[0]
	if item["__type"] != "ReplyToItem:#Exchange" {
		t.Fatalf("reply type = %v, want ReplyToItem:#Exchange", item["__type"])
	}
	ref := item["ReferenceItemId"].(map[string]interface{})
	if ref["Id"] != "msg-1" {
		t.Fatalf("ReferenceItemId = %v, want msg-1", ref["Id"])
	}
	if ref["ChangeKey"] != "ck-1" {
		t.Fatalf("ChangeKey = %v, want ck-1", ref["ChangeKey"])
	}
	newBody := item["NewBodyContent"].(map[string]interface{})
	if newBody["Value"] != "Thanks" {
		t.Fatalf("NewBodyContent.Value = %v, want Thanks", newBody["Value"])
	}

	req, err = buildReplyRequest("msg-2", msg, tokens, body, true, true)
	if err != nil {
		t.Fatalf("buildReplyRequest error: %v", err)
	}
	bodyMap = req["Body"].(map[string]interface{})
	items = bodyMap["Items"].([]map[string]interface{})
	item = items[0]
	if item["__type"] != "ReplyAllToItem:#Exchange" {
		t.Fatalf("reply type = %v, want ReplyAllToItem:#Exchange", item["__type"])
	}
}

func TestBuildReplyRequestMissingID(t *testing.T) {
	if _, err := buildReplyRequest("", nil, nil, &MessageBody{Value: "x"}, false, true); err == nil {
		t.Fatalf("expected error for empty message ID")
	}
}
