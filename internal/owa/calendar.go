package owa

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-rod/rod"
)

// CalendarViewResult represents a calendar list response.
type CalendarViewResult struct {
	TotalCount int             `json:"TotalCount,omitempty"`
	Events     []CalendarEvent `json:"Events,omitempty"`
}

// ListCalendarEvents lists calendar events in the given time range.
func ListCalendarEvents(page *rod.Page, tokens *Tokens, start string, end string, maxResults int, folderID string) (*CalendarViewResult, error) {
	if strings.TrimSpace(start) == "" || strings.TrimSpace(end) == "" {
		return nil, errors.New("start and end are required")
	}
	resolved, err := resolveCalendarFolderInput(page, tokens, folderID)
	if err != nil {
		return nil, err
	}
	folderID = resolved

	body, err := buildCalendarViewRequest(start, end, maxResults, folderID)
	if err != nil {
		return nil, err
	}

	resp, err := CallOWAAction(page, tokens, "FindItem", body)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("calendar list failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}

	events, total, err := extractCalendarEventsFromResponse(resp.Body)
	if err != nil {
		return nil, err
	}

	return &CalendarViewResult{
		TotalCount: total,
		Events:     events,
	}, nil
}

// GetCalendarEvent retrieves a single calendar event by ID.
func GetCalendarEvent(page *rod.Page, tokens *Tokens, eventID string) (*CalendarEvent, error) {
	reqBody, err := buildGetCalendarEventRequest(eventID)
	if err != nil {
		return nil, err
	}
	resp, err := CallOWAAction(page, tokens, "GetItem", reqBody)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("get event failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}
	events, _, err := extractCalendarEventsFromResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, errors.New("event not found")
	}
	return &events[0], nil
}

// CreateCalendarEvent creates a new calendar event.
func CreateCalendarEvent(page *rod.Page, tokens *Tokens, draft *CalendarEventDraft) (*CalendarEvent, error) {
	reqBody, err := buildCreateCalendarEventRequest(draft)
	if err != nil {
		return nil, err
	}
	resp, err := CallOWAAction(page, tokens, "CreateItem", reqBody)
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("create event failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}
	events, _, err := extractCalendarEventsFromResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, errors.New("event creation returned no items")
	}
	return &events[0], nil
}

// UpdateCalendarEvent updates an existing calendar event.
func UpdateCalendarEvent(page *rod.Page, tokens *Tokens, eventID string, update *CalendarEventUpdate) error {
	reqBody, err := buildUpdateCalendarEventRequest(eventID, update)
	if err != nil {
		return err
	}
	resp, err := CallOWAAction(page, tokens, "UpdateItem", reqBody)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return fmt.Errorf("update event failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}
	return nil
}

// DeleteCalendarEvent deletes a calendar event.
func DeleteCalendarEvent(page *rod.Page, tokens *Tokens, eventID string) error {
	if strings.TrimSpace(eventID) == "" {
		return errors.New("event ID required")
	}
	body := map[string]interface{}{
		"ItemIds": []map[string]interface{}{
			{"__type": "ItemId:#Exchange", "Id": eventID},
		},
		"DeleteType": "MoveToDeletedItems",
	}
	resp, err := CallOWAAction(page, tokens, "DeleteItem", body)
	if err != nil {
		return err
	}
	if resp.Status != 200 {
		return fmt.Errorf("delete event failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}
	return nil
}

func buildCalendarViewRequest(start string, end string, maxResults int, folderID string) (map[string]interface{}, error) {
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if start == "" || end == "" {
		return nil, errors.New("start and end required")
	}
	if maxResults <= 0 {
		maxResults = 50
	}

	body := map[string]interface{}{
		"__type": "FindItemRequest:#Exchange",
		"ItemShape": map[string]interface{}{
			"BaseShape": "IdOnly",
			"AdditionalProperties": []map[string]interface{}{
				{"FieldURI": "item:Subject"},
				{"FieldURI": "calendar:Start"},
				{"FieldURI": "calendar:End"},
				{"FieldURI": "calendar:IsAllDayEvent"},
				{"FieldURI": "calendar:Location"},
				{"FieldURI": "calendar:Organizer"},
				{"FieldURI": "calendar:RequiredAttendees"},
				{"FieldURI": "calendar:OptionalAttendees"},
				{"FieldURI": "calendar:LegacyFreeBusyStatus"},
				{"FieldURI": "calendar:IsCancelled"},
				{"FieldURI": "calendar:IsOrganizer"},
			},
		},
		"CalendarView": map[string]interface{}{
			"__type":             "CalendarView:#Exchange",
			"StartDate":          start,
			"EndDate":            end,
			"MaxEntriesReturned": maxResults,
		},
		"Traversal": "Shallow",
		"SortOrder": []map[string]interface{}{
			{
				"Order": "Ascending",
				"Path": map[string]interface{}{
					"__type":   "PropertyUri:#Exchange",
					"FieldURI": "calendar:Start",
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
			{"__type": "DistinguishedFolderId:#Exchange", "Id": "calendar"},
		}
	}

	return body, nil
}

func buildGetCalendarEventRequest(eventID string) (map[string]interface{}, error) {
	if strings.TrimSpace(eventID) == "" {
		return nil, errors.New("event ID required")
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
					{"__type": "PropertyUri:#Exchange", "FieldURI": "calendar:Start"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "calendar:End"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "calendar:IsAllDayEvent"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "calendar:Location"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "calendar:Organizer"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "calendar:RequiredAttendees"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "calendar:OptionalAttendees"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "calendar:LegacyFreeBusyStatus"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "calendar:IsCancelled"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "calendar:IsOrganizer"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "item:Importance"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "item:Sensitivity"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "item:Categories"},
				},
			},
			"ItemIds": []map[string]interface{}{
				{"__type": "ItemId:#Exchange", "Id": eventID},
			},
		},
	}
	return req, nil
}

func buildCreateCalendarEventRequest(draft *CalendarEventDraft) (map[string]interface{}, error) {
	if draft == nil {
		return nil, errors.New("event draft is required")
	}
	if strings.TrimSpace(draft.Subject) == "" {
		return nil, errors.New("event subject is required")
	}
	if strings.TrimSpace(draft.Start) == "" || strings.TrimSpace(draft.End) == "" {
		return nil, errors.New("event start and end are required")
	}

	item := map[string]interface{}{
		"__type":               "CalendarItem:#Exchange",
		"Subject":              draft.Subject,
		"Start":                draft.Start,
		"End":                  draft.End,
		"IsAllDayEvent":        draft.IsAllDayEvent,
		"LegacyFreeBusyStatus": "Busy",
	}
	if draft.Body != nil {
		bodyType := draft.Body.BodyType
		if bodyType == "" {
			bodyType = "HTML"
		}
		item["Body"] = map[string]interface{}{
			"BodyType": bodyType,
			"Value":    draft.Body.Value,
		}
	}
	if strings.TrimSpace(draft.Location) != "" {
		item["Location"] = map[string]interface{}{
			"DisplayName": draft.Location,
		}
	}
	if attendees := buildCalendarAttendees(draft.RequiredAttendees); len(attendees) > 0 {
		item["RequiredAttendees"] = attendees
	}
	if attendees := buildCalendarAttendees(draft.OptionalAttendees); len(attendees) > 0 {
		item["OptionalAttendees"] = attendees
	}

	sendInvites := "SendToNone"
	if len(draft.RequiredAttendees)+len(draft.OptionalAttendees) > 0 {
		sendInvites = "SendToAllAndSaveCopy"
	}

	req := map[string]interface{}{
		"__type": "CreateItemJsonRequest:#Exchange",
		"Header": buildJsonRequestHeader(),
		"Body": map[string]interface{}{
			"__type":                 "CreateItemRequest:#Exchange",
			"Items":                  []map[string]interface{}{item},
			"SendMeetingInvitations": sendInvites,
		},
	}
	return req, nil
}

func buildUpdateCalendarEventRequest(eventID string, update *CalendarEventUpdate) (map[string]interface{}, error) {
	if strings.TrimSpace(eventID) == "" {
		return nil, errors.New("event ID required")
	}
	if update == nil {
		return nil, errors.New("update payload required")
	}

	changes := []map[string]interface{}{}
	setField := func(field string, item map[string]interface{}) {
		changes = append(changes, map[string]interface{}{
			"__type": "SetItemField:#Exchange",
			"Path": map[string]interface{}{
				"__type":   "PropertyUri:#Exchange",
				"FieldURI": field,
			},
			"Item": item,
		})
	}

	if update.Subject != nil {
		setField("item:Subject", map[string]interface{}{
			"__type":  "CalendarItem:#Exchange",
			"Subject": *update.Subject,
		})
	}
	if update.Start != nil {
		setField("calendar:Start", map[string]interface{}{
			"__type": "CalendarItem:#Exchange",
			"Start":  *update.Start,
		})
	}
	if update.End != nil {
		setField("calendar:End", map[string]interface{}{
			"__type": "CalendarItem:#Exchange",
			"End":    *update.End,
		})
	}
	if update.IsAllDayEvent != nil {
		setField("calendar:IsAllDayEvent", map[string]interface{}{
			"__type":        "CalendarItem:#Exchange",
			"IsAllDayEvent": *update.IsAllDayEvent,
		})
	}
	if update.Location != nil {
		setField("calendar:Location", map[string]interface{}{
			"__type": "CalendarItem:#Exchange",
			"Location": map[string]interface{}{
				"DisplayName": *update.Location,
			},
		})
	}
	if update.Body != nil {
		bodyType := update.Body.BodyType
		if bodyType == "" {
			bodyType = "HTML"
		}
		setField("item:Body", map[string]interface{}{
			"__type": "CalendarItem:#Exchange",
			"Body": map[string]interface{}{
				"BodyType": bodyType,
				"Value":    update.Body.Value,
			},
		})
	}

	if len(changes) == 0 {
		return nil, errors.New("no updates provided")
	}

	body := map[string]interface{}{
		"__type":             "UpdateItemRequest:#Exchange",
		"MessageDisposition": "SaveOnly",
		"ConflictResolution": "AlwaysOverwrite",
		"ItemChanges": []map[string]interface{}{
			{
				"ItemId": map[string]interface{}{
					"__type": "ItemId:#Exchange",
					"Id":     eventID,
				},
				"Updates": changes,
			},
		},
		"SendMeetingInvitationsOrCancellations": "SendToNone",
	}

	return map[string]interface{}{
		"__type": "UpdateItemJsonRequest:#Exchange",
		"Header": buildJsonRequestHeader(),
		"Body":   body,
	}, nil
}

func buildCalendarAttendees(list []EmailAddress) []map[string]interface{} {
	if len(list) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, addr := range list {
		if strings.TrimSpace(addr.Address) == "" {
			continue
		}
		mailbox := map[string]interface{}{
			"EmailAddress": addr.Address,
		}
		if addr.Name != "" {
			mailbox["Name"] = addr.Name
		}
		if addr.RoutingType != "" {
			mailbox["RoutingType"] = addr.RoutingType
		}
		if addr.MailboxType != "" {
			mailbox["MailboxType"] = addr.MailboxType
		}
		out = append(out, map[string]interface{}{
			"Mailbox": mailbox,
		})
	}
	return out
}

func extractCalendarEventsFromResponse(body json.RawMessage) ([]CalendarEvent, int, error) {
	if len(body) == 0 {
		return nil, 0, errors.New("empty response")
	}
	var wrapper struct {
		Body struct {
			Items            []CalendarEvent `json:"Items"`
			TotalItemsInView int             `json:"TotalItemsInView"`
			ResponseMessages struct {
				Items []struct {
					RootFolder struct {
						Items            []CalendarEvent `json:"Items"`
						TotalItemsInView int             `json:"TotalItemsInView"`
					} `json:"RootFolder"`
					Items []CalendarEvent `json:"Items"`
				} `json:"Items"`
			} `json:"ResponseMessages"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil {
		if len(wrapper.Body.Items) > 0 {
			return wrapper.Body.Items, wrapper.Body.TotalItemsInView, nil
		}
		collected := []CalendarEvent{}
		total := 0
		for _, msg := range wrapper.Body.ResponseMessages.Items {
			if len(msg.Items) > 0 {
				collected = append(collected, msg.Items...)
				if total == 0 {
					total = len(msg.Items)
				}
			}
			if len(msg.RootFolder.Items) > 0 {
				collected = append(collected, msg.RootFolder.Items...)
				if total == 0 {
					total = msg.RootFolder.TotalItemsInView
				}
			}
		}
		if total == 0 && len(collected) > 0 {
			total = len(collected)
		}
		if len(collected) > 0 {
			return collected, total, nil
		}
	}

	var events []CalendarEvent
	if err := json.Unmarshal(body, &events); err == nil {
		return events, len(events), nil
	}

	return []CalendarEvent{}, 0, nil
}

func resolveCalendarFolderInput(page *rod.Page, tokens *Tokens, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}
	if name, ok := normalizeFolderName(input); ok {
		return resolveFolderID(page, tokens, name)
	}
	return input, nil
}
