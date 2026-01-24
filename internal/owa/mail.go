package owa

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-rod/rod"
)

// SearchMessages searches for messages using OWA FindItem.
func SearchMessages(page *rod.Page, canary string, query string, folderID string, maxResults int) (*SearchResult, error) {
	body := buildSearchMessagesBody(query, folderID, maxResults)

	resp, err := CallOWAAction(page, canary, "FindItem", body)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("search failed with status %d: %s", resp.Status, resp.StatusText)
	}

	result, err := UnmarshalSearchResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	return result, nil
}

// SearchMessagesWithBearer searches for messages using bearer auth only.
func SearchMessagesWithBearer(page *rod.Page, bearer string, query string, folderID string, maxResults int) (*SearchResult, error) {
	body := buildSearchMessagesBody(query, folderID, maxResults)

	resp, err := CallOWAActionWithBearer(page, bearer, "FindItem", body)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("search failed with status %d: %s", resp.Status, resp.StatusText)
	}

	result, err := UnmarshalSearchResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	return result, nil
}

func buildSearchMessagesBody(query string, folderID string, maxResults int) map[string]interface{} {
	if maxResults <= 0 {
		maxResults = 50
	}

	body := map[string]interface{}{
		"ItemShape": map[string]interface{}{
			"BaseShape": "IdOnly",
			"AdditionalProperties": []map[string]interface{}{
				{"FieldURI": "item:Subject"},
				{"FieldURI": "item:DateTimeReceived"},
				{"FieldURI": "item:DateTimeSent"},
				{"FieldURI": "message:From"},
				{"FieldURI": "message:ToRecipients"},
				{"FieldURI": "item:HasAttachments"},
				{"FieldURI": "item:Importance"},
				{"FieldURI": "message:IsRead"},
				{"FieldURI": "item:Preview"},
			},
		},
		"Paging": map[string]interface{}{
			"__type":             "IndexedPageView:#Exchange",
			"BasePoint":          "Beginning",
			"Offset":             0,
			"MaxEntriesReturned": maxResults,
		},
		"ViewFilter":        "All",
		"FocusedViewFilter": -1,
		"SortOrder": []map[string]interface{}{
			{
				"Order": "Descending",
				"Path": map[string]interface{}{
					"__type":   "PropertyUri:#Exchange",
					"FieldURI": "item:DateTimeReceived",
				},
			},
		},
	}

	if folderID != "" {
		body["ParentFolderIds"] = []map[string]interface{}{
			{"__type": "FolderId:#Exchange", "Id": folderID},
		}
	} else {
		body["ParentFolderIds"] = []map[string]interface{}{
			{"__type": "DistinguishedFolderId:#Exchange", "Id": "inbox"},
		}
	}

	if query != "" {
		body["Restriction"] = map[string]interface{}{
			"__type":                "Contains:#Exchange",
			"ContainmentMode":       "Substring",
			"ContainmentComparison": "IgnoreCase",
			"Item": map[string]interface{}{
				"__type":   "PropertyUri:#Exchange",
				"FieldURI": "item:Subject",
			},
			"Constant": map[string]interface{}{
				"__type": "ConstantValue:#Exchange",
				"Value":  query,
			},
		}
	}

	return body
}

// SearchConversations searches for conversations.
func SearchConversations(page *rod.Page, canary string, query string, folderID string, maxResults int) (*SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 50
	}

	body := map[string]interface{}{
		"ConversationShape": map[string]interface{}{
			"BaseShape": "IdOnly",
		},
		"Paging": map[string]interface{}{
			"__type":             "IndexedPageView:#Exchange",
			"BasePoint":          "Beginning",
			"Offset":             0,
			"MaxEntriesReturned": maxResults,
		},
		"ViewFilter": "All",
		"SortOrder": []map[string]interface{}{
			{
				"Order": "Descending",
				"Path": map[string]interface{}{
					"__type":   "PropertyUri:#Exchange",
					"FieldURI": "conversation:LastDeliveryTime",
				},
			},
		},
	}

	if folderID != "" {
		body["ParentFolderIds"] = []map[string]interface{}{
			{"__type": "FolderId:#Exchange", "Id": folderID},
		}
	} else {
		body["ParentFolderIds"] = []map[string]interface{}{
			{"__type": "DistinguishedFolderId:#Exchange", "Id": "inbox"},
		}
	}

	if query != "" {
		body["QueryString"] = query
	}

	resp, err := CallOWAAction(page, canary, "FindConversation", body)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("search conversations failed with status %d: %s", resp.Status, resp.StatusText)
	}

	result, err := UnmarshalSearchResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse conversation response: %w", err)
	}

	return result, nil
}

// GetMessage retrieves a single message by ID.
func GetMessage(page *rod.Page, canary string, messageID string) (*Message, error) {
	body := map[string]interface{}{
		"ItemShape": map[string]interface{}{
			"BaseShape":          "Default",
			"IncludeMimeContent": false,
			"AdditionalProperties": []map[string]interface{}{
				{"FieldURI": "item:Body"},
				{"FieldURI": "item:Attachments"},
			},
		},
		"ItemIds": []map[string]interface{}{
			{"__type": "ItemId:#Exchange", "Id": messageID},
		},
	}

	resp, err := CallOWAAction(page, canary, "GetItem", body)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("get message failed with status %d: %s", resp.Status, resp.StatusText)
	}

	// Parse response
	var wrapper struct {
		Body struct {
			Items []Message `json:"Items"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(resp.Body, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse message response: %w", err)
	}

	if len(wrapper.Body.Items) == 0 {
		return nil, errors.New("message not found")
	}

	return &wrapper.Body.Items[0], nil
}

// GetConversation retrieves all messages in a conversation.
func GetConversation(page *rod.Page, canary string, conversationID string, folderID string) (*Conversation, error) {
	body := map[string]interface{}{
		"ItemShape": map[string]interface{}{
			"BaseShape": "Default",
			"AdditionalProperties": []map[string]interface{}{
				{"FieldURI": "item:Body"},
				{"FieldURI": "item:Attachments"},
			},
		},
		"MaxItemsToReturn": 100,
		"SortOrder":        "DateOrderAscending",
	}

	if folderID != "" {
		body["Conversations"] = []map[string]interface{}{
			{
				"ConversationId": map[string]interface{}{
					"__type": "ItemId:#Exchange",
					"Id":     conversationID,
				},
				"SyncState": nil,
			},
		}
		body["FoldersToIgnore"] = []map[string]interface{}{}
	} else {
		body["Conversations"] = []map[string]interface{}{
			{
				"ConversationId": map[string]interface{}{
					"__type": "ItemId:#Exchange",
					"Id":     conversationID,
				},
			},
		}
	}

	resp, err := CallOWAAction(page, canary, "GetConversationItems", body)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("get conversation failed with status %d: %s", resp.Status, resp.StatusText)
	}

	// Parse response
	var wrapper struct {
		Body struct {
			Conversations []struct {
				ConversationId struct {
					Id string `json:"Id"`
				} `json:"ConversationId"`
				SyncState string    `json:"SyncState"`
				Items     []Message `json:"Items"`
			} `json:"Conversations"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(resp.Body, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse conversation response: %w", err)
	}

	if len(wrapper.Body.Conversations) == 0 {
		return nil, errors.New("conversation not found")
	}

	conv := &Conversation{
		ID:       wrapper.Body.Conversations[0].ConversationId.Id,
		Messages: wrapper.Body.Conversations[0].Items,
	}
	if len(conv.Messages) > 0 {
		conv.Topic = conv.Messages[0].Subject
	}

	return conv, nil
}

// CreateDraft creates a new draft message.
func CreateDraft(page *rod.Page, canary string, draft *Draft) (*Message, error) {
	body := map[string]interface{}{
		"Items": []map[string]interface{}{
			{
				"__type":  "Message:#Exchange",
				"Subject": draft.Subject,
			},
		},
		"MessageDisposition": "SaveOnly",
	}

	item := body["Items"].([]map[string]interface{})[0]

	if draft.Body != nil {
		item["Body"] = map[string]interface{}{
			"__type":   "BodyContentType:#Exchange",
			"BodyType": draft.Body.BodyType,
			"Value":    draft.Body.Value,
		}
	}

	if len(draft.ToRecipients) > 0 {
		recipients := make([]map[string]interface{}, len(draft.ToRecipients))
		for i, r := range draft.ToRecipients {
			recipients[i] = map[string]interface{}{
				"Name":         r.Name,
				"EmailAddress": r.Address,
				"RoutingType":  "SMTP",
				"MailboxType":  "Mailbox",
			}
		}
		item["ToRecipients"] = recipients
	}

	if len(draft.CcRecipients) > 0 {
		recipients := make([]map[string]interface{}, len(draft.CcRecipients))
		for i, r := range draft.CcRecipients {
			recipients[i] = map[string]interface{}{
				"Name":         r.Name,
				"EmailAddress": r.Address,
				"RoutingType":  "SMTP",
			}
		}
		item["CcRecipients"] = recipients
	}

	if draft.Importance != "" {
		item["Importance"] = draft.Importance
	}

	resp, err := CallOWAAction(page, canary, "CreateItem", body)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("create draft failed with status %d: %s", resp.Status, resp.StatusText)
	}

	var wrapper struct {
		Body struct {
			Items []Message `json:"Items"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(resp.Body, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse create response: %w", err)
	}

	if len(wrapper.Body.Items) == 0 {
		return nil, errors.New("draft creation returned no items")
	}

	return &wrapper.Body.Items[0], nil
}

// UpdateDraft updates an existing draft message.
func UpdateDraft(page *rod.Page, canary string, draftID string, draft *Draft) (*Message, error) {
	changes := []map[string]interface{}{}

	if draft.Subject != "" {
		changes = append(changes, map[string]interface{}{
			"__type": "SetItemField:#Exchange",
			"Path": map[string]interface{}{
				"__type":   "PropertyUri:#Exchange",
				"FieldURI": "item:Subject",
			},
			"Item": map[string]interface{}{
				"__type":  "Message:#Exchange",
				"Subject": draft.Subject,
			},
		})
	}

	if draft.Body != nil {
		changes = append(changes, map[string]interface{}{
			"__type": "SetItemField:#Exchange",
			"Path": map[string]interface{}{
				"__type":   "PropertyUri:#Exchange",
				"FieldURI": "item:Body",
			},
			"Item": map[string]interface{}{
				"__type": "Message:#Exchange",
				"Body": map[string]interface{}{
					"BodyType": draft.Body.BodyType,
					"Value":    draft.Body.Value,
				},
			},
		})
	}

	if len(draft.ToRecipients) > 0 {
		recipients := make([]map[string]interface{}, len(draft.ToRecipients))
		for i, r := range draft.ToRecipients {
			recipients[i] = map[string]interface{}{
				"Name":         r.Name,
				"EmailAddress": r.Address,
				"RoutingType":  "SMTP",
			}
		}
		changes = append(changes, map[string]interface{}{
			"__type": "SetItemField:#Exchange",
			"Path": map[string]interface{}{
				"__type":   "PropertyUri:#Exchange",
				"FieldURI": "message:ToRecipients",
			},
			"Item": map[string]interface{}{
				"__type":       "Message:#Exchange",
				"ToRecipients": recipients,
			},
		})
	}

	body := map[string]interface{}{
		"ItemChanges": []map[string]interface{}{
			{
				"ItemId": map[string]interface{}{
					"__type": "ItemId:#Exchange",
					"Id":     draftID,
				},
				"Updates": changes,
			},
		},
		"MessageDisposition": "SaveOnly",
	}

	resp, err := CallOWAAction(page, canary, "UpdateItem", body)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("update draft failed with status %d: %s", resp.Status, resp.StatusText)
	}

	return GetMessage(page, canary, draftID)
}

// DeleteDraft deletes a draft message.
func DeleteDraft(page *rod.Page, canary string, draftID string) error {
	body := map[string]interface{}{
		"ItemIds": []map[string]interface{}{
			{"__type": "ItemId:#Exchange", "Id": draftID},
		},
		"DeleteType": "MoveToDeletedItems",
	}

	resp, err := CallOWAAction(page, canary, "DeleteItem", body)
	if err != nil {
		return err
	}

	if resp.Status != 200 {
		return fmt.Errorf("delete draft failed with status %d: %s", resp.Status, resp.StatusText)
	}

	return nil
}

// SendDraft sends an existing draft.
func SendDraft(page *rod.Page, canary string, draftID string) error {
	body := map[string]interface{}{
		"ItemIds": []map[string]interface{}{
			{"__type": "ItemId:#Exchange", "Id": draftID},
		},
		"SaveItemToFolder": true,
		"SavedItemFolderId": map[string]interface{}{
			"__type": "DistinguishedFolderId:#Exchange",
			"Id":     "sentitems",
		},
	}

	resp, err := CallOWAAction(page, canary, "SendItem", body)
	if err != nil {
		return err
	}

	if resp.Status != 200 {
		return fmt.Errorf("send draft failed with status %d: %s", resp.Status, resp.StatusText)
	}

	return nil
}

// SendMessage creates and sends a message in one operation.
func SendMessage(page *rod.Page, canary string, draft *Draft) error {
	body := map[string]interface{}{
		"Items": []map[string]interface{}{
			{
				"__type":  "Message:#Exchange",
				"Subject": draft.Subject,
			},
		},
		"MessageDisposition": "SendAndSaveCopy",
	}

	item := body["Items"].([]map[string]interface{})[0]

	if draft.Body != nil {
		item["Body"] = map[string]interface{}{
			"__type":   "BodyContentType:#Exchange",
			"BodyType": draft.Body.BodyType,
			"Value":    draft.Body.Value,
		}
	}

	if len(draft.ToRecipients) > 0 {
		recipients := make([]map[string]interface{}, len(draft.ToRecipients))
		for i, r := range draft.ToRecipients {
			recipients[i] = map[string]interface{}{
				"Name":         r.Name,
				"EmailAddress": r.Address,
				"RoutingType":  "SMTP",
				"MailboxType":  "Mailbox",
			}
		}
		item["ToRecipients"] = recipients
	}

	if len(draft.CcRecipients) > 0 {
		recipients := make([]map[string]interface{}, len(draft.CcRecipients))
		for i, r := range draft.CcRecipients {
			recipients[i] = map[string]interface{}{
				"Name":         r.Name,
				"EmailAddress": r.Address,
				"RoutingType":  "SMTP",
			}
		}
		item["CcRecipients"] = recipients
	}

	if draft.Importance != "" {
		item["Importance"] = draft.Importance
	}

	resp, err := CallOWAAction(page, canary, "CreateItem", body)
	if err != nil {
		return err
	}

	if resp.Status != 200 {
		return fmt.Errorf("send message failed with status %d: %s", resp.Status, resp.StatusText)
	}

	return nil
}

// ListAttachments lists attachments for a message.
func ListAttachments(page *rod.Page, canary string, messageID string) ([]Attachment, error) {
	msg, err := GetMessage(page, canary, messageID)
	if err != nil {
		return nil, err
	}

	return msg.Attachments, nil
}

// GetAttachment retrieves attachment content.
func GetAttachment(page *rod.Page, canary string, attachmentID string) ([]byte, string, error) {
	body := map[string]interface{}{
		"AttachmentShape": map[string]interface{}{
			"IncludeMimeContent": true,
		},
		"AttachmentIds": []map[string]interface{}{
			{"__type": "AttachmentId:#Exchange", "Id": attachmentID},
		},
	}

	resp, err := CallOWAAction(page, canary, "GetAttachment", body)
	if err != nil {
		return nil, "", err
	}

	if resp.Status != 200 {
		return nil, "", fmt.Errorf("get attachment failed with status %d: %s", resp.Status, resp.StatusText)
	}

	var wrapper struct {
		Body struct {
			Attachments []struct {
				ContentType string `json:"ContentType"`
				Name        string `json:"Name"`
				Content     string `json:"Content"`
			} `json:"Attachments"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(resp.Body, &wrapper); err != nil {
		return nil, "", fmt.Errorf("failed to parse attachment response: %w", err)
	}

	if len(wrapper.Body.Attachments) == 0 {
		return nil, "", errors.New("attachment not found")
	}

	att := wrapper.Body.Attachments[0]
	content, err := base64.StdEncoding.DecodeString(att.Content)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode attachment content: %w", err)
	}

	return content, att.Name, nil
}
