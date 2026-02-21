package owa

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

// DirectoryCalendarInput describes how to resolve and add a directory calendar.
type DirectoryCalendarInput struct {
	Email          string
	Name           string
	DisplayName    string
	AllowAmbiguous bool
}

// DirectoryCalendarResult is the outcome of adding a directory calendar.
type DirectoryCalendarResult struct {
	Email             string `json:"email"`
	ResolvedName      string `json:"resolved_name,omitempty"`
	CalendarName      string `json:"calendar_name"`
	FolderID          string `json:"folder_id"`
	FolderChangeKey   string `json:"folder_change_key,omitempty"`
	CalendarID        string `json:"calendar_id"`
	CalendarChangeKey string `json:"calendar_change_key,omitempty"`
	CalendarGroupID   string `json:"calendar_group_id"`
	AlreadyExists     bool   `json:"already_exists,omitempty"`
}

type directoryPersonaCandidate struct {
	DisplayName string
	Email       string
	Relevance   float64
}

type directoryLookupResponse struct {
	Body struct {
		ResponseClass string `json:"ResponseClass"`
		ResponseCode  string `json:"ResponseCode"`
		ResultSet     []struct {
			DisplayName    string  `json:"DisplayName"`
			RelevanceScore float64 `json:"RelevanceScore"`
			EmailAddress   struct {
				EmailAddress string `json:"EmailAddress"`
			} `json:"EmailAddress"`
			EmailAddresses []struct {
				EmailAddress string `json:"EmailAddress"`
			} `json:"EmailAddresses"`
		} `json:"ResultSet"`
	} `json:"Body"`
}

type calendarGroupsResponse struct {
	CalendarFolders []struct {
		DisplayName string `json:"DisplayName"`
		FolderID    struct {
			ID string `json:"Id"`
		} `json:"FolderId"`
	} `json:"CalendarFolders"`
	CalendarGroups []struct {
		GroupID   string `json:"GroupId"`
		GroupName string `json:"GroupName"`
		GroupType int    `json:"GroupType"`
	} `json:"CalendarGroups"`
	ErrorCode     serviceErrorCode `json:"ErrorCode"`
	WasSuccessful bool             `json:"WasSuccessful"`
}

type serviceErrorCode string

func (c *serviceErrorCode) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*c = ""
		return nil
	}
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		*c = serviceErrorCode(strings.TrimSpace(asString))
		return nil
	}
	var asNumber json.Number
	if err := json.Unmarshal(data, &asNumber); err == nil {
		*c = serviceErrorCode(asNumber.String())
		return nil
	}
	var asFloat float64
	if err := json.Unmarshal(data, &asFloat); err == nil {
		*c = serviceErrorCode(strconv.FormatFloat(asFloat, 'f', -1, 64))
		return nil
	}
	return fmt.Errorf("unsupported error code json: %s", raw)
}

func (c serviceErrorCode) IsSuccess() bool {
	value := strings.TrimSpace(strings.ToLower(string(c)))
	return value == "" || value == "0" || value == "noerror"
}

type createCalendarGraphQLResponse struct {
	Data struct {
		CreateCalendar *struct {
			ID          string `json:"id"`
			ChangeKey   string `json:"changeKey"`
			Name        string `json:"name"`
			ParentGroup string `json:"parentGroupId"`
			FolderID    struct {
				ID        string `json:"Id"`
				ChangeKey string `json:"ChangeKey"`
			} `json:"FolderId"`
			CalendarID struct {
				ID        string `json:"id"`
				ChangeKey string `json:"changeKey"`
			} `json:"calendarId"`
		} `json:"createCalendar"`
	} `json:"data"`
	Errors []struct {
		Message    string `json:"message"`
		Extensions struct {
			InnerMessage string `json:"InnerMessage"`
		} `json:"extensions"`
	} `json:"errors"`
}

type createCalendarMutationResult struct {
	FolderID struct {
		ID        string
		ChangeKey string
	}
	CalendarID struct {
		ID        string
		ChangeKey string
	}
}

// CalendarFolder represents a calendar folder visible in the mailbox.
type CalendarFolder struct {
	DisplayName string `json:"display_name"`
	FolderID    string `json:"folder_id"`
}

const createCalendarMutation = "mutation CreateCalendar($newCalendarName: String!, $parentGroupServerId: String!, $emailAddress: String, $mailboxInfo: MailboxInfo!, $calendarTypename: String!) { createCalendar(input: { newCalendarName: $newCalendarName, parentGroupServerId: $parentGroupServerId, emailAddress: $emailAddress, mailboxInfo: $mailboxInfo, calendarTypename: $calendarTypename }) { FolderId { Id ChangeKey } calendarId { id changeKey mailboxInfo } } }"
const createCalendarMutationV2 = "mutation CreateCalendar($input: CreateCalendarInput!) { createCalendar(input: $input) { id changeKey name parentGroupId } }"
const syncTeammatesCalendarGroupMutation = "mutation SyncTeammatesCalendarGroup($request: SyncTeammatesCalendarGroupRequest!) { syncTeammatesCalendarGroup(request: $request) { __typename } }"
const createCalendarModuleKnownID = 999566

// ListCalendarFolders lists visible calendar folders from OWA.
func ListCalendarFolders(page *rod.Page, tokens *Tokens) ([]CalendarFolder, error) {
	resp, err := CallOWAActionWithOptions(page, tokens, "GetCalendarFolders", map[string]interface{}{}, OWAActionOptions{
		App:       "Calendar",
		ReqSource: "Calendar",
	})
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("get calendar folders failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}
	var payload calendarGroupsResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse calendar folders response: %w", err)
	}
	if !payload.ErrorCode.IsSuccess() {
		return nil, fmt.Errorf("get calendar folders returned error code %q", payload.ErrorCode)
	}

	folders := make([]CalendarFolder, 0, len(payload.CalendarFolders))
	for _, folder := range payload.CalendarFolders {
		id := strings.TrimSpace(folder.FolderID.ID)
		if id == "" {
			continue
		}
		folders = append(folders, CalendarFolder{
			DisplayName: strings.TrimSpace(folder.DisplayName),
			FolderID:    id,
		})
	}
	return folders, nil
}

// AddDirectoryCalendar adds a linked calendar from the directory by email or name.
func AddDirectoryCalendar(page *rod.Page, tokens *Tokens, input DirectoryCalendarInput) (*DirectoryCalendarResult, error) {
	if page == nil {
		return nil, errors.New("page is nil")
	}
	if tokens == nil {
		return nil, errors.New("tokens are nil")
	}

	email := strings.TrimSpace(input.Email)
	name := strings.TrimSpace(input.Name)
	if email == "" && name == "" {
		return nil, errors.New("email or name is required")
	}

	resolvedName := ""
	if email == "" {
		persona, err := resolveDirectoryPersona(page, tokens, name, input.AllowAmbiguous)
		if err != nil {
			return nil, err
		}
		email = persona.Email
		resolvedName = persona.DisplayName
	}

	parentGroupID, err := resolvePeoplesCalendarGroupID(page, tokens)
	if err != nil {
		return nil, err
	}
	beforeFolders, err := ListCalendarFolders(page, tokens)
	if err != nil {
		return nil, err
	}
	if resolvedName == "" && email != "" {
		if persona, lookupErr := resolveDirectoryPersona(page, tokens, email, true); lookupErr == nil && strings.EqualFold(strings.TrimSpace(persona.Email), email) {
			resolvedName = strings.TrimSpace(persona.DisplayName)
		}
	}

	calendarName := strings.TrimSpace(input.DisplayName)
	if calendarName == "" {
		if resolvedName != "" {
			calendarName = resolvedName
		} else {
			calendarName = email
		}
	}
	if existingFolderID, existingName, ok := findExistingDirectoryCalendarFolder(beforeFolders, calendarName, resolvedName, email); ok {
		return &DirectoryCalendarResult{
			Email:           email,
			ResolvedName:    resolvedName,
			CalendarName:    existingName,
			FolderID:        existingFolderID,
			CalendarID:      existingFolderID,
			CalendarGroupID: parentGroupID,
			AlreadyExists:   true,
		}, nil
	}

	mailbox := buildMailboxInfo(tokens)
	if mailbox == nil {
		hydrateMailboxIdentityFromPage(page, tokens)
		mailbox = buildMailboxInfo(tokens)
	}
	if mailbox == nil {
		mailbox = map[string]interface{}{
			"type":            "UserMailbox",
			"mailboxRank":     "Coprincipal",
			"mailboxProvider": "Office365",
		}
	}
	ensureCalendarSessionRouting(tokens, mailbox)

	created, err := createLinkedCalendar(page, tokens, calendarName, parentGroupID, email, mailbox)
	if err != nil {
		if !shouldTrySyncTeammatesCalendarFallback(err) {
			return nil, err
		}
		if syncErr := syncTeammatesCalendarGroup(page, tokens, email); syncErr != nil {
			return nil, syncErr
		}
		folderID, folderErr := waitForAddedDirectoryCalendarFolder(page, tokens, beforeFolders, calendarName, resolvedName, email)
		if folderErr != nil {
			return nil, folderErr
		}
		return &DirectoryCalendarResult{
			Email:           email,
			ResolvedName:    resolvedName,
			CalendarName:    calendarName,
			FolderID:        folderID,
			CalendarID:      folderID,
			CalendarGroupID: parentGroupID,
		}, nil
	}

	return &DirectoryCalendarResult{
		Email:             email,
		ResolvedName:      resolvedName,
		CalendarName:      calendarName,
		FolderID:          created.FolderID.ID,
		FolderChangeKey:   created.FolderID.ChangeKey,
		CalendarID:        created.CalendarID.ID,
		CalendarChangeKey: created.CalendarID.ChangeKey,
		CalendarGroupID:   parentGroupID,
	}, nil
}

func shouldRetryCreateCalendarWithAltType(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "calendartypename") ||
		strings.Contains(msg, "calendar typename") ||
		strings.Contains(msg, "unknown") && strings.Contains(msg, "typename")
}

func resolveDirectoryPersona(page *rod.Page, tokens *Tokens, query string, allowAmbiguous bool) (directoryPersonaCandidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return directoryPersonaCandidate{}, errors.New("name is required for directory lookup")
	}

	req := buildFindPeopleDirectoryRequest(query)
	resp, err := CallOWAAction(page, tokens, "FindPeople", req)
	if err != nil {
		return directoryPersonaCandidate{}, err
	}
	if resp.Status != 200 {
		return directoryPersonaCandidate{}, fmt.Errorf("directory lookup failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}

	var payload directoryLookupResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return directoryPersonaCandidate{}, fmt.Errorf("failed to parse directory lookup response: %w", err)
	}
	candidates := extractDirectoryPersonaCandidates(payload)
	return selectDirectoryPersona(query, candidates, allowAmbiguous)
}

func buildFindPeopleDirectoryRequest(query string) map[string]interface{} {
	query = strings.TrimSpace(query)
	return map[string]interface{}{
		"__type": "FindPeopleJsonRequest:#Exchange",
		"Header": map[string]interface{}{
			"__type":               "JsonRequestHeaders:#Exchange",
			"RequestServerVersion": "V2018_01_08",
			"TimeZoneContext": map[string]interface{}{
				"__type": "TimeZoneContext:#Exchange",
				"TimeZoneDefinition": map[string]interface{}{
					"__type": "TimeZoneDefinitionType:#Exchange",
					"Id":     "UTC",
				},
			},
		},
		"Body": map[string]interface{}{
			"__type": "FindPeopleRequest:#Exchange",
			"Context": []map[string]interface{}{
				{
					"__type": "ContextProperty:#Exchange",
					"Key":    "AppName",
					"Value":  "OWA",
				},
				{
					"__type": "ContextProperty:#Exchange",
					"Key":    "AppScenario",
					"Value":  "owa.react.recipientSearch",
				},
			},
			"ContextInfo": map[string]interface{}{
				"__type":       "FindPeopleContextInfo:#Exchange",
				"RecipientsTo": []interface{}{},
			},
			"IndexedPageItemView": map[string]interface{}{
				"__type":             "IndexedPageView:#Exchange",
				"BasePoint":          "Beginning",
				"MaxEntriesReturned": 25,
				"Offset":             0,
			},
			"PersonaShape": map[string]interface{}{
				"__type":    "PersonaResponseShape:#Exchange",
				"BaseShape": "IdOnly",
				"AdditionalProperties": []map[string]interface{}{
					{"__type": "PropertyUri:#Exchange", "FieldURI": "PersonaEmailAddress"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "EmailAddresses"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "PersonaDisplayName"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "PersonaDisplayNames"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "PersonaId"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "PersonaType"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "PersonaImAddress"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "PersonaTitle"},
				},
			},
			"QueryString":                     query,
			"QuerySources":                    []string{"Directory"},
			"SearchPeopleSuggestionIndex":     true,
			"ShouldResolveOneOffEmailAddress": false,
		},
	}
}

func extractDirectoryPersonaCandidates(payload directoryLookupResponse) []directoryPersonaCandidate {
	if !strings.EqualFold(payload.Body.ResponseClass, "Success") {
		return nil
	}
	candidates := make([]directoryPersonaCandidate, 0, len(payload.Body.ResultSet))
	seen := make(map[string]struct{}, len(payload.Body.ResultSet))

	for _, item := range payload.Body.ResultSet {
		email := strings.TrimSpace(item.EmailAddress.EmailAddress)
		if email == "" && len(item.EmailAddresses) > 0 {
			email = strings.TrimSpace(item.EmailAddresses[0].EmailAddress)
		}
		if email == "" {
			continue
		}
		key := strings.ToLower(email)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		candidates = append(candidates, directoryPersonaCandidate{
			DisplayName: strings.TrimSpace(item.DisplayName),
			Email:       email,
			Relevance:   item.RelevanceScore,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Relevance == candidates[j].Relevance {
			return strings.ToLower(candidates[i].Email) < strings.ToLower(candidates[j].Email)
		}
		return candidates[i].Relevance > candidates[j].Relevance
	})
	return candidates
}

func selectDirectoryPersona(query string, candidates []directoryPersonaCandidate, allowAmbiguous bool) (directoryPersonaCandidate, error) {
	if len(candidates) == 0 {
		return directoryPersonaCandidate{}, fmt.Errorf("no directory matches found for %q", query)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return directoryPersonaCandidate{}, errors.New("query is required")
	}

	if exact := filterDirectoryCandidates(candidates, func(c directoryPersonaCandidate) bool {
		return strings.EqualFold(c.Email, query) || strings.EqualFold(c.DisplayName, query)
	}); len(exact) == 1 {
		return exact[0], nil
	} else if len(exact) > 1 && !allowAmbiguous {
		return directoryPersonaCandidate{}, ambiguousDirectoryMatchError(query, exact)
	}

	if prefix := filterDirectoryCandidates(candidates, func(c directoryPersonaCandidate) bool {
		return strings.HasPrefix(strings.ToLower(c.DisplayName), q) ||
			strings.HasPrefix(strings.ToLower(c.Email), q)
	}); len(prefix) == 1 {
		return prefix[0], nil
	} else if len(prefix) > 1 && !allowAmbiguous {
		return directoryPersonaCandidate{}, ambiguousDirectoryMatchError(query, prefix)
	}

	if allowAmbiguous {
		return candidates[0], nil
	}
	return directoryPersonaCandidate{}, ambiguousDirectoryMatchError(query, candidates)
}

func filterDirectoryCandidates(candidates []directoryPersonaCandidate, keep func(directoryPersonaCandidate) bool) []directoryPersonaCandidate {
	out := make([]directoryPersonaCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if keep(candidate) {
			out = append(out, candidate)
		}
	}
	return out
}

func ambiguousDirectoryMatchError(query string, candidates []directoryPersonaCandidate) error {
	max := len(candidates)
	if max > 5 {
		max = 5
	}
	parts := make([]string, 0, max)
	for _, candidate := range candidates[:max] {
		if candidate.DisplayName != "" {
			parts = append(parts, fmt.Sprintf("%s <%s>", candidate.DisplayName, candidate.Email))
		} else {
			parts = append(parts, candidate.Email)
		}
	}
	return fmt.Errorf("multiple directory matches for %q: %s (pass --allow-ambiguous or use --email)", query, strings.Join(parts, ", "))
}

func resolvePeoplesCalendarGroupID(page *rod.Page, tokens *Tokens) (string, error) {
	payload, err := getCalendarGroups(page, tokens)
	if err != nil {
		return "", err
	}
	if len(payload.CalendarGroups) == 0 {
		return "", errors.New("calendar groups not found in response")
	}

	groupID := selectPeoplesCalendarGroupID(payload.CalendarGroups)
	if groupID == "" {
		return "", errors.New("no usable calendar group id returned")
	}
	return groupID, nil
}

func getCalendarGroups(page *rod.Page, tokens *Tokens) (*calendarGroupsResponse, error) {
	resp, err := CallOWAActionWithOptions(page, tokens, "GetCalendarFolders", map[string]interface{}{}, OWAActionOptions{
		App:       "Calendar",
		ReqSource: "Calendar",
	})
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("get calendar folders failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}
	var payload calendarGroupsResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse calendar groups response: %w", err)
	}
	if !payload.ErrorCode.IsSuccess() {
		return nil, fmt.Errorf("get calendar folders returned error code %q", payload.ErrorCode)
	}
	return &payload, nil
}

func createLinkedCalendar(
	page *rod.Page,
	tokens *Tokens,
	calendarName string,
	parentGroupID string,
	email string,
	mailbox map[string]interface{},
) (*createCalendarMutationResult, error) {
	created, moduleErr := createLinkedCalendarViaOWAModule(page, calendarName, parentGroupID, email, mailbox, "LinkedCalendarEntry")
	if moduleErr == nil {
		return created, nil
	}
	if shouldRetryCreateCalendarWithAltType(moduleErr) {
		created, retryErr := createLinkedCalendarViaOWAModule(page, calendarName, parentGroupID, email, mailbox, "LinkedCalendarEntryV2")
		if retryErr == nil {
			return created, nil
		}
		moduleErr = retryErr
	}

	created, graphErr := createLinkedCalendarViaGraphQL(page, tokens, calendarName, parentGroupID, email, mailbox)
	if graphErr == nil {
		return created, nil
	}
	if moduleErr != nil {
		return nil, fmt.Errorf("create calendar via owa module failed: %v; graphql fallback failed: %w", moduleErr, graphErr)
	}
	return nil, graphErr
}

func createLinkedCalendarViaGraphQL(
	page *rod.Page,
	tokens *Tokens,
	calendarName string,
	parentGroupID string,
	email string,
	mailbox map[string]interface{},
) (*createCalendarMutationResult, error) {
	v2Payload := buildCreateCalendarV2Payload(calendarName, parentGroupID, buildDirectoryLinkedMailboxInfo(email, mailbox))
	resp, err := callCalendarGraphQL(page, tokens, v2Payload)
	if err != nil {
		return nil, err
	}
	created, parseErr := parseCreateCalendarMutationResult(resp.Body)
	if parseErr == nil {
		return created, nil
	}
	if !shouldFallbackToLegacyCreateCalendar(parseErr) {
		return nil, parseErr
	}

	legacyCreated, legacyErr := createLinkedCalendarLegacy(page, tokens, calendarName, parentGroupID, email, mailbox)
	if legacyErr != nil {
		return nil, legacyErr
	}
	return legacyCreated, nil
}

func createLinkedCalendarViaOWAModule(
	page *rod.Page,
	calendarName string,
	parentGroupID string,
	email string,
	mailbox map[string]interface{},
	calendarType string,
) (*createCalendarMutationResult, error) {
	if page == nil {
		return nil, errors.New("page is nil")
	}
	if strings.TrimSpace(calendarType) == "" {
		calendarType = "LinkedCalendarEntry"
	}
	variables := map[string]interface{}{
		"newCalendarName":     calendarName,
		"parentGroupServerId": parentGroupID,
		"emailAddress":        email,
		"mailboxInfo":         mailbox,
		"calendarTypename":    calendarType,
	}
	result, err := runOWACreateCalendarModule(page, variables)
	if err != nil && shouldWarmCreateCalendarModule(err) {
		warmCreateCalendarModule(page)
		result, err = runOWACreateCalendarModule(page, variables)
	}
	if err != nil {
		return nil, err
	}
	return parseCreateCalendarMutationResult(result)
}

func shouldWarmCreateCalendarModule(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "runtime unavailable") ||
		strings.Contains(msg, "module not found") ||
		strings.Contains(msg, "export missing")
}

func warmCreateCalendarModule(page *rod.Page) {
	if page == nil {
		return
	}
	if hasCreateCalendarModule(page) {
		return
	}
	if url := calendarViewURL(page); url != "" {
		_ = page.Navigate(url)
		_ = page.WaitLoad()
	}
	clickXPath := func(selectors []string) {
		_, _ = page.Eval(`(selectors) => {
			for (const selector of selectors) {
				try {
					const node = document.evaluate(
						selector,
						document,
						null,
						XPathResult.FIRST_ORDERED_NODE_TYPE,
						null
					).singleNodeValue;
					if (node && typeof node.click === "function") {
						node.click();
						return selector;
					}
				} catch {}
			}
			return "";
		}`, selectors)
	}

	clickXPath([]string{
		"//button[contains(., 'Add calendar')]",
		"//div[@role='button' and contains(., 'Add calendar')]",
		"//span[contains(., 'Add calendar')]",
	})
	time.Sleep(1200 * time.Millisecond)
	if hasCreateCalendarModule(page) {
		return
	}
	clickXPath([]string{
		"//button[contains(., 'Add from directory')]",
		"//div[@role='menuitem' and contains(., 'Add from directory')]",
		"//span[contains(., 'Add from directory')]",
		"//button[contains(., 'From directory')]",
		"//span[contains(., 'From directory')]",
	})
	time.Sleep(1800 * time.Millisecond)
}

func hasCreateCalendarModule(page *rod.Page) bool {
	if page == nil {
		return false
	}
	result, err := page.Eval(`() => {
		if (!Array.isArray(window.webpackChunkOwa)) {
			return false;
		}
		if (typeof window.__owaReq !== "function") {
			const marker = Math.floor(Date.now() % 1000000000) + 987654321;
			window.webpackChunkOwa.push([[marker], {}, (req) => { window.__owaReq = req; }]);
		}
		const req = window.__owaReq;
		if (!req || !req.m) {
			return false;
		}
		if (req.m[999566]) {
			return true;
		}
		for (const id of Object.keys(req.m)) {
			let src = "";
			try {
				src = String(req.m[id]);
			} catch {}
			if (
				src.includes("mutation CreateCalendar(") &&
				src.includes("newCalendarName") &&
				src.includes("parentGroupServerId") &&
				src.includes("mailboxInfo") &&
				src.includes("createCalendar(input")
			) {
				return true;
			}
		}
		return false;
	}`)
	if err != nil {
		return false
	}
	var ok bool
	if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &ok); err != nil {
		return false
	}
	return ok
}

func calendarViewURL(page *rod.Page) string {
	origin := searchServiceOrigin(page)
	if origin == "" {
		origin = strings.TrimSuffix(OWABaseURL, "/mail/")
	}
	origin = strings.TrimRight(origin, "/")
	if origin == "" {
		return ""
	}
	return origin + "/calendar/view/month"
}

func createLinkedCalendarLegacy(
	page *rod.Page,
	tokens *Tokens,
	calendarName string,
	parentGroupID string,
	email string,
	mailbox map[string]interface{},
) (*createCalendarMutationResult, error) {
	payload := map[string]interface{}{
		"operationName": "CreateCalendar",
		"variables": map[string]interface{}{
			"newCalendarName":     calendarName,
			"parentGroupServerId": parentGroupID,
			"emailAddress":        email,
			"mailboxInfo":         mailbox,
			"calendarTypename":    "LinkedCalendarEntry",
		},
		"query": createCalendarMutation,
	}
	resp, err := callCalendarGraphQL(page, tokens, payload)
	if err != nil {
		return nil, err
	}
	created, parseErr := parseCreateCalendarMutationResult(resp.Body)
	if parseErr == nil {
		return created, nil
	}
	if shouldRetryCreateCalendarWithAltType(parseErr) {
		payload = map[string]interface{}{
			"operationName": "CreateCalendar",
			"variables": map[string]interface{}{
				"newCalendarName":     calendarName,
				"parentGroupServerId": parentGroupID,
				"emailAddress":        email,
				"mailboxInfo":         mailbox,
				"calendarTypename":    "LinkedCalendarEntryV2",
			},
			"query": createCalendarMutation,
		}
		resp, err = callCalendarGraphQL(page, tokens, payload)
		if err != nil {
			return nil, err
		}
		return parseCreateCalendarMutationResult(resp.Body)
	}
	return nil, parseErr
}

func buildCreateCalendarV2Payload(calendarName string, parentGroupID string, mailbox map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"operationName": "CreateCalendar",
		"variables": map[string]interface{}{
			"input": map[string]interface{}{
				"calendarName":        calendarName,
				"parentGroupServerId": parentGroupID,
				"mailboxInfo":         mailbox,
			},
		},
		"query": createCalendarMutationV2,
	}
}

func buildDirectoryLinkedMailboxInfo(email string, fallback map[string]interface{}) map[string]interface{} {
	linkedEmail := strings.TrimSpace(email)
	if linkedEmail == "" {
		return fallback
	}
	mailbox := map[string]interface{}{
		"type":               "UserMailbox",
		"userIdentity":       linkedEmail,
		"mailboxSmtpAddress": linkedEmail,
	}
	if fallback != nil {
		if typ, ok := fallback["type"].(string); ok && strings.TrimSpace(typ) != "" {
			mailbox["type"] = strings.TrimSpace(typ)
		}
		if rank, ok := fallback["mailboxRank"].(string); ok && strings.TrimSpace(rank) != "" {
			mailbox["mailboxRank"] = strings.TrimSpace(rank)
		}
	}
	return mailbox
}

func runOWACreateCalendarModule(page *rod.Page, variables map[string]interface{}) ([]byte, error) {
	result, err := page.Eval(`async (variables, knownModuleID) => {
		const ensureReq = () => {
			if (typeof window.__owaReq === "function") {
				return window.__owaReq;
			}
			if (!Array.isArray(window.webpackChunkOwa)) {
				return null;
			}
			const marker = Math.floor(Date.now() % 1000000000) + 987654321;
			window.webpackChunkOwa.push([[marker], {}, (req) => { window.__owaReq = req; }]);
			return typeof window.__owaReq === "function" ? window.__owaReq : null;
		};
		const req = ensureReq();
		if (!req) {
			return { ok: false, error: "owa webpack runtime unavailable" };
		}

		const findCandidates = () => {
			const out = [];
			const add = (id) => {
				const n = Number(id);
				if (!Number.isFinite(n)) {
					return;
				}
				if (out.indexOf(n) < 0) {
					out.push(n);
				}
			};
			if (knownModuleID && req.m && (req.m[knownModuleID] || req.m[String(knownModuleID)])) {
				add(knownModuleID);
			}
			if (!req.m) {
				return out;
			}
			for (const id of Object.keys(req.m)) {
				let src = "";
				try {
					src = String(req.m[id]);
				} catch {}
				if (
					src.includes("mutation CreateCalendar(") &&
					src.includes("newCalendarName") &&
					src.includes("parentGroupServerId") &&
					src.includes("mailboxInfo") &&
					src.includes("createCalendar(input")
				) {
					add(id);
				}
			}
			return out;
		};

		let candidates = findCandidates();
		if (candidates.length === 0 && typeof req.e === "function") {
			for (const chunkID of [37462, knownModuleID]) {
				if (!chunkID) {
					continue;
				}
				try {
					await req.e(chunkID);
				} catch {}
			}
			candidates = findCandidates();
		}
		if (candidates.length === 0) {
			return { ok: false, error: "create calendar module not found" };
		}

		let lastError = "";
		for (const moduleID of candidates) {
			let mod = null;
			try {
				mod = req(moduleID);
			} catch (err) {
				lastError = String(err && err.message ? err.message : err);
				continue;
			}
			if (!mod || typeof mod !== "object" || typeof mod.y !== "function") {
				lastError = "create calendar export missing";
				continue;
			}
			try {
				const payload = await mod.y(variables);
				return { ok: true, moduleID, result: payload };
			} catch (err) {
				lastError = String(err && err.message ? err.message : err);
			}
		}
		return { ok: false, error: lastError || "create calendar invocation failed" };
	}`, variables, createCalendarModuleKnownID)
	if err != nil {
		return nil, fmt.Errorf("owa module create calendar eval failed: %w", err)
	}

	var parsed struct {
		OK      bool            `json:"ok"`
		Error   string          `json:"error"`
		Result  json.RawMessage `json:"result"`
		Module  int             `json:"moduleID"`
		ModName string          `json:"moduleName"`
	}
	if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse owa module create calendar response: %w", err)
	}
	if !parsed.OK {
		msg := strings.TrimSpace(parsed.Error)
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("owa module create calendar failed: %s", msg)
	}
	if len(parsed.Result) == 0 || string(parsed.Result) == "null" {
		return nil, errors.New("owa module create calendar returned empty result")
	}
	return []byte(parsed.Result), nil
}

func parseCreateCalendarMutationResult(body []byte) (*createCalendarMutationResult, error) {
	var parsed createCalendarGraphQLResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse createCalendar response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		msg := strings.TrimSpace(parsed.Errors[0].Message)
		inner := strings.TrimSpace(parsed.Errors[0].Extensions.InnerMessage)
		switch {
		case msg != "" && inner != "":
			return nil, fmt.Errorf("create calendar failed: %s (%s)", msg, inner)
		case msg != "":
			return nil, fmt.Errorf("create calendar failed: %s", msg)
		case inner != "":
			return nil, fmt.Errorf("create calendar failed: %s", inner)
		default:
			return nil, errors.New("create calendar failed: unknown graphql error")
		}
	}
	if parsed.Data.CreateCalendar == nil {
		var direct struct {
			ID        string `json:"id"`
			ChangeKey string `json:"changeKey"`
			FolderID  struct {
				ID        string `json:"Id"`
				ChangeKey string `json:"ChangeKey"`
			} `json:"FolderId"`
			CalendarID struct {
				ID        string `json:"id"`
				ChangeKey string `json:"changeKey"`
			} `json:"calendarId"`
		}
		if err := json.Unmarshal(body, &direct); err == nil {
			result := &createCalendarMutationResult{}
			if id := strings.TrimSpace(direct.ID); id != "" {
				result.FolderID.ID = id
				result.CalendarID.ID = id
				result.FolderID.ChangeKey = strings.TrimSpace(direct.ChangeKey)
				result.CalendarID.ChangeKey = strings.TrimSpace(direct.ChangeKey)
				return result, nil
			}
			result.FolderID.ID = strings.TrimSpace(direct.FolderID.ID)
			result.FolderID.ChangeKey = strings.TrimSpace(direct.FolderID.ChangeKey)
			result.CalendarID.ID = strings.TrimSpace(direct.CalendarID.ID)
			result.CalendarID.ChangeKey = strings.TrimSpace(direct.CalendarID.ChangeKey)
			if normalizeCreateCalendarMutationResult(result) {
				return result, nil
			}
		}
		return nil, errors.New("create calendar failed: empty mutation result")
	}

	result := &createCalendarMutationResult{}
	if id := strings.TrimSpace(parsed.Data.CreateCalendar.ID); id != "" {
		result.FolderID.ID = id
		result.CalendarID.ID = id
		result.FolderID.ChangeKey = strings.TrimSpace(parsed.Data.CreateCalendar.ChangeKey)
		result.CalendarID.ChangeKey = strings.TrimSpace(parsed.Data.CreateCalendar.ChangeKey)
		return result, nil
	}

	result.FolderID.ID = strings.TrimSpace(parsed.Data.CreateCalendar.FolderID.ID)
	result.FolderID.ChangeKey = strings.TrimSpace(parsed.Data.CreateCalendar.FolderID.ChangeKey)
	result.CalendarID.ID = strings.TrimSpace(parsed.Data.CreateCalendar.CalendarID.ID)
	result.CalendarID.ChangeKey = strings.TrimSpace(parsed.Data.CreateCalendar.CalendarID.ChangeKey)
	if !normalizeCreateCalendarMutationResult(result) {
		return nil, errors.New("create calendar failed: missing calendar id")
	}
	return result, nil
}

func normalizeCreateCalendarMutationResult(result *createCalendarMutationResult) bool {
	if result == nil {
		return false
	}
	if strings.TrimSpace(result.FolderID.ID) == "" && strings.TrimSpace(result.CalendarID.ID) != "" {
		result.FolderID.ID = strings.TrimSpace(result.CalendarID.ID)
		if strings.TrimSpace(result.FolderID.ChangeKey) == "" {
			result.FolderID.ChangeKey = strings.TrimSpace(result.CalendarID.ChangeKey)
		}
	}
	if strings.TrimSpace(result.CalendarID.ID) == "" && strings.TrimSpace(result.FolderID.ID) != "" {
		result.CalendarID.ID = strings.TrimSpace(result.FolderID.ID)
		if strings.TrimSpace(result.CalendarID.ChangeKey) == "" {
			result.CalendarID.ChangeKey = strings.TrimSpace(result.FolderID.ChangeKey)
		}
	}
	return strings.TrimSpace(result.FolderID.ID) != "" && strings.TrimSpace(result.CalendarID.ID) != ""
}

func shouldFallbackToLegacyCreateCalendar(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "createcalendarinput") ||
		strings.Contains(msg, "unknown argument") && strings.Contains(msg, "input") ||
		strings.Contains(msg, "cannot query field 'id'") ||
		strings.Contains(msg, "cannot query field \"id\"")
}

func shouldTrySyncTeammatesCalendarFallback(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return (strings.Contains(msg, "createcalendar") || strings.Contains(msg, "create calendar")) &&
		(strings.Contains(msg, "found null instead: createcalendar") ||
			strings.Contains(msg, "found null instead: create calendar") ||
			strings.Contains(msg, "empty mutation result"))
}

func syncTeammatesCalendarGroup(page *rod.Page, tokens *Tokens, email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("directory email is required")
	}
	payload := map[string]interface{}{
		"operationName": "SyncTeammatesCalendarGroup",
		"variables": map[string]interface{}{
			"request": map[string]interface{}{
				"mailboxId": email,
			},
		},
		"query": syncTeammatesCalendarGroupMutation,
	}
	resp, err := callCalendarGraphQL(page, tokens, payload)
	if err != nil {
		return err
	}
	var parsed struct {
		Data struct {
			SyncTeammatesCalendarGroup interface{} `json:"syncTeammatesCalendarGroup"`
		} `json:"data"`
		Errors []struct {
			Message    string `json:"message"`
			Extensions struct {
				InnerMessage string `json:"InnerMessage"`
			} `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(resp.Body, &parsed); err != nil {
		return fmt.Errorf("failed to parse syncTeammatesCalendarGroup response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		msg := strings.TrimSpace(parsed.Errors[0].Message)
		inner := strings.TrimSpace(parsed.Errors[0].Extensions.InnerMessage)
		switch {
		case msg != "" && inner != "":
			return fmt.Errorf("sync teammates calendar group failed: %s (%s)", msg, inner)
		case msg != "":
			return fmt.Errorf("sync teammates calendar group failed: %s", msg)
		case inner != "":
			return fmt.Errorf("sync teammates calendar group failed: %s", inner)
		default:
			return errors.New("sync teammates calendar group failed")
		}
	}
	return nil
}

func waitForAddedDirectoryCalendarFolder(
	page *rod.Page,
	tokens *Tokens,
	before []CalendarFolder,
	calendarName string,
	resolvedName string,
	email string,
) (string, error) {
	const attempts = 6
	const interval = 1 * time.Second
	beforeByID := make(map[string]struct{}, len(before))
	for _, folder := range before {
		id := strings.TrimSpace(folder.FolderID)
		if id != "" {
			beforeByID[id] = struct{}{}
		}
	}

	for i := 0; i < attempts; i++ {
		after, err := ListCalendarFolders(page, tokens)
		if err != nil {
			return "", err
		}
		if folderID, ok := findAddedDirectoryCalendarFolderID(beforeByID, after, calendarName, resolvedName, email); ok {
			return folderID, nil
		}
		time.Sleep(interval)
	}

	return "", errors.New("directory calendar sync completed but calendar folder was not found")
}

func findAddedDirectoryCalendarFolderID(
	beforeByID map[string]struct{},
	after []CalendarFolder,
	calendarName string,
	resolvedName string,
	email string,
) (string, bool) {
	normalize := func(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
	wantNames := map[string]struct{}{}
	for _, candidate := range []string{calendarName, resolvedName, email} {
		normalized := normalize(candidate)
		if normalized != "" {
			wantNames[normalized] = struct{}{}
		}
	}

	newIDs := make([]CalendarFolder, 0, len(after))
	for _, folder := range after {
		id := strings.TrimSpace(folder.FolderID)
		if id == "" {
			continue
		}
		if _, exists := beforeByID[id]; exists {
			continue
		}
		newIDs = append(newIDs, folder)
	}
	if len(newIDs) == 1 {
		return strings.TrimSpace(newIDs[0].FolderID), true
	}
	for _, folder := range newIDs {
		if _, ok := wantNames[normalize(folder.DisplayName)]; ok {
			return strings.TrimSpace(folder.FolderID), true
		}
	}
	return "", false
}

func findExistingDirectoryCalendarFolder(folders []CalendarFolder, identities ...string) (string, string, bool) {
	wants := make(map[string]struct{}, len(identities))
	for _, identity := range identities {
		normalized := strings.ToLower(strings.TrimSpace(identity))
		if normalized != "" {
			wants[normalized] = struct{}{}
		}
	}
	if len(wants) == 0 {
		return "", "", false
	}

	for _, folder := range folders {
		folderID := strings.TrimSpace(folder.FolderID)
		if folderID == "" {
			continue
		}
		name := strings.TrimSpace(folder.DisplayName)
		if name == "" {
			continue
		}
		if _, ok := wants[strings.ToLower(name)]; ok {
			return folderID, name, true
		}
	}
	return "", "", false
}

func callCalendarGraphQL(page *rod.Page, tokens *Tokens, body map[string]interface{}) (*FetchResponse, error) {
	if page == nil {
		return nil, errors.New("page is nil")
	}
	if tokens == nil {
		return nil, errors.New("tokens are nil")
	}
	endpoint := calendarGraphQLEndpoint(page)
	if endpoint == "" {
		return nil, errors.New("calendar graphql endpoint unavailable")
	}

	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
		"X-Req-Source": "Calendar",
	}
	if tokens.Bearer != "" {
		headers["Authorization"] = tokens.Bearer
	}
	if tokens.Canary != "" {
		headers["X-OWA-CANARY"] = tokens.Canary
	}
	if !tokens.Session.IsZero() {
		SetSessionHeaders(tokens.Session)
	}
	applySessionHeaders(headers)
	applyPreferHeader(headers)
	if origin := searchServiceOrigin(page); origin != "" {
		headers["Origin"] = origin
	}

	resp, err := fetchFn(page, FetchRequest{
		URL:     endpoint,
		Method:  "POST",
		Headers: headers,
		Body:    body,
	})
	if err != nil {
		return nil, err
	}
	if resp.Status != 200 && len(resp.Body) == 0 {
		return nil, fmt.Errorf("calendar graphql failed with status %d: %s", resp.Status, formatOWAErrorDetails(resp))
	}
	return resp, nil
}

func calendarGraphQLEndpoint(page *rod.Page) string {
	origin := searchServiceOrigin(page)
	if origin == "" {
		return ""
	}
	return origin + "/outlookgatewayb2/graphql"
}

func selectPeoplesCalendarGroupID(groups []struct {
	GroupID   string `json:"GroupId"`
	GroupName string `json:"GroupName"`
	GroupType int    `json:"GroupType"`
}) string {
	for _, group := range groups {
		if group.GroupType == 2 && strings.TrimSpace(group.GroupID) != "" {
			return strings.TrimSpace(group.GroupID)
		}
	}
	for _, group := range groups {
		if strings.TrimSpace(group.GroupID) != "" {
			return strings.TrimSpace(group.GroupID)
		}
	}
	return ""
}

func hydrateMailboxIdentityFromPage(page *rod.Page, tokens *Tokens) {
	if page == nil || tokens == nil {
		return
	}
	updated := false
	if email, err := getUserEmailFromPage(page); err == nil {
		email = strings.TrimSpace(email)
		if strings.Contains(email, "@") && !strings.EqualFold(email, tokens.UserEmail) {
			tokens.UserEmail = email
			updated = true
		}
	}
	if session, err := getSessionHeadersFromPage(page); err == nil && !session.IsZero() {
		merged := MergeSessionHeaders(tokens.Session, session)
		if merged != tokens.Session {
			tokens.Session = merged
			updated = true
		}
	}
	if bearerTokens, err := getBearerTokensFromStorage(page); err == nil {
		if token := strings.TrimSpace(bearerTokens.OWA); token != "" && token != tokens.Bearer {
			tokens.Bearer = token
			updated = true
		}
		if token := strings.TrimSpace(bearerTokens.Graph); token != "" && token != tokens.GraphBearer {
			tokens.GraphBearer = token
			updated = true
		}
		if token := strings.TrimSpace(bearerTokens.Substrate); token != "" && token != tokens.Substrate {
			tokens.Substrate = token
			updated = true
		}
	}
	if tokens.Session.AnchorMailbox == "" && strings.Contains(tokens.UserEmail, "@") {
		tokens.Session.AnchorMailbox = tokens.UserEmail
		updated = true
	}
	if updated {
		_ = SaveTokens(tokens)
	}
}

func ensureCalendarSessionRouting(tokens *Tokens, mailbox map[string]interface{}) {
	if tokens == nil || mailbox == nil {
		return
	}
	if strings.TrimSpace(tokens.Session.AnchorMailbox) == "" {
		if smtp, ok := mailbox["mailboxSmtpAddress"].(string); ok {
			smtp = strings.TrimSpace(smtp)
			if strings.Contains(smtp, "@") {
				tokens.Session.AnchorMailbox = smtp
			}
		}
	}
	if strings.TrimSpace(tokens.Session.TenantID) == "" {
		if tenantID := tenantIDFromBearerTokens(tokens); tenantID != "" {
			tokens.Session.TenantID = tenantID
		}
	}
	if !tokens.Session.IsZero() {
		SetSessionHeaders(tokens.Session)
	}
}

func tenantIDFromBearerTokens(tokens *Tokens) string {
	if tokens == nil {
		return ""
	}
	for _, raw := range []string{tokens.Bearer, tokens.GraphBearer, tokens.Substrate} {
		if tenantID := tenantIDFromBearerToken(raw); tenantID != "" {
			return tenantID
		}
	}
	return ""
}

func tenantIDFromBearerToken(raw string) string {
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
	payload := strings.TrimSpace(parts[1])
	if payload == "" {
		return ""
	}
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			return ""
		}
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(data, &claims); err != nil {
		return ""
	}
	for _, key := range []string{"tid", "tenantid", "tenant_id"} {
		if value, ok := claims[key].(string); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}
