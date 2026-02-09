package owa

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-rod/rod"
)

const substrateMessagesEndpoint = "https://substrate.office.com/api/beta/me/messages"

func buildSubstrateConversationURL(conversationID string, maxResults int) (string, error) {
	if strings.TrimSpace(conversationID) == "" {
		return "", errors.New("conversation ID required")
	}
	if maxResults <= 0 {
		maxResults = 20
	}
	values := url.Values{}
	values.Set("$select", "sender,sentDateTime,toRecipients")
	values.Set("$top", strconv.Itoa(maxResults))
	values.Set("$filter", fmt.Sprintf("(isDraft eq false and conversationId eq '%s')", conversationID))
	return substrateMessagesEndpoint + "?" + values.Encode(), nil
}

type conversationMessagesResponse struct {
	Value []struct {
		ID      string `json:"Id"`
		IDLower string `json:"id"`
	} `json:"value"`
}

func parseSubstrateMessageIDs(body json.RawMessage) ([]string, error) {
	if len(body) == 0 {
		return nil, errors.New("empty response")
	}
	var resp conversationMessagesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse substrate response: %w", err)
	}
	ids := make([]string, 0, len(resp.Value))
	for _, item := range resp.Value {
		id := item.ID
		if id == "" {
			id = item.IDLower
		}
		if strings.TrimSpace(id) == "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func fetchConversationMessagesViaSubstrate(page *rod.Page, tokens *Tokens, conversationID string, maxResults int) ([]Message, error) {
	if page == nil {
		return nil, errors.New("page is nil")
	}
	if tokens == nil {
		return nil, errors.New("tokens are nil")
	}
	bearer := tokens.Substrate
	if bearer == "" {
		bearer = tokens.GraphBearer
	}
	if bearer == "" {
		bearer = tokens.Bearer
	}
	if bearer == "" {
		return nil, errors.New("missing bearer token")
	}
	endpoint, err := buildSubstrateConversationURL(conversationID, maxResults)
	if err != nil {
		return nil, err
	}
	ids, err := fetchConversationMessageIDsHTTP(endpoint, bearer)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, 0, len(ids))
	var firstErr error
	for _, id := range ids {
		msg, err := GetMessage(page, tokens, id)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		messages = append(messages, *msg)
	}
	if len(messages) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return messages, nil
}
