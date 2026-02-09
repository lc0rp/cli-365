package owa

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

const graphMessagesEndpoint = "https://graph.microsoft.com/v1.0/me/messages"

func buildGraphConversationURL(conversationID string, maxResults int) (string, error) {
	if strings.TrimSpace(conversationID) == "" {
		return "", errors.New("conversation ID required")
	}
	if maxResults <= 0 {
		maxResults = 20
	}
	values := url.Values{}
	values.Set("$select", "sender,from,toRecipients,ccRecipients,bccRecipients,subject,body,bodyPreview,sentDateTime,receivedDateTime,conversationId,isRead,hasAttachments,importance")
	values.Set("$top", strconv.Itoa(maxResults))
	values.Set("$filter", fmt.Sprintf("isDraft eq false and conversationId eq '%s'", conversationID))
	return graphMessagesEndpoint + "?" + values.Encode(), nil
}

func fetchConversationMessagesViaGraph(page *rod.Page, tokens *Tokens, conversationID string, maxResults int) ([]Message, error) {
	if page == nil {
		return nil, errors.New("page is nil")
	}
	if tokens == nil {
		return nil, errors.New("tokens are nil")
	}
	bearer := tokens.GraphBearer
	if bearer == "" {
		bearer = tokens.Bearer
	}
	if bearer == "" {
		return nil, errors.New("missing bearer token")
	}
	endpoint, err := buildGraphConversationURL(conversationID, maxResults)
	if err != nil {
		return nil, err
	}
	return fetchGraphConversationMessagesHTTP(endpoint, bearer)
}

func fetchConversationMessageIDsHTTP(endpoint string, bearer string) ([]string, error) {
	if strings.TrimSpace(bearer) == "" {
		return nil, errors.New("missing bearer token")
	}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", bearer)
	req.Header.Set("Accept", "application/json")
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		errBody := string(body)
		if len(errBody) > 2048 {
			errBody = errBody[:2048] + "...(truncated)"
		}
		return nil, fmt.Errorf("graph conversation fetch failed with status %d: %s", resp.StatusCode, errBody)
	}
	return parseSubstrateMessageIDs(body)
}

type graphMessagesResponse struct {
	Value []graphMessage `json:"value"`
}

type graphMessage struct {
	ID             string           `json:"id"`
	ConversationID string           `json:"conversationId"`
	Subject        string           `json:"subject"`
	BodyPreview    string           `json:"bodyPreview"`
	Body           graphMessageBody `json:"body"`
	From           graphRecipient   `json:"from"`
	Sender         graphRecipient   `json:"sender"`
	ToRecipients   []graphRecipient `json:"toRecipients"`
	CcRecipients   []graphRecipient `json:"ccRecipients"`
	BccRecipients  []graphRecipient `json:"bccRecipients"`
	SentDateTime   string           `json:"sentDateTime"`
	ReceivedDate   string           `json:"receivedDateTime"`
	IsRead         bool             `json:"isRead"`
	HasAttachments bool             `json:"hasAttachments"`
	Importance     string           `json:"importance"`
}

type graphMessageBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type graphRecipient struct {
	EmailAddress graphEmailAddress `json:"emailAddress"`
}

type graphEmailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

func fetchGraphConversationMessagesHTTP(endpoint string, bearer string) ([]Message, error) {
	if strings.TrimSpace(bearer) == "" {
		return nil, errors.New("missing bearer token")
	}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", bearer)
	req.Header.Set("Accept", "application/json")
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		errBody := string(body)
		if len(errBody) > 2048 {
			errBody = errBody[:2048] + "...(truncated)"
		}
		return nil, fmt.Errorf("graph conversation fetch failed with status %d: %s", resp.StatusCode, errBody)
	}
	var parsed graphMessagesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse graph response: %w", err)
	}
	messages := make([]Message, 0, len(parsed.Value))
	for _, msg := range parsed.Value {
		messages = append(messages, graphMessageToMessage(msg))
	}
	return messages, nil
}

func graphMessageToMessage(msg graphMessage) Message {
	result := Message{
		ID:               msg.ID,
		ConversationID:   msg.ConversationID,
		Subject:          msg.Subject,
		BodyPreview:      msg.BodyPreview,
		DateTimeSent:     msg.SentDateTime,
		DateTimeReceived: msg.ReceivedDate,
		IsRead:           msg.IsRead,
		HasAttachments:   msg.HasAttachments,
		Importance:       msg.Importance,
	}
	if msg.Body.Content != "" {
		result.Body = &MessageBody{
			BodyType: graphBodyType(msg.Body.ContentType),
			Value:    msg.Body.Content,
		}
	}
	if addr := graphRecipientToEmailAddress(msg.From); addr != nil {
		result.From = addr
	}
	if addr := graphRecipientToEmailAddress(msg.Sender); addr != nil {
		result.Sender = addr
	}
	result.ToRecipients = graphRecipientList(msg.ToRecipients)
	result.CcRecipients = graphRecipientList(msg.CcRecipients)
	result.BccRecipients = graphRecipientList(msg.BccRecipients)
	return result
}

func graphRecipientToEmailAddress(rec graphRecipient) *EmailAddress {
	if strings.TrimSpace(rec.EmailAddress.Address) == "" {
		return nil
	}
	return &EmailAddress{
		Name:        rec.EmailAddress.Name,
		Address:     rec.EmailAddress.Address,
		RoutingType: "SMTP",
	}
}

func graphRecipientList(list []graphRecipient) []EmailAddress {
	if len(list) == 0 {
		return nil
	}
	out := make([]EmailAddress, 0, len(list))
	for _, rec := range list {
		addr := graphRecipientToEmailAddress(rec)
		if addr == nil {
			continue
		}
		out = append(out, *addr)
	}
	return out
}

func graphBodyType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "html":
		return "HTML"
	case "text":
		return "Text"
	default:
		return contentType
	}
}
