package owa

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-rod/rod"
)

// SearchProvider selects the search backend.
type SearchProvider string

const (
	SearchProviderAuto          SearchProvider = "auto"
	SearchProviderOWA           SearchProvider = "owa"
	SearchProviderSearchService SearchProvider = "searchservice"
)

// SearchMessages searches for messages using the default provider (auto).
func SearchMessages(page *rod.Page, tokens *Tokens, query string, folderID string, maxResults int) (*SearchResult, error) {
	return SearchMessagesWithProvider(page, tokens, query, folderID, maxResults, SearchProviderAuto)
}

// SearchMessagesWithProvider searches for messages using the selected provider.
func SearchMessagesWithProvider(page *rod.Page, tokens *Tokens, query string, folderID string, maxResults int, provider SearchProvider) (*SearchResult, error) {
	switch provider {
	case SearchProviderSearchService:
		res, err := SearchMessagesSearchService(page, tokens, query, folderID, maxResults)
		if res != nil {
			limitSearchResults(res, maxResults)
		}
		return res, err
	case SearchProviderOWA:
		res, err := searchMessagesOWA(page, tokens, query, folderID, maxResults)
		if res != nil {
			limitSearchResults(res, maxResults)
		}
		return res, err
	case SearchProviderAuto:
		if res, err := SearchMessagesSearchService(page, tokens, query, folderID, maxResults); err == nil {
			limitSearchResults(res, maxResults)
			return res, nil
		}
		res, err := searchMessagesOWA(page, tokens, query, folderID, maxResults)
		if res != nil {
			limitSearchResults(res, maxResults)
		}
		return res, err
	default:
		return nil, fmt.Errorf("unknown search provider: %s", provider)
	}
}

// searchMessagesOWA searches for messages using OWA FindItem.
func searchMessagesOWA(page *rod.Page, tokens *Tokens, query string, folderID string, maxResults int) (*SearchResult, error) {
	resolved, err := resolveFolderInput(page, tokens, folderID)
	if err != nil {
		return nil, err
	}
	folderID = resolved
	body := buildSearchMessagesBody(query, folderID, maxResults)

	resp, err := CallOWAAction(page, tokens, "FindItem", body)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("search failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
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
		"__type": "FindItemRequest:#Exchange",
		"ItemShape": map[string]interface{}{
			"BaseShape": "IdOnly",
			"AdditionalProperties": []map[string]interface{}{
				{"FieldURI": "item:Subject"},
				{"FieldURI": "item:ConversationId"},
				{"FieldURI": "item:DateTimeReceived"},
				{"FieldURI": "item:DateTimeSent"},
				{"FieldURI": "message:From"},
				{"FieldURI": "message:ToRecipients"},
				{"FieldURI": "item:HasAttachments"},
				{"FieldURI": "item:Importance"},
				{"FieldURI": "message:IsRead"},
				{"FieldURI": "item:Preview"},
				{"FieldURI": "item:ParentFolderId"},
			},
		},
		"Paging": map[string]interface{}{
			"__type":             "IndexedPageView:#Exchange",
			"BasePoint":          "Beginning",
			"Offset":             0,
			"MaxEntriesReturned": maxResults,
		},
		"Traversal":  "Shallow",
		"ViewFilter": "All",
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

func resolveFolderID(page *rod.Page, tokens *Tokens, distinguished string) (string, error) {
	if tokens == nil || distinguished == "" {
		return "", nil
	}
	if tokens.Folders != nil {
		if id := tokens.Folders[distinguished]; id != "" {
			return id, nil
		}
	}
	if page == nil {
		return "", errors.New("page not initialized")
	}

	body := buildGetFolderRequest(distinguished)

	resp, err := CallOWAAction(page, tokens, "GetFolder", body)
	if err != nil {
		return "", err
	}
	if resp.Status != 200 {
		return "", fmt.Errorf("get folder failed with status %d: %s", resp.Status, resp.StatusText)
	}

	var wrapper struct {
		Body struct {
			Folders []struct {
				FolderId struct {
					Id string `json:"Id"`
				} `json:"FolderId"`
			} `json:"Folders"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(resp.Body, &wrapper); err != nil {
		return "", fmt.Errorf("failed to parse folder response: %w", err)
	}
	if len(wrapper.Body.Folders) == 0 {
		return "", nil
	}
	resolvedID := wrapper.Body.Folders[0].FolderId.Id
	rememberFolderID(tokens, distinguished, resolvedID)
	return resolvedID, nil
}

func rememberFolderID(tokens *Tokens, distinguished string, folderID string) {
	if tokens == nil || distinguished == "" || folderID == "" {
		return
	}
	if tokens.Folders == nil {
		tokens.Folders = map[string]string{}
	}
	tokens.Folders[distinguished] = folderID
}

func buildGetFolderRequest(distinguished string) map[string]interface{} {
	return map[string]interface{}{
		"__type": "GetFolderRequest:#Exchange",
		"FolderShape": map[string]interface{}{
			"BaseShape": "IdOnly",
		},
		"FolderIds": []map[string]interface{}{
			{"__type": "DistinguishedFolderId:#Exchange", "Id": distinguished},
		},
	}
}

func resolveFolderInput(page *rod.Page, tokens *Tokens, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}
	if name, ok := normalizeFolderName(input); ok {
		return resolveFolderID(page, tokens, name)
	}
	return input, nil
}

func normalizeFolderName(raw string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(raw))
	key = strings.ReplaceAll(key, " ", "")
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, "_", "")
	switch key {
	case "inbox":
		return "inbox", true
	case "draft", "drafts":
		return "drafts", true
	case "sent", "sentitems", "sentitem", "sentmail":
		return "sentitems", true
	case "deleted", "deleteditems", "trash", "bin":
		return "deleteditems", true
	case "junk", "junkmail", "junkemail", "spam":
		return "junkemail", true
	case "archive", "archives":
		return "archive", true
	case "outbox":
		return "outbox", true
	case "calendar", "cal":
		return "calendar", true
	default:
		return "", false
	}
}

// SearchConversations searches for conversations.
func SearchConversations(page *rod.Page, tokens *Tokens, query string, folderID string, maxResults int) (*SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 50
	}
	resolved, err := resolveFolderInput(page, tokens, folderID)
	if err != nil {
		return nil, err
	}
	folderID = resolved

	body := map[string]interface{}{
		"__type": "FindConversationRequest:#Exchange",
		"ConversationShape": map[string]interface{}{
			"BaseShape": "IdOnly",
		},
		"Paging": map[string]interface{}{
			"__type":             "IndexedPageView:#Exchange",
			"BasePoint":          "Beginning",
			"Offset":             0,
			"MaxEntriesReturned": maxResults,
		},
		"Traversal":  "Shallow",
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

	resp, err := CallOWAAction(page, tokens, "FindConversation", body)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("search conversations failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}

	result, err := UnmarshalSearchResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse conversation response: %w", err)
	}

	return result, nil
}

// GetMessage retrieves a single message by ID.
func GetMessage(page *rod.Page, tokens *Tokens, messageID string) (*Message, error) {
	reqBody, err := buildGetItemRequest(messageID)
	if err != nil {
		return nil, err
	}
	resp, err := CallOWAAction(page, tokens, "GetItem", reqBody)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("get message failed with status %d: %s", resp.Status, resp.StatusText)
	}

	// Parse response
	items, err := extractMessagesFromResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("message not found")
	}

	return &items[0], nil
}

// GetConversation retrieves all messages in a conversation.
func GetConversation(page *rod.Page, tokens *Tokens, conversationID string, folderID string) (*Conversation, error) {
	reqBody, err := buildGetConversationItemsRequest(conversationID, folderID, tokens)
	if err != nil {
		return nil, err
	}
	resp, err := CallOWAAction(page, tokens, "GetConversationItems", reqBody)
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
		messages, ferr := fetchConversationMessagesViaGraph(page, tokens, conversationID, 20)
		if ferr != nil {
			messages, ferr = fetchConversationMessagesViaSubstrate(page, tokens, conversationID, 20)
			if ferr != nil {
				return nil, ferr
			}
		}
		if len(messages) == 0 {
			return nil, errors.New("conversation not found")
		}
		conv := &Conversation{
			ID:       conversationID,
			Messages: messages,
		}
		if len(conv.Messages) > 0 {
			conv.Topic = conv.Messages[0].Subject
		}
		return conv, nil
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
func CreateDraft(page *rod.Page, tokens *Tokens, draft *Draft) (*Message, error) {
	reqBody, err := buildCreateDraftRequest(draft, tokens)
	if err != nil {
		return nil, err
	}
	resp, err := CallOWAAction(page, tokens, "CreateItem", reqBody)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("create draft failed with status %d: %s", resp.Status, resp.StatusText)
	}

	items, err := extractMessagesFromResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("draft creation returned no items")
	}

	return &items[0], nil
}

// UpdateDraft updates an existing draft message.
func UpdateDraft(page *rod.Page, tokens *Tokens, draftID string, draft *Draft) (*Message, error) {
	reqBody, err := buildUpdateDraftRequest(draftID, draft)
	if err != nil {
		return nil, err
	}
	resp, err := CallOWAAction(page, tokens, "UpdateItem", reqBody)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("update draft failed with status %d: %s", resp.Status, resp.StatusText)
	}

	return GetMessage(page, tokens, draftID)
}

func formatOWAErrorDetails(resp *FetchResponse) string {
	if resp == nil {
		return "unknown error"
	}
	info := parseOWAError(resp.Body)
	parts := []string{}
	if info.Code != "" {
		parts = append(parts, info.Code)
	}
	if info.Exception != "" {
		parts = append(parts, info.Exception)
	}
	if info.Message != "" {
		parts = append(parts, info.Message)
	}
	body := summarizeOWAErrorBody(resp.Body)
	if body != "" && (info.Code != "" || info.Exception != "" || info.Message != "") {
		parts = append(parts, "body="+body)
	}
	if body != "" && len(parts) == 0 {
		parts = append(parts, body)
	}
	if len(parts) == 0 {
		if resp.StatusText != "" {
			return resp.StatusText
		}
		return "unknown error"
	}
	return strings.Join(parts, " | ")
}

func summarizeOWAErrorBody(body json.RawMessage) string {
	if len(body) == 0 {
		return ""
	}

	// If the response body is a JSON string, prefer the decoded string value.
	// Some error responses are binary/garbage (often NULs) and should not be printed verbatim.
	var decoded string
	if err := json.Unmarshal(body, &decoded); err == nil {
		if decoded == "" {
			return ""
		}
		nul := strings.Count(decoded, "\x00")
		if nul > 0 {
			return fmt.Sprintf("<body: string len=%d nul=%d>", len(decoded), nul)
		}
		trimmed := strings.TrimSpace(decoded)
		if trimmed == "" {
			return ""
		}
		if len(trimmed) > 2048 {
			trimmed = trimmed[:2048] + "...(truncated)"
		}
		return trimmed
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > 2048 {
		trimmed = trimmed[:2048] + "...(truncated)"
	}
	return trimmed
}

// DeleteDraft deletes a draft message.
func DeleteDraft(page *rod.Page, tokens *Tokens, draftID string) error {
	body := map[string]interface{}{
		"ItemIds": []map[string]interface{}{
			{"__type": "ItemId:#Exchange", "Id": draftID},
		},
		"DeleteType": "MoveToDeletedItems",
	}

	resp, err := CallOWAAction(page, tokens, "DeleteItem", body)
	if err != nil {
		return err
	}

	if resp.Status != 200 {
		return fmt.Errorf("delete draft failed with status %d: %s", resp.Status, resp.StatusText)
	}

	return nil
}

// SendDraft sends an existing draft.
func SendDraft(page *rod.Page, tokens *Tokens, draftID string) error {
	return SendDraftWithItem(page, tokens, ItemID{ID: draftID})
}

func SendDraftWithItem(page *rod.Page, tokens *Tokens, item ItemID) error {
	if strings.TrimSpace(item.ID) == "" {
		return errors.New("draft ID required")
	}
	itemID := map[string]interface{}{
		"__type": "ItemId:#Exchange",
		"Id":     item.ID,
	}
	if strings.TrimSpace(item.ChangeKey) != "" {
		itemID["ChangeKey"] = item.ChangeKey
	}
	body := map[string]interface{}{
		"ItemIds":          []map[string]interface{}{itemID},
		"SaveItemToFolder": true,
		"SavedItemFolderId": map[string]interface{}{
			"__type": "DistinguishedFolderId:#Exchange",
			"Id":     "sentitems",
		},
	}

	resp, err := CallOWAAction(page, tokens, "SendItem", body)
	if err != nil {
		return err
	}

	if resp.Status != 200 {
		return fmt.Errorf("send draft failed with status %d: %s", resp.Status, resp.StatusText)
	}

	return nil
}

// SendMessage creates and sends a message in one operation.
func SendMessage(page *rod.Page, tokens *Tokens, draft *Draft) error {
	reqBody, err := buildSendRequest(draft, tokens)
	if err != nil {
		return err
	}
	resp, err := CallOWAAction(page, tokens, "CreateItem", reqBody)
	if err != nil {
		return err
	}

	if resp.Status != 200 {
		return fmt.Errorf("send message failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}

	return nil
}

// ReplyToMessage sends a reply (or reply-all) to an existing message.
func ReplyToMessage(page *rod.Page, tokens *Tokens, messageID string, body *MessageBody, replyAll bool) error {
	var msg *Message
	if page != nil && tokens != nil {
		if fetched, err := GetMessage(page, tokens, messageID); err == nil {
			msg = fetched
		}
	}
	reqBody, err := buildReplyRequest(messageID, msg, tokens, body, replyAll, true)
	if err != nil {
		return err
	}

	resp, err := CallOWAAction(page, tokens, "CreateItem", reqBody)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		if resp.Status == 500 && isSerializationError(resp.Body) {
			if page != nil {
				if msg == nil || msg.ChangeKey == "" {
					if fetched, err := GetMessage(page, tokens, messageID); err == nil {
						msg = fetched
					}
				}
				if msg != nil {
					reqBody, err = buildReplyRequest(messageID, msg, tokens, body, replyAll, false)
					if err == nil {
						if retry, rerr := CallOWAAction(page, tokens, "CreateItem", reqBody); rerr == nil {
							if retry.Status == 200 {
								if item := extractCreatedItem(retry.Body); item.ID != "" {
									if sendErr := SendDraftWithItem(page, tokens, item); sendErr != nil {
										return sendErr
									}
								}
								return nil
							}
							return fmt.Errorf("reply failed with status %d: %s", retry.Status, formatOWAErrorDetails(retry))
						}
					}
				}
			}
		}
		return fmt.Errorf("reply failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}
	return nil
}

func buildReplyRequest(messageID string, msg *Message, tokens *Tokens, body *MessageBody, replyAll bool, sendNow bool) (map[string]interface{}, error) {
	if strings.TrimSpace(messageID) == "" {
		return nil, errors.New("message ID required")
	}
	itemType := "ReplyToItem:#Exchange"
	if replyAll {
		itemType = "ReplyAllToItem:#Exchange"
	}
	ref := map[string]interface{}{
		"__type": "ItemId:#Exchange",
		"Id":     messageID,
	}
	if msg != nil && strings.TrimSpace(msg.ChangeKey) != "" {
		ref["ChangeKey"] = msg.ChangeKey
	}
	item := map[string]interface{}{
		"__type":                     itemType,
		"ReferenceItemId":            ref,
		"MessageDisposition":         "SaveOnly",
		"IsSendIndividually":         false,
		"Importance":                 "Normal",
		"Sensitivity":                "Normal",
		"IsDeliveryReceiptRequested": false,
		"IsReadReceiptRequested":     false,
		"ShouldIgnoreChangeKey":      true,
	}
	if body != nil && strings.TrimSpace(body.Value) != "" {
		bodyType := body.BodyType
		if bodyType == "" {
			bodyType = "HTML"
		}
		newBody := map[string]interface{}{
			"__type":   "BodyContentType:#Exchange",
			"BodyType": bodyType,
			"Value":    body.Value,
		}
		if strings.EqualFold(bodyType, "HTML") {
			if count := countDataURIs(body.Value); count > 0 {
				newBody["DataUriCount"] = count
			}
		}
		item["NewBodyContent"] = newBody
	}
	if msg != nil && strings.TrimSpace(msg.Subject) != "" {
		item["Subject"] = formatReplySubject(msg.Subject)
	}
	if recipients := buildReplyRecipients(msg, tokens, replyAll); len(recipients) > 0 {
		item["ToRecipients"] = recipients
	}
	if tokens != nil {
		if mailbox := buildMailboxInfo(tokens); mailbox != nil {
			item["mailboxInfo"] = mailbox
			item["referenceItemMailboxInfo"] = mailbox
		}
		if sendAs := buildSendAs(tokens, msg); sendAs != nil {
			item["sendAs"] = sendAs
			item["From"] = map[string]interface{}{
				"Mailbox": sendAs,
			}
		}
	}
	if replyAll {
		item["operation"] = "ReplyAll"
		item["draftComposeType"] = "replyAll"
	} else {
		item["operation"] = "Reply"
		item["draftComposeType"] = "reply"
	}

	messageDisposition := "SaveOnly"
	if sendNow {
		messageDisposition = "SendAndSaveCopy"
		item["MessageDisposition"] = "SendAndSaveCopy"
	}

	req := map[string]interface{}{
		"__type": "CreateItemJsonRequest:#Exchange",
		"Header": map[string]interface{}{
			"__type":               "JsonRequestHeaders:#Exchange",
			"RequestServerVersion": "V2018_01_08",
		},
		"Body": map[string]interface{}{
			"__type":              "CreateItemRequest:#Exchange",
			"ClientSupportsIrm":   true,
			"ComposeOperation":    "newMail",
			"MessageDisposition":  messageDisposition,
			"Items":               []map[string]interface{}{item},
			"SendOnNotFoundError": true,
			"RemoteExecute":       false,
			"TimeFormat":          "h:mm tt",
			"ShapeName":           "MailCompose",
			"ItemShape": map[string]interface{}{
				"__type":    "ItemResponseShape:#Exchange",
				"BaseShape": "IdOnly",
				"BodyType":  "HTML",
			},
			"ShouldSuppressReadReceipt":          true,
			"SuppressMarkAsReadOnReplyOrForward": true,
			"OutboundCharset":                    "AutoDetect",
			"UseGB18030":                         false,
			"UseISO885915":                       false,
		},
	}
	return req, nil
}

func buildGetItemRequest(messageID string) (map[string]interface{}, error) {
	if strings.TrimSpace(messageID) == "" {
		return nil, errors.New("message ID required")
	}
	req := map[string]interface{}{
		"__type": "GetItemJsonRequest:#Exchange",
		"Header": buildJsonRequestHeader(),
		"Body": map[string]interface{}{
			"__type": "GetItemRequest:#Exchange",
			"ItemShape": map[string]interface{}{
				"__type":             "ItemResponseShape:#Exchange",
				"BaseShape":          "Default",
				"IncludeMimeContent": false,
				"BodyType":           "HTML",
				"AdditionalProperties": []map[string]interface{}{
					{"__type": "PropertyUri:#Exchange", "FieldURI": "item:Body"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "item:Attachments"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "item:ConversationId"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "item:ParentFolderId"},
				},
			},
			"ItemIds": []map[string]interface{}{
				{"__type": "ItemId:#Exchange", "Id": messageID},
			},
		},
	}
	return req, nil
}

func buildGetConversationItemsRequest(conversationID string, folderID string, tokens *Tokens) (map[string]interface{}, error) {
	if strings.TrimSpace(conversationID) == "" {
		return nil, errors.New("conversation ID required")
	}
	conversation := map[string]interface{}{
		"ConversationId": map[string]interface{}{
			"__type": "ConversationId:#Exchange",
			"Id":     conversationID,
		},
	}
	if folderID != "" {
		conversation["SyncState"] = nil
		conversation["ParentFolderId"] = map[string]interface{}{
			"__type": "FolderId:#Exchange",
			"Id":     folderID,
		}
	}
	body := map[string]interface{}{
		"__type":           "GetConversationItemsRequest:#Exchange",
		"MaxItemsToReturn": 100,
		"SortOrder":        "DateOrderAscending",
		"ItemShape": map[string]interface{}{
			"__type":    "ItemResponseShape:#Exchange",
			"BaseShape": "Default",
			"BodyType":  "HTML",
			"AdditionalProperties": []map[string]interface{}{
				{"__type": "PropertyUri:#Exchange", "FieldURI": "item:Body"},
				{"__type": "PropertyUri:#Exchange", "FieldURI": "item:Attachments"},
			},
		},
		"Conversations": []map[string]interface{}{conversation},
	}
	if tokens != nil {
		if mailbox := buildMailboxInfo(tokens); mailbox != nil {
			body["MailboxInfo"] = mailbox
		}
	}
	if folderID != "" {
		body["FoldersToIgnore"] = []map[string]interface{}{}
	}
	return map[string]interface{}{
		"__type": "GetConversationItemsJsonRequest:#Exchange",
		"Header": buildJsonRequestHeader(),
		"Body":   body,
	}, nil
}

func buildUpdateDraftRequest(draftID string, draft *Draft) (map[string]interface{}, error) {
	if strings.TrimSpace(draftID) == "" {
		return nil, errors.New("draft ID required")
	}
	if draft == nil {
		return nil, errors.New("draft is required")
	}
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
		recipients := buildDraftRecipients(draft.ToRecipients)
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
		"__type":             "UpdateItemRequest:#Exchange",
		"MessageDisposition": "SaveOnly",
		"ConflictResolution": "AlwaysOverwrite",
		"ItemChanges": []map[string]interface{}{
			{
				"ItemId": map[string]interface{}{
					"__type": "ItemId:#Exchange",
					"Id":     draftID,
				},
				"Updates": changes,
			},
		},
	}
	return map[string]interface{}{
		"__type": "UpdateItemJsonRequest:#Exchange",
		"Header": buildJsonRequestHeader(),
		"Body":   body,
	}, nil
}

func buildJsonRequestHeader() map[string]interface{} {
	return map[string]interface{}{
		"__type":               "JsonRequestHeaders:#Exchange",
		"RequestServerVersion": "V2018_01_08",
	}
}

func limitSearchResults(result *SearchResult, maxResults int) {
	if result == nil || maxResults <= 0 {
		return
	}
	if len(result.Messages) > maxResults {
		result.Messages = result.Messages[:maxResults]
	}
	if len(result.Conversations) > maxResults {
		result.Conversations = result.Conversations[:maxResults]
	}
}

func buildSendRequest(draft *Draft, tokens *Tokens) (map[string]interface{}, error) {
	return buildMessageRequest(draft, tokens, "SendAndSaveCopy")
}

func buildCreateDraftRequest(draft *Draft, tokens *Tokens) (map[string]interface{}, error) {
	return buildMessageRequest(draft, tokens, "SaveOnly")
}

func buildMessageRequest(draft *Draft, tokens *Tokens, disposition string) (map[string]interface{}, error) {
	if draft == nil {
		return nil, errors.New("draft is required")
	}
	if len(draft.ToRecipients) == 0 && len(draft.CcRecipients) == 0 && len(draft.BccRecipients) == 0 {
		return nil, errors.New("at least one recipient is required")
	}
	if disposition == "" {
		disposition = "SaveOnly"
	}

	item := map[string]interface{}{
		"__type":             "Message:#Exchange",
		"MessageDisposition": disposition,
		"IsSendIndividually": false,
		"Importance":         "Normal",
		"Sensitivity":        "Normal",
	}
	if strings.TrimSpace(draft.Subject) != "" {
		item["Subject"] = draft.Subject
	}
	if draft.Importance != "" {
		item["Importance"] = draft.Importance
	}
	if draft.Body != nil && strings.TrimSpace(draft.Body.Value) != "" {
		bodyType := draft.Body.BodyType
		if bodyType == "" {
			bodyType = "HTML"
		}
		body := map[string]interface{}{
			"BodyType": bodyType,
			"Value":    draft.Body.Value,
		}
		if strings.EqualFold(bodyType, "HTML") {
			if count := countDataURIs(draft.Body.Value); count > 0 {
				body["DataUriCount"] = count
			}
		}
		item["Body"] = body
	}
	if recipients := buildDraftRecipients(draft.ToRecipients); len(recipients) > 0 {
		item["ToRecipients"] = recipients
	}
	if recipients := buildDraftRecipients(draft.CcRecipients); len(recipients) > 0 {
		item["CcRecipients"] = recipients
	}
	if recipients := buildDraftRecipients(draft.BccRecipients); len(recipients) > 0 {
		item["BccRecipients"] = recipients
	}
	if tokens != nil {
		if mailbox := buildMailboxInfo(tokens); mailbox != nil {
			item["mailboxInfo"] = mailbox
		}
		if sendAs := buildSendAs(tokens, nil); sendAs != nil {
			item["sendAs"] = sendAs
			item["From"] = map[string]interface{}{
				"Mailbox": sendAs,
			}
		}
	}

	req := map[string]interface{}{
		"__type": "CreateItemJsonRequest:#Exchange",
		"Header": map[string]interface{}{
			"__type":               "JsonRequestHeaders:#Exchange",
			"RequestServerVersion": "V2018_01_08",
		},
		"Body": map[string]interface{}{
			"__type":              "CreateItemRequest:#Exchange",
			"ClientSupportsIrm":   true,
			"ComposeOperation":    "newMail",
			"MessageDisposition":  disposition,
			"Items":               []map[string]interface{}{item},
			"SendOnNotFoundError": true,
			"RemoteExecute":       false,
			"TimeFormat":          "h:mm tt",
			"ShapeName":           "MailCompose",
			"ItemShape": map[string]interface{}{
				"__type":    "ItemResponseShape:#Exchange",
				"BaseShape": "IdOnly",
				"BodyType":  "HTML",
			},
			"ShouldSuppressReadReceipt":          true,
			"SuppressMarkAsReadOnReplyOrForward": true,
			"OutboundCharset":                    "AutoDetect",
			"UseGB18030":                         false,
			"UseISO885915":                       false,
		},
	}
	return req, nil
}

func isSerializationError(body json.RawMessage) bool {
	info := parseOWAError(body)
	return strings.EqualFold(info.Exception, "OwaSerializationException") ||
		strings.Contains(strings.ToLower(info.Code), "errorinternalservererror")
}

func extractCreatedItem(body json.RawMessage) ItemID {
	if len(body) == 0 {
		return ItemID{}
	}
	var resp struct {
		Body struct {
			ResponseMessages struct {
				Items []struct {
					Items []struct {
						ItemID ItemID `json:"ItemId"`
					} `json:"Items"`
				} `json:"Items"`
			} `json:"ResponseMessages"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ItemID{}
	}
	for _, item := range resp.Body.ResponseMessages.Items {
		for _, msg := range item.Items {
			if msg.ItemID.ID != "" {
				return msg.ItemID
			}
		}
	}
	return ItemID{}
}

func extractMessagesFromResponse(body json.RawMessage) ([]Message, error) {
	if len(body) == 0 {
		return nil, errors.New("empty response")
	}
	var resp struct {
		Body struct {
			ResponseMessages struct {
				Items []struct {
					Items []Message `json:"Items"`
				} `json:"Items"`
			} `json:"ResponseMessages"`
			Items []Message `json:"Items"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse message response: %w", err)
	}
	out := []Message{}
	for _, item := range resp.Body.ResponseMessages.Items {
		if len(item.Items) > 0 {
			out = append(out, item.Items...)
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	if len(resp.Body.Items) > 0 {
		return resp.Body.Items, nil
	}
	return []Message{}, nil
}

func buildReplyRecipients(msg *Message, tokens *Tokens, replyAll bool) []map[string]interface{} {
	if msg == nil {
		return nil
	}
	self := ""
	if tokens != nil && strings.Contains(tokens.UserEmail, "@") {
		self = strings.ToLower(tokens.UserEmail)
	}
	seen := map[string]bool{}
	add := func(addr EmailAddress, out *[]map[string]interface{}) {
		if strings.TrimSpace(addr.Address) == "" {
			return
		}
		email := strings.ToLower(addr.Address)
		if self != "" && email == self {
			return
		}
		if seen[email] {
			return
		}
		seen[email] = true
		rt := addr.RoutingType
		if rt == "" {
			rt = "SMTP"
		}
		mt := addr.MailboxType
		if mt == "" {
			mt = "OneOff"
		}
		*out = append(*out, map[string]interface{}{
			"Name":         addr.Name,
			"EmailAddress": addr.Address,
			"RoutingType":  rt,
			"MailboxType":  mt,
		})
	}
	out := []map[string]interface{}{}
	if msg.Sender != nil {
		add(*msg.Sender, &out)
	} else if msg.From != nil {
		add(*msg.From, &out)
	}
	if replyAll {
		for _, addr := range msg.ToRecipients {
			add(addr, &out)
		}
		for _, addr := range msg.CcRecipients {
			add(addr, &out)
		}
	}
	return out
}

func buildDraftRecipients(list []EmailAddress) []map[string]interface{} {
	if len(list) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, addr := range list {
		if strings.TrimSpace(addr.Address) == "" {
			continue
		}
		rt := addr.RoutingType
		if rt == "" {
			rt = "SMTP"
		}
		mt := addr.MailboxType
		if mt == "" {
			mt = "OneOff"
		}
		out = append(out, map[string]interface{}{
			"Name":         addr.Name,
			"EmailAddress": addr.Address,
			"RoutingType":  rt,
			"MailboxType":  mt,
		})
	}
	return out
}

func buildMailboxInfo(tokens *Tokens) map[string]interface{} {
	mailboxAddress := mailboxSMTPAddress(tokens)
	if mailboxAddress == "" {
		return nil
	}
	userIdentity := strings.TrimSpace(tokens.UserEmail)
	if userIdentity == "" {
		userIdentity = mailboxAddress
	}
	return map[string]interface{}{
		"type":               "UserMailbox",
		"mailboxSmtpAddress": mailboxAddress,
		"userIdentity":       userIdentity,
		"mailboxRank":        "Coprincipal",
		"mailboxProvider":    "Office365",
	}
}

func mailboxSMTPAddress(tokens *Tokens) string {
	if tokens == nil {
		return ""
	}
	userEmail := strings.TrimSpace(tokens.UserEmail)
	if strings.Contains(userEmail, "@") {
		return userEmail
	}
	anchor := strings.TrimSpace(tokens.Session.AnchorMailbox)
	if anchor == "" {
		return mailboxFromBearerTokens(tokens)
	}
	if idx := strings.Index(anchor, ":"); idx >= 0 && idx+1 < len(anchor) {
		anchor = anchor[idx+1:]
	}
	anchor = strings.TrimSpace(anchor)
	if strings.Contains(anchor, "@") {
		return anchor
	}
	return mailboxFromBearerTokens(tokens)
}

func mailboxFromBearerTokens(tokens *Tokens) string {
	if tokens == nil {
		return ""
	}
	for _, raw := range []string{tokens.Bearer, tokens.GraphBearer, tokens.Substrate} {
		if email := mailboxFromBearerToken(raw); email != "" {
			return email
		}
	}
	return ""
}

func mailboxFromBearerToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		raw = strings.TrimSpace(raw[len("bearer "):])
	}
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return ""
	}
	payloadSeg := strings.TrimSpace(parts[1])
	if payloadSeg == "" {
		return ""
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadSeg)
	if err != nil {
		payloadBytes, err = base64.URLEncoding.DecodeString(payloadSeg)
		if err != nil {
			return ""
		}
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return ""
	}
	for _, key := range []string{"preferred_username", "upn", "email", "unique_name"} {
		if value, ok := claims[key].(string); ok {
			value = strings.TrimSpace(value)
			if strings.Contains(value, "@") {
				return value
			}
		}
	}
	if values, ok := claims["emails"].([]interface{}); ok {
		for _, value := range values {
			if email, ok := value.(string); ok {
				email = strings.TrimSpace(email)
				if strings.Contains(email, "@") {
					return email
				}
			}
		}
	}
	return ""
}

func buildSendAs(tokens *Tokens, msg *Message) map[string]interface{} {
	if tokens == nil || !strings.Contains(tokens.UserEmail, "@") {
		return nil
	}
	return map[string]interface{}{
		"MailboxType":  "Mailbox",
		"RoutingType":  "SMTP",
		"EmailAddress": tokens.UserEmail,
		"Name":         tokens.UserEmail,
	}
}

func formatReplySubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return subject
	}
	lower := strings.ToLower(subject)
	if strings.HasPrefix(lower, "re:") {
		return subject
	}
	return "Re: " + subject
}

func countDataURIs(value string) int {
	if value == "" {
		return 0
	}
	return strings.Count(strings.ToLower(value), "data:")
}

// ListAttachments lists attachments for a message.
func ListAttachments(page *rod.Page, tokens *Tokens, messageID string) ([]Attachment, error) {
	msg, err := GetMessage(page, tokens, messageID)
	if err != nil {
		return nil, err
	}

	return msg.Attachments, nil
}

// GetAttachment retrieves attachment content.
func GetAttachment(page *rod.Page, tokens *Tokens, attachmentID string) ([]byte, string, error) {
	body := map[string]interface{}{
		"AttachmentShape": map[string]interface{}{
			"IncludeMimeContent": true,
		},
		"AttachmentIds": []map[string]interface{}{
			{"__type": "AttachmentId:#Exchange", "Id": attachmentID},
		},
	}

	resp, err := CallOWAAction(page, tokens, "GetAttachment", body)
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
