package owa

import (
	"encoding/json"
	"testing"
)

// Export buildSearchMessagesBody for testing
var BuildSearchMessagesBody = buildSearchMessagesBody

func TestBuildSearchMessagesBody(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		folderID   string
		maxResults int
		wantQuery  bool
		wantFolder string
	}{
		{
			name:       "defaults",
			query:      "",
			folderID:   "",
			maxResults: 0,
			wantQuery:  false,
			wantFolder: "inbox",
		},
		{
			name:       "with query",
			query:      "test search",
			folderID:   "",
			maxResults: 25,
			wantQuery:  true,
			wantFolder: "inbox",
		},
		{
			name:       "with custom folder",
			query:      "",
			folderID:   "custom-folder-id",
			maxResults: 10,
			wantQuery:  false,
			wantFolder: "custom-folder-id",
		},
		{
			name:       "all params",
			query:      "important",
			folderID:   "folder-123",
			maxResults: 100,
			wantQuery:  true,
			wantFolder: "folder-123",
		},
		{
			name:       "negative maxResults defaults to 50",
			query:      "",
			folderID:   "",
			maxResults: -5,
			wantQuery:  false,
			wantFolder: "inbox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := buildSearchMessagesBody(tt.query, tt.folderID, tt.maxResults)

			// Verify it serializes to valid JSON
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			if len(data) == 0 {
				t.Fatal("serialized body is empty")
			}

			// Check ItemShape exists
			itemShape, ok := body["ItemShape"].(map[string]interface{})
			if !ok {
				t.Fatal("ItemShape missing or wrong type")
			}
			if itemShape["BaseShape"] != "IdOnly" {
				t.Errorf("BaseShape = %v, want IdOnly", itemShape["BaseShape"])
			}

			// Check additional properties
			props, ok := itemShape["AdditionalProperties"].([]map[string]interface{})
			if !ok {
				t.Fatal("AdditionalProperties missing")
			}
			if len(props) < 5 {
				t.Errorf("AdditionalProperties has %d items, want at least 5", len(props))
			}

			// Check paging
			paging, ok := body["Paging"].(map[string]interface{})
			if !ok {
				t.Fatal("Paging missing")
			}
			expectedMax := tt.maxResults
			if expectedMax <= 0 {
				expectedMax = 50
			}
			if paging["MaxEntriesReturned"] != expectedMax {
				t.Errorf("MaxEntriesReturned = %v, want %d", paging["MaxEntriesReturned"], expectedMax)
			}

			// Check parent folder IDs
			folders, ok := body["ParentFolderIds"].([]map[string]interface{})
			if !ok || len(folders) == 0 {
				t.Fatal("ParentFolderIds missing or empty")
			}

			if tt.folderID != "" {
				if folders[0]["__type"] != "FolderId:#Exchange" {
					t.Errorf("folder type = %v, want FolderId:#Exchange", folders[0]["__type"])
				}
				if folders[0]["Id"] != tt.folderID {
					t.Errorf("folder Id = %v, want %s", folders[0]["Id"], tt.folderID)
				}
			} else {
				if folders[0]["__type"] != "DistinguishedFolderId:#Exchange" {
					t.Errorf("folder type = %v, want DistinguishedFolderId:#Exchange", folders[0]["__type"])
				}
				if folders[0]["Id"] != "inbox" {
					t.Errorf("folder Id = %v, want inbox", folders[0]["Id"])
				}
			}

			// Check restriction (query)
			_, hasRestriction := body["Restriction"]
			if tt.wantQuery && !hasRestriction {
				t.Error("expected Restriction for query, but not found")
			}
			if !tt.wantQuery && hasRestriction {
				t.Error("unexpected Restriction without query")
			}

			if hasRestriction {
				restriction := body["Restriction"].(map[string]interface{})
				if restriction["__type"] != "Contains:#Exchange" {
					t.Errorf("restriction type = %v, want Contains:#Exchange", restriction["__type"])
				}
				constant := restriction["Constant"].(map[string]interface{})
				if constant["Value"] != tt.query {
					t.Errorf("query value = %v, want %s", constant["Value"], tt.query)
				}
			}

			// Check sort order
			sortOrder, ok := body["SortOrder"].([]map[string]interface{})
			if !ok || len(sortOrder) == 0 {
				t.Fatal("SortOrder missing or empty")
			}
			if sortOrder[0]["Order"] != "Descending" {
				t.Errorf("sort order = %v, want Descending", sortOrder[0]["Order"])
			}
		})
	}
}

func TestBuildSearchMessagesBodyJSON(t *testing.T) {
	body := buildSearchMessagesBody("test", "", 20)

	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Parse back to verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	requiredKeys := []string{"ItemShape", "Paging", "ViewFilter", "SortOrder", "ParentFolderIds", "Restriction"}
	for _, key := range requiredKeys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing required key: %s", key)
		}
	}
}
