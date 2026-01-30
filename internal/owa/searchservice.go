package owa

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

type searchServiceResponse struct {
	EntitySets []struct {
		ResultSets []struct {
			ResultCount          int                   `json:"ResultCount"`
			Total                int                   `json:"Total"`
			TotalCount           int                   `json:"TotalCount"`
			MoreResultsAvailable bool                  `json:"MoreResultsAvailable"`
			Results              []searchServiceResult `json:"Results"`
		} `json:"ResultSets"`
	} `json:"EntitySets"`
}

type searchServiceResult struct {
	Type   string              `json:"Type"`
	Source searchServiceSource `json:"Source"`
}

type searchServiceSource struct {
	ItemId struct {
		Id string `json:"Id"`
	} `json:"ItemId"`
	ImmutableId    string `json:"ImmutableId"`
	ParentFolderId struct {
		Id string `json:"Id"`
	} `json:"ParentFolderId"`
	ConversationId struct {
		Id string `json:"Id"`
	} `json:"ConversationId"`
	ConversationTopic  string   `json:"ConversationTopic"`
	LastDeliveryTime   string   `json:"LastDeliveryTime"`
	LastModifiedTime   string   `json:"LastModifiedTime"`
	UnreadCount        int      `json:"UnreadCount"`
	MessageCount       int      `json:"MessageCount"`
	GlobalMessageCount int      `json:"GlobalMessageCount"`
	HasAttachments     bool     `json:"HasAttachments"`
	Importance         string   `json:"Importance"`
	Preview            string   `json:"Preview"`
	UniqueRecipients   []string `json:"UniqueRecipients"`
	UniqueSenders      []string `json:"UniqueSenders"`
	SenderSMTPAddress  string   `json:"SenderSMTPAddress"`
	From               struct {
		EmailAddress struct {
			Name    string `json:"Name"`
			Address string `json:"Address"`
		} `json:"EmailAddress"`
	} `json:"From"`
}

func SearchMessagesSearchService(page *rod.Page, tokens *Tokens, query string, folderID string, maxResults int) (*SearchResult, error) {
	if page == nil || tokens == nil {
		return nil, fmt.Errorf("page or tokens are nil")
	}
	if maxResults <= 0 {
		maxResults = 50
	}
	origin := searchServiceOrigin(page)
	if origin == "" {
		return nil, fmt.Errorf("search service origin unavailable")
	}

	filter := buildSearchServiceFilter(folderID)
	reqBody := buildSearchServiceBody(query, maxResults, filter)
	headers := map[string]string{
		"Accept":       "*/*",
		"Content-Type": "application/json",
		"Prefer":       "IdType=\"ImmutableId\"",
		"ScenarioTag":  "1stPg_cv",
		"X-Req-Source": "Mail",
	}
	applySearchHeaders(headers, tokens, origin)
	applyPreferHeader(headers)

	req := FetchRequest{
		URL:     origin + "/searchservice/api/v2/query",
		Method:  "POST",
		Headers: headers,
		Body:    reqBody,
	}

	resp, err := Fetch(page, req)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("search service failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}

	var payload searchServiceResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse search service response: %w", err)
	}

	return searchServiceToResult(payload), nil
}

func searchServiceToResult(payload searchServiceResponse) *SearchResult {
	result := &SearchResult{}
	if len(payload.EntitySets) == 0 || len(payload.EntitySets[0].ResultSets) == 0 {
		return result
	}
	rs := payload.EntitySets[0].ResultSets[0]
	switch {
	case rs.Total > 0:
		result.TotalCount = rs.Total
	case rs.TotalCount > 0:
		result.TotalCount = rs.TotalCount
	case rs.ResultCount > 0:
		result.TotalCount = rs.ResultCount
	default:
		result.TotalCount = len(rs.Results)
	}

	for _, item := range rs.Results {
		if strings.EqualFold(item.Type, "Conversation") {
			result.Conversations = append(result.Conversations, Conversation{
				ID:                 item.Source.ConversationId.Id,
				Topic:              item.Source.ConversationTopic,
				LastDeliveryTime:   item.Source.LastDeliveryTime,
				LastModifiedTime:   item.Source.LastModifiedTime,
				UnreadCount:        item.Source.UnreadCount,
				MessageCount:       item.Source.MessageCount,
				GlobalMessageCount: item.Source.GlobalMessageCount,
				HasAttachments:     item.Source.HasAttachments,
				Importance:         item.Source.Importance,
				Preview:            item.Source.Preview,
				UniqueRecipients:   item.Source.UniqueRecipients,
				UniqueSenders:      item.Source.UniqueSenders,
			})
		}
		if msg := searchServiceMessageFromItem(item); msg != nil {
			result.Messages = append(result.Messages, *msg)
		}
	}

	return result
}

func searchServiceMessageFromItem(item searchServiceResult) *Message {
	id := item.Source.ItemId.Id
	if id == "" {
		id = item.Source.ImmutableId
	}
	if id == "" {
		return nil
	}
	msg := &Message{
		ID:               id,
		ConversationID:   item.Source.ConversationId.Id,
		Subject:          item.Source.ConversationTopic,
		BodyPreview:      item.Source.Preview,
		DateTimeReceived: item.Source.LastDeliveryTime,
		HasAttachments:   item.Source.HasAttachments,
		Importance:       item.Source.Importance,
		ParentFolderId:   item.Source.ParentFolderId.Id,
	}
	name := item.Source.From.EmailAddress.Name
	addr := item.Source.From.EmailAddress.Address
	if addr != "" || name != "" {
		msg.From = &EmailAddress{Name: name, Address: addr}
	} else if item.Source.SenderSMTPAddress != "" {
		msg.From = &EmailAddress{Address: item.Source.SenderSMTPAddress}
	}
	return msg
}

func buildSearchServiceBody(query string, maxResults int, filter map[string]interface{}) map[string]interface{} {
	if maxResults <= 0 {
		maxResults = 50
	}
	cvid := newUUID()
	logicalID := newUUID()
	body := map[string]interface{}{
		"Cvid":            cvid,
		"LogicalId":       logicalID,
		"Scenario":        map[string]interface{}{"Name": "owa.react"},
		"TextDecorations": "Off",
		"TimeZone":        "UTC",
		"QueryAlterationOptions": map[string]interface{}{
			"EnableAlteration": true,
			"EnableSuggestion": true,
			"SupportedRecourseDisplayTypes": []string{
				"Suggestion",
				"NoResultModification",
				"NoRequeryModification",
				"Modification",
			},
		},
		"EntityRequests": []map[string]interface{}{
			{
				"EntityType":       "Conversation",
				"ContentSources":   []string{"Exchange"},
				"EnableTopResults": true,
				"TopResultsCount":  minInt(7, maxResults),
				"From":             0,
				"Size":             maxResults,
				"Query":            map[string]interface{}{"QueryString": query},
				"Sort": []map[string]interface{}{
					{"Count": 7, "Field": "Score", "SortDirection": "Desc"},
					{"Field": "Time", "SortDirection": "Desc"},
				},
			},
		},
	}
	if filter == nil {
		filter = map[string]interface{}{
			"Or": []map[string]interface{}{
				{"Term": map[string]interface{}{"DistinguishedFolderName": "msgfolderroot"}},
				{"Term": map[string]interface{}{"DistinguishedFolderName": "DeletedItems"}},
			},
		}
	}
	body["EntityRequests"].([]map[string]interface{})[0]["Filter"] = filter
	return body
}

func buildSearchServiceFilter(folderID string) map[string]interface{} {
	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return nil
	}
	if name, ok := normalizeFolderName(folderID); ok {
		return map[string]interface{}{
			"Or": []map[string]interface{}{
				{"Term": map[string]interface{}{"DistinguishedFolderName": searchServiceFolderName(name)}},
			},
		}
	}
	return map[string]interface{}{
		"Or": []map[string]interface{}{
			{"Term": map[string]interface{}{"ParentFolderId": folderID}},
		},
	}
}

func searchServiceFolderName(distinguished string) string {
	switch strings.ToLower(distinguished) {
	case "inbox":
		return "Inbox"
	case "drafts":
		return "Drafts"
	case "sentitems":
		return "SentItems"
	case "deleteditems":
		return "DeletedItems"
	case "junkemail":
		return "JunkEmail"
	case "archive":
		return "Archive"
	case "outbox":
		return "Outbox"
	default:
		return distinguished
	}
}

func applySearchHeaders(headers map[string]string, tokens *Tokens, origin string) {
	if headers == nil || tokens == nil {
		return
	}
	if tokens.Bearer != "" {
		headers["Authorization"] = tokens.Bearer
	}
	if tokens.Canary != "" {
		headers["X-OWA-Canary"] = tokens.Canary
	}
	applySessionHeaders(headers)

	session := CurrentSessionHeaders()
	if session.OwaAppID != "" {
		headers["OwaAppId"] = session.OwaAppID
	}
	if session.ClientID != "" {
		headers["X-ClientId"] = session.ClientID
	}
	if session.ClientFlights != "" {
		headers["X-Client-Flights"] = session.ClientFlights
	}
	if session.RoutingKey != "" {
		headers["X-RoutingParameter-SessionKey"] = session.RoutingKey
	}
	if session.MSAppName != "" {
		headers["X-MS-AppName"] = session.MSAppName
	}
	if session.SearchGriffin != "" {
		headers["X-Search-Griffin-Version"] = session.SearchGriffin
	} else {
		headers["X-Search-Griffin-Version"] = "GWSv2"
	}

	headers["Client-Request-Id"] = newUUID()
	headers["Client-Session-Id"] = newUUID()
	headers["X-Client-LocalTime"] = time.Now().Format("2006-01-02T15:04:05.000-07:00")
	if origin != "" {
		headers["Origin"] = origin
	}
}

func searchServiceOrigin(page *rod.Page) string {
	if page == nil {
		return ""
	}
	info, err := page.Info()
	if err != nil || info == nil || info.URL == "" {
		return ""
	}
	parsed, err := url.Parse(info.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
		uint16(b[6])<<8|uint16(b[7]),
		uint16(b[8])<<8|uint16(b[9]),
		uint64(b[10])<<40|uint64(b[11])<<32|uint64(b[12])<<24|uint64(b[13])<<16|uint64(b[14])<<8|uint64(b[15]),
	)
}
