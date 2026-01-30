package owa

// FeatureCatalog describes packaged OWA actions.
type FeatureCatalog struct {
	Actions map[string]FeatureDefinition `json:"actions"`
}

// FeatureDefinition describes a single action capability.
type FeatureDefinition struct {
	Name        string `json:"name"`
	Area        string `json:"area,omitempty"`
	Description string `json:"description,omitempty"`
}

// DefaultFeatureCatalog returns the built-in, universal action list.
func DefaultFeatureCatalog() FeatureCatalog {
	return FeatureCatalog{
		Actions: map[string]FeatureDefinition{
			"FindItem": {
				Name:        "FindItem",
				Area:        "mail",
				Description: "Search messages",
			},
			"FindConversation": {
				Name:        "FindConversation",
				Area:        "mail",
				Description: "Search conversations",
			},
			"GetItem": {
				Name:        "GetItem",
				Area:        "mail",
				Description: "Fetch a single message",
			},
			"GetConversationItems": {
				Name:        "GetConversationItems",
				Area:        "mail",
				Description: "Fetch conversation messages",
			},
			"CreateItem": {
				Name:        "CreateItem",
				Area:        "mail",
				Description: "Create a draft or send message",
			},
			"UpdateItem": {
				Name:        "UpdateItem",
				Area:        "mail",
				Description: "Update a draft",
			},
			"DeleteItem": {
				Name:        "DeleteItem",
				Area:        "mail",
				Description: "Delete a draft",
			},
			"SendItem": {
				Name:        "SendItem",
				Area:        "mail",
				Description: "Send a draft",
			},
			"GetAttachment": {
				Name:        "GetAttachment",
				Area:        "mail",
				Description: "Download attachment content",
			},
		},
	}
}

// IsKnown reports whether the action exists in the catalog.
func (c FeatureCatalog) IsKnown(action string) bool {
	if action == "" {
		return false
	}
	_, ok := c.Actions[action]
	return ok
}
