package owa

import "testing"

func TestBuildSendRequest(t *testing.T) {
	draft := &Draft{
		Subject: "Hello",
		Body:    &MessageBody{BodyType: "Text", Value: "Body"},
		ToRecipients: []EmailAddress{
			{Name: "Alice", Address: "alice@example.com"},
		},
	}
	tokens := &Tokens{UserEmail: "me@example.com"}
	req, err := buildSendRequest(draft, tokens)
	if err != nil {
		t.Fatalf("buildSendRequest error: %v", err)
	}
	if req["__type"] != "CreateItemJsonRequest:#Exchange" {
		t.Fatalf("__type = %v", req["__type"])
	}
	body := req["Body"].(map[string]interface{})
	if body["MessageDisposition"] != "SendAndSaveCopy" {
		t.Fatalf("MessageDisposition = %v", body["MessageDisposition"])
	}
	items := body["Items"].([]map[string]interface{})
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	item := items[0]
	if item["__type"] != "Message:#Exchange" {
		t.Fatalf("item __type = %v", item["__type"])
	}
	if item["Subject"] != "Hello" {
		t.Fatalf("Subject = %v", item["Subject"])
	}
	to := item["ToRecipients"].([]map[string]interface{})
	if len(to) != 1 {
		t.Fatalf("to recipients = %d", len(to))
	}
}

func TestBuildSendRequestMissingRecipients(t *testing.T) {
	if _, err := buildSendRequest(&Draft{Subject: "Hi"}, &Tokens{UserEmail: "me@example.com"}); err == nil {
		t.Fatalf("expected error for missing recipients")
	}
}
