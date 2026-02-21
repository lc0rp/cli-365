package owa

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestBuildFindPeopleDirectoryRequest(t *testing.T) {
	req := buildFindPeopleDirectoryRequest("alice")

	if req["__type"] != "FindPeopleJsonRequest:#Exchange" {
		t.Fatalf("__type = %v", req["__type"])
	}
	body, ok := req["Body"].(map[string]interface{})
	if !ok {
		t.Fatal("Body missing")
	}
	if body["__type"] != "FindPeopleRequest:#Exchange" {
		t.Fatalf("Body.__type = %v", body["__type"])
	}
	if body["QueryString"] != "alice" {
		t.Fatalf("QueryString = %v, want alice", body["QueryString"])
	}
	sources, ok := body["QuerySources"].([]string)
	if !ok || len(sources) != 1 || sources[0] != "Directory" {
		t.Fatalf("QuerySources = %#v, want [Directory]", body["QuerySources"])
	}
}

func TestExtractDirectoryPersonaCandidates(t *testing.T) {
	payload := directoryLookupResponse{}
	payload.Body.ResponseClass = "Success"
	payload.Body.ResultSet = []struct {
		DisplayName    string  `json:"DisplayName"`
		RelevanceScore float64 `json:"RelevanceScore"`
		EmailAddress   struct {
			EmailAddress string `json:"EmailAddress"`
		} `json:"EmailAddress"`
		EmailAddresses []struct {
			EmailAddress string `json:"EmailAddress"`
		} `json:"EmailAddresses"`
	}{
		{
			DisplayName:    "Alice A",
			RelevanceScore: 5,
			EmailAddress: struct {
				EmailAddress string `json:"EmailAddress"`
			}{EmailAddress: "alice@example.com"},
		},
		{
			DisplayName:    "Alice Dup",
			RelevanceScore: 1,
			EmailAddress: struct {
				EmailAddress string `json:"EmailAddress"`
			}{EmailAddress: "alice@example.com"},
		},
		{
			DisplayName:    "Bob B",
			RelevanceScore: 10,
			EmailAddress: struct {
				EmailAddress string `json:"EmailAddress"`
			}{EmailAddress: "bob@example.com"},
		},
	}

	candidates := extractDirectoryPersonaCandidates(payload)
	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}
	if candidates[0].Email != "bob@example.com" {
		t.Fatalf("candidates[0].Email = %q, want bob@example.com", candidates[0].Email)
	}
	if candidates[1].Email != "alice@example.com" {
		t.Fatalf("candidates[1].Email = %q, want alice@example.com", candidates[1].Email)
	}
}

func TestSelectDirectoryPersona(t *testing.T) {
	candidates := []directoryPersonaCandidate{
		{DisplayName: "Alice Adams", Email: "alice.adams@example.com", Relevance: 9},
		{DisplayName: "Alice Anders", Email: "alice.anders@example.com", Relevance: 8},
		{DisplayName: "Bob Brown", Email: "bob@example.com", Relevance: 7},
	}

	got, err := selectDirectoryPersona("bob@example.com", candidates, false)
	if err != nil {
		t.Fatalf("selectDirectoryPersona email exact error: %v", err)
	}
	if got.Email != "bob@example.com" {
		t.Fatalf("exact match email = %q, want bob@example.com", got.Email)
	}

	_, err = selectDirectoryPersona("Alice", candidates, false)
	if err == nil {
		t.Fatal("expected ambiguity error for Alice")
	}
	if !strings.Contains(err.Error(), "multiple directory matches") {
		t.Fatalf("ambiguity error = %q", err.Error())
	}

	got, err = selectDirectoryPersona("Alice", candidates, true)
	if err != nil {
		t.Fatalf("allow ambiguous error: %v", err)
	}
	if got.Email != "alice.adams@example.com" {
		t.Fatalf("allow ambiguous selected = %q, want alice.adams@example.com", got.Email)
	}
}

func TestSelectPeoplesCalendarGroupID(t *testing.T) {
	groups := []struct {
		GroupID   string `json:"GroupId"`
		GroupName string `json:"GroupName"`
		GroupType int    `json:"GroupType"`
	}{
		{GroupID: "my", GroupType: 0},
		{GroupID: "other", GroupType: 1},
		{GroupID: "people", GroupType: 2},
	}
	if got := selectPeoplesCalendarGroupID(groups); got != "people" {
		t.Fatalf("selectPeoplesCalendarGroupID() = %q, want people", got)
	}

	groups = []struct {
		GroupID   string `json:"GroupId"`
		GroupName string `json:"GroupName"`
		GroupType int    `json:"GroupType"`
	}{
		{GroupID: "", GroupType: 2},
		{GroupID: "fallback", GroupType: 1},
	}
	if got := selectPeoplesCalendarGroupID(groups); got != "fallback" {
		t.Fatalf("fallback group id = %q, want fallback", got)
	}
}

func TestCalendarGroupsResponseErrorCodeSupportsNumber(t *testing.T) {
	raw := []byte(`{"CalendarGroups":[],"ErrorCode":0,"WasSuccessful":true}`)
	var payload calendarGroupsResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if string(payload.ErrorCode) != "0" {
		t.Fatalf("ErrorCode = %q, want %q", payload.ErrorCode, "0")
	}
	if !payload.ErrorCode.IsSuccess() {
		t.Fatal("ErrorCode=0 should be success")
	}
}

func TestServiceErrorCodeIsSuccess(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{code: "", want: true},
		{code: "0", want: true},
		{code: "NoError", want: true},
		{code: "1", want: false},
		{code: "ErrorAccessDenied", want: false},
	}
	for _, tc := range cases {
		got := serviceErrorCode(tc.code).IsSuccess()
		if got != tc.want {
			t.Fatalf("IsSuccess(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestBuildDirectoryLinkedMailboxInfo(t *testing.T) {
	fallback := map[string]interface{}{
		"type":        "UserMailbox",
		"mailboxRank": "Coprincipal",
	}
	got := buildDirectoryLinkedMailboxInfo("user@example.com", fallback)
	want := map[string]interface{}{
		"type":               "UserMailbox",
		"userIdentity":       "user@example.com",
		"mailboxSmtpAddress": "user@example.com",
		"mailboxRank":        "Coprincipal",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildDirectoryLinkedMailboxInfo() = %#v, want %#v", got, want)
	}
}

func TestParseCreateCalendarMutationResultV2(t *testing.T) {
	raw := []byte(`{
	  "data": {
	    "createCalendar": {
	      "id": "cal-123",
	      "changeKey": "ck-123",
	      "parentGroupId": "grp-1",
	      "name": "Example User"
	    }
	  }
	}`)

	got, err := parseCreateCalendarMutationResult(raw)
	if err != nil {
		t.Fatalf("parseCreateCalendarMutationResult error: %v", err)
	}
	if got.FolderID.ID != "cal-123" {
		t.Fatalf("FolderID.ID = %q, want cal-123", got.FolderID.ID)
	}
	if got.CalendarID.ID != "cal-123" {
		t.Fatalf("CalendarID.ID = %q, want cal-123", got.CalendarID.ID)
	}
	if got.FolderID.ChangeKey != "ck-123" {
		t.Fatalf("FolderID.ChangeKey = %q, want ck-123", got.FolderID.ChangeKey)
	}
	if got.CalendarID.ChangeKey != "ck-123" {
		t.Fatalf("CalendarID.ChangeKey = %q, want ck-123", got.CalendarID.ChangeKey)
	}
}

func TestParseCreateCalendarMutationResultOWAModuleShape(t *testing.T) {
	raw := []byte(`{
	  "FolderId": {
	    "Id": "folder-123",
	    "ChangeKey": "folder-ck"
	  },
	  "calendarId": {
	    "id": "calendar-123",
	    "changeKey": "calendar-ck"
	  }
	}`)

	got, err := parseCreateCalendarMutationResult(raw)
	if err != nil {
		t.Fatalf("parseCreateCalendarMutationResult error: %v", err)
	}
	if got.FolderID.ID != "folder-123" {
		t.Fatalf("FolderID.ID = %q, want folder-123", got.FolderID.ID)
	}
	if got.FolderID.ChangeKey != "folder-ck" {
		t.Fatalf("FolderID.ChangeKey = %q, want folder-ck", got.FolderID.ChangeKey)
	}
	if got.CalendarID.ID != "calendar-123" {
		t.Fatalf("CalendarID.ID = %q, want calendar-123", got.CalendarID.ID)
	}
	if got.CalendarID.ChangeKey != "calendar-ck" {
		t.Fatalf("CalendarID.ChangeKey = %q, want calendar-ck", got.CalendarID.ChangeKey)
	}
}

func TestParseCreateCalendarMutationResultOWAModuleCalendarIDOnly(t *testing.T) {
	raw := []byte(`{
	  "FolderId": null,
	  "calendarId": {
	    "id": "calendar-only-123",
	    "changeKey": "calendar-only-ck"
	  }
	}`)

	got, err := parseCreateCalendarMutationResult(raw)
	if err != nil {
		t.Fatalf("parseCreateCalendarMutationResult error: %v", err)
	}
	if got.CalendarID.ID != "calendar-only-123" {
		t.Fatalf("CalendarID.ID = %q, want calendar-only-123", got.CalendarID.ID)
	}
	if got.FolderID.ID != "calendar-only-123" {
		t.Fatalf("FolderID.ID = %q, want calendar-only-123", got.FolderID.ID)
	}
	if got.CalendarID.ChangeKey != "calendar-only-ck" {
		t.Fatalf("CalendarID.ChangeKey = %q, want calendar-only-ck", got.CalendarID.ChangeKey)
	}
	if got.FolderID.ChangeKey != "calendar-only-ck" {
		t.Fatalf("FolderID.ChangeKey = %q, want calendar-only-ck", got.FolderID.ChangeKey)
	}
}

func TestFindAddedDirectoryCalendarFolderID(t *testing.T) {
	before := map[string]struct{}{
		"folder-existing": {},
	}
	after := []CalendarFolder{
		{DisplayName: "Calendar", FolderID: "folder-existing"},
		{DisplayName: "Example User", FolderID: "folder-new"},
	}

	got, ok := findAddedDirectoryCalendarFolderID(before, after, "Example User", "", "user@example.com")
	if !ok {
		t.Fatal("expected new folder match")
	}
	if got != "folder-new" {
		t.Fatalf("folder id = %q, want folder-new", got)
	}
}

func TestFindExistingDirectoryCalendarFolder(t *testing.T) {
	folders := []CalendarFolder{
		{DisplayName: "Calendar", FolderID: "folder-calendar"},
		{DisplayName: "Example User", FolderID: "folder-sample"},
	}
	folderID, name, ok := findExistingDirectoryCalendarFolder(folders, "", "example user", "user@example.com")
	if !ok {
		t.Fatal("expected existing folder match")
	}
	if folderID != "folder-sample" {
		t.Fatalf("folderID = %q, want folder-sample", folderID)
	}
	if name != "Example User" {
		t.Fatalf("name = %q, want Example User", name)
	}
}

func TestShouldTrySyncTeammatesCalendarFallback(t *testing.T) {
	err := errors.New("create calendar failed: Error occurred during processing the request. More details in error code(s) (Object value expected but found null instead: createCalendar)")
	if !shouldTrySyncTeammatesCalendarFallback(err) {
		t.Fatal("expected sync fallback")
	}
}

func TestShouldWarmCreateCalendarModule(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{err: errors.New("owa webpack runtime unavailable"), want: true},
		{err: errors.New("create calendar module not found"), want: true},
		{err: errors.New("create calendar export missing"), want: true},
		{err: errors.New("createCalendarMutationWeb: failed to create calendar. Error code: 3"), want: false},
	}
	for _, tc := range cases {
		if got := shouldWarmCreateCalendarModule(tc.err); got != tc.want {
			t.Fatalf("shouldWarmCreateCalendarModule(%q) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestEnsureCalendarSessionRoutingSetsAnchorMailbox(t *testing.T) {
	token := testCalendarJWTWithClaims(t, map[string]interface{}{
		"tid": "11111111-2222-3333-4444-555555555555",
	})
	tokens := &Tokens{Bearer: "Bearer " + token}
	mailbox := map[string]interface{}{
		"mailboxSmtpAddress": "user@example.com",
	}
	ensureCalendarSessionRouting(tokens, mailbox)
	if tokens.Session.AnchorMailbox != "user@example.com" {
		t.Fatalf("AnchorMailbox = %q, want user@example.com", tokens.Session.AnchorMailbox)
	}
	if tokens.Session.TenantID != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("TenantID = %q, want 11111111-2222-3333-4444-555555555555", tokens.Session.TenantID)
	}
}

func TestTenantIDFromBearerToken(t *testing.T) {
	token := testCalendarJWTWithClaims(t, map[string]interface{}{
		"tid": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	})
	got := tenantIDFromBearerToken("Bearer " + token)
	if got != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("tenantIDFromBearerToken() = %q, want aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", got)
	}
}

func testCalendarJWTWithClaims(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	headerBytes, err := json.Marshal(map[string]interface{}{
		"alg": "none",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("marshal jwt header: %v", err)
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal jwt claims: %v", err)
	}
	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + ".sig"
}
