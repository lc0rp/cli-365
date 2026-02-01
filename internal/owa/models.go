package owa

import (
	"encoding/json"
	"time"
)

// Message represents an email message.
type Message struct {
	ID                string         `json:"ItemId,omitempty"`
	ChangeKey         string         `json:"ChangeKey,omitempty"`
	ConversationID    string         `json:"ConversationId,omitempty"`
	Subject           string         `json:"Subject,omitempty"`
	BodyPreview       string         `json:"BodyPreview,omitempty"`
	Body              *MessageBody   `json:"Body,omitempty"`
	From              *EmailAddress  `json:"From,omitempty"`
	Sender            *EmailAddress  `json:"Sender,omitempty"`
	ToRecipients      []EmailAddress `json:"ToRecipients,omitempty"`
	CcRecipients      []EmailAddress `json:"CcRecipients,omitempty"`
	BccRecipients     []EmailAddress `json:"BccRecipients,omitempty"`
	DateTimeReceived  string         `json:"DateTimeReceived,omitempty"`
	DateTimeSent      string         `json:"DateTimeSent,omitempty"`
	DateTimeCreated   string         `json:"DateTimeCreated,omitempty"`
	IsRead            bool           `json:"IsRead,omitempty"`
	IsDraft           bool           `json:"IsDraft,omitempty"`
	HasAttachments    bool           `json:"HasAttachments,omitempty"`
	Importance        string         `json:"Importance,omitempty"`
	Categories        []string       `json:"Categories,omitempty"`
	Flag              *MessageFlag   `json:"Flag,omitempty"`
	Attachments       []Attachment   `json:"Attachments,omitempty"`
	InternetMessageId string         `json:"InternetMessageId,omitempty"`
	ParentFolderId    string         `json:"ParentFolderId,omitempty"`
}

// UnmarshalJSON supports ItemId as string or {Id, ChangeKey}.
func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias Message
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var id string
	var changeKey string
	var conversationID string
	var parentFolderID string
	if v, ok := raw["ItemId"]; ok {
		if err := json.Unmarshal(v, &id); err != nil {
			var item ItemID
			if err := json.Unmarshal(v, &item); err == nil {
				id = item.ID
				if item.ChangeKey != "" {
					changeKey = item.ChangeKey
				}
			}
		}
		delete(raw, "ItemId")
	}
	if v, ok := raw["ConversationId"]; ok {
		if err := json.Unmarshal(v, &conversationID); err != nil {
			var item ItemID
			if err := json.Unmarshal(v, &item); err == nil {
				conversationID = item.ID
			}
		}
		delete(raw, "ConversationId")
	}
	if v, ok := raw["ParentFolderId"]; ok {
		if err := json.Unmarshal(v, &parentFolderID); err != nil {
			var item ItemID
			if err := json.Unmarshal(v, &item); err == nil {
				parentFolderID = item.ID
			}
		}
		delete(raw, "ParentFolderId")
	}
	if v, ok := raw["ChangeKey"]; ok {
		var ck string
		if err := json.Unmarshal(v, &ck); err == nil && ck != "" {
			changeKey = ck
		}
		delete(raw, "ChangeKey")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*m = Message(alias)
	if id != "" {
		m.ID = id
	}
	if changeKey != "" {
		m.ChangeKey = changeKey
	}
	if conversationID != "" {
		m.ConversationID = conversationID
	}
	if parentFolderID != "" {
		m.ParentFolderId = parentFolderID
	}
	return nil
}

// MessageBody represents the body of a message.
type MessageBody struct {
	BodyType string `json:"BodyType,omitempty"` // "Text" or "HTML"
	Value    string `json:"Value,omitempty"`
}

// EmailAddress represents an email address with optional name.
type EmailAddress struct {
	Name        string `json:"Name,omitempty"`
	Address     string `json:"EmailAddress,omitempty"`
	RoutingType string `json:"RoutingType,omitempty"`
	MailboxType string `json:"MailboxType,omitempty"`
}

// MessageFlag represents message flag status.
type MessageFlag struct {
	FlagStatus string `json:"FlagStatus,omitempty"` // "NotFlagged", "Flagged", "Complete"
}

// Attachment represents an email attachment.
type Attachment struct {
	ID          string `json:"AttachmentId,omitempty"`
	Name        string `json:"Name,omitempty"`
	ContentType string `json:"ContentType,omitempty"`
	Size        int64  `json:"Size,omitempty"`
	IsInline    bool   `json:"IsInline,omitempty"`
	ContentID   string `json:"ContentId,omitempty"`
}

// UnmarshalJSON supports AttachmentId as string or {Id}.
func (a *Attachment) UnmarshalJSON(data []byte) error {
	type Alias Attachment
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var id string
	if v, ok := raw["AttachmentId"]; ok {
		if err := json.Unmarshal(v, &id); err != nil {
			var item ItemID
			if err := json.Unmarshal(v, &item); err == nil {
				id = item.ID
			}
		}
		delete(raw, "AttachmentId")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*a = Attachment(alias)
	if id != "" {
		a.ID = id
	}
	return nil
}

// Conversation represents an email conversation/thread.
type Conversation struct {
	ID                 string    `json:"ConversationId,omitempty"`
	Topic              string    `json:"ConversationTopic,omitempty"`
	LastDeliveryTime   string    `json:"LastDeliveryTime,omitempty"`
	UnreadCount        int       `json:"UnreadCount,omitempty"`
	MessageCount       int       `json:"MessageCount,omitempty"`
	GlobalMessageCount int       `json:"GlobalMessageCount,omitempty"`
	HasAttachments     bool      `json:"HasAttachments,omitempty"`
	Importance         string    `json:"Importance,omitempty"`
	Categories         []string  `json:"Categories,omitempty"`
	UniqueRecipients   []string  `json:"UniqueRecipients,omitempty"`
	UniqueSenders      []string  `json:"UniqueSenders,omitempty"`
	LastModifiedTime   string    `json:"LastModifiedTime,omitempty"`
	Preview            string    `json:"Preview,omitempty"`
	Messages           []Message `json:"Messages,omitempty"`
}

// Folder represents a mail folder.
type Folder struct {
	ID               string `json:"FolderId,omitempty"`
	DisplayName      string `json:"DisplayName,omitempty"`
	ParentFolderID   string `json:"ParentFolderId,omitempty"`
	ChildFolderCount int    `json:"ChildFolderCount,omitempty"`
	TotalCount       int    `json:"TotalCount,omitempty"`
	UnreadCount      int    `json:"UnreadCount,omitempty"`
	FolderClass      string `json:"FolderClass,omitempty"`
}

// SearchResult represents a search result.
type SearchResult struct {
	TotalCount    int            `json:"TotalCount,omitempty"`
	Messages      []Message      `json:"Messages,omitempty"`
	Conversations []Conversation `json:"Conversations,omitempty"`
}

// Draft represents a draft message for creation/update.
type Draft struct {
	Subject         string         `json:"Subject,omitempty"`
	Body            *MessageBody   `json:"Body,omitempty"`
	ToRecipients    []EmailAddress `json:"ToRecipients,omitempty"`
	CcRecipients    []EmailAddress `json:"CcRecipients,omitempty"`
	BccRecipients   []EmailAddress `json:"BccRecipients,omitempty"`
	Importance      string         `json:"Importance,omitempty"`
	SaveToSentItems bool           `json:"SaveToSentItems,omitempty"`
}

// ItemID is the standard OWA item ID format.
type ItemID struct {
	ID        string `json:"Id"`
	ChangeKey string `json:"ChangeKey,omitempty"`
}

// FolderID is the standard OWA folder ID format.
type FolderID struct {
	ID        string `json:"Id"`
	ChangeKey string `json:"ChangeKey,omitempty"`
}

// DistinguishedFolderID represents a well-known folder.
type DistinguishedFolderID struct {
	ID string `json:"Id"` // "inbox", "drafts", "sentitems", "deleteditems", etc.
}

// ParseTime parses an OWA datetime string.
func ParseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000Z",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, nil
}

// UnmarshalSearchResponse parses a search response from OWA.
func UnmarshalSearchResponse(data json.RawMessage) (*SearchResult, error) {
	// OWA search responses can have various shapes
	// Try to extract messages/conversations from common locations

	var result SearchResult

	// First try direct unmarshal
	if err := json.Unmarshal(data, &result); err == nil && (len(result.Messages) > 0 || len(result.Conversations) > 0) {
		return &result, nil
	}

	// Try Body wrapper
	var bodyWrapper struct {
		Body struct {
			SearchResult     *SearchResult  `json:"SearchResult"`
			Items            []Message      `json:"Items"`
			Conversations    []Conversation `json:"Conversations"`
			TotalItemsInView int            `json:"TotalItemsInView"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(data, &bodyWrapper); err == nil {
		if bodyWrapper.Body.SearchResult != nil {
			return bodyWrapper.Body.SearchResult, nil
		}
		result.Messages = bodyWrapper.Body.Items
		result.Conversations = bodyWrapper.Body.Conversations
		result.TotalCount = bodyWrapper.Body.TotalItemsInView
		return &result, nil
	}

	// Try Items array directly
	var itemsWrapper struct {
		Items []Message `json:"Items"`
	}
	if err := json.Unmarshal(data, &itemsWrapper); err == nil {
		result.Messages = itemsWrapper.Items
		return &result, nil
	}

	return &result, nil
}
