package owa

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"

	"github.com/lc0rp/cli-365/internal/paths"
)

const (
	OWABaseURL     = "https://outlook.office.com/mail/"
	OWAAPIBase     = "https://outlook.office.com/owa/0/service.svc"
	TokenCacheFile = "tokens.json"
)

// Client manages OWA browser session and tokens.
type Client struct {
	browser *rod.Browser
	page    *rod.Page
	tokens  *Tokens
}

// Tokens holds the extracted OWA authentication tokens.
type Tokens struct {
	Canary      string            `json:"canary"`
	Bearer      string            `json:"bearer,omitempty"`
	GraphBearer string            `json:"graph_bearer,omitempty"`
	Substrate   string            `json:"substrate_bearer,omitempty"`
	UserEmail   string            `json:"user_email,omitempty"`
	Session     SessionHeaders    `json:"session,omitempty"`
	Folders     map[string]string `json:"folders,omitempty"`
	ExtractedAt time.Time         `json:"extracted_at"`
	ExpiresAt   time.Time         `json:"expires_at,omitempty"`
}

// MergeTokens merges non-empty fields from src into dst.
func MergeTokens(dst, src *Tokens) *Tokens {
	if dst == nil {
		return src
	}
	if src == nil {
		return dst
	}
	if src.Canary != "" {
		dst.Canary = src.Canary
	}
	if src.Bearer != "" {
		dst.Bearer = src.Bearer
	}
	if src.GraphBearer != "" {
		dst.GraphBearer = src.GraphBearer
	}
	if src.Substrate != "" {
		dst.Substrate = src.Substrate
	}
	if src.UserEmail != "" {
		dst.UserEmail = src.UserEmail
	}
	if !src.Session.IsZero() {
		dst.Session = MergeSessionHeaders(dst.Session, src.Session)
	}
	if !src.ExpiresAt.IsZero() {
		dst.ExpiresAt = src.ExpiresAt
	}
	if !src.ExtractedAt.IsZero() {
		dst.ExtractedAt = src.ExtractedAt
	}
	if len(src.Folders) > 0 {
		if dst.Folders == nil {
			dst.Folders = map[string]string{}
		}
		for k, v := range src.Folders {
			if v != "" {
				dst.Folders[k] = v
			}
		}
	}
	return dst
}

// NewClient creates a new OWA client from an existing browser connection.
func NewClient(browser *rod.Browser) *Client {
	return &Client{
		browser: browser,
	}
}

// Connect connects to or creates a page for OWA.
func (c *Client) Connect() error {
	if c.browser == nil {
		return errors.New("browser not initialized")
	}

	pages, err := c.browser.Pages()
	if err != nil {
		return fmt.Errorf("failed to get pages: %w", err)
	}

	// Look for existing OWA page with valid canary
	var firstOWA *rod.Page
	for _, p := range pages {
		info, err := p.Info()
		if err != nil {
			continue
		}
		if !isOWAURL(info.URL) {
			continue
		}
		if firstOWA == nil {
			firstOWA = p
		}
		canary, _ := getCanaryFromCookies(p)
		if canary == "" {
			canary, _ = getCanaryFromPage(p)
		}
		if canary != "" {
			c.page = p
			return nil
		}
	}
	if firstOWA != nil {
		c.page = firstOWA
		return nil
	}

	// Create new page if none found
	c.page = c.browser.MustPage("about:blank")
	return nil
}

// Page returns the current page.
func (c *Client) Page() *rod.Page {
	return c.page
}

// Tokens returns the current tokens.
func (c *Client) Tokens() *Tokens {
	return c.tokens
}

// SetTokens sets the tokens.
func (c *Client) SetTokens(t *Tokens) {
	c.tokens = t
}

// Close closes the page (but not the browser).
func (c *Client) Close() error {
	if c.page != nil {
		return c.page.Close()
	}
	return nil
}

// TokenCachePath returns the path to the token cache file.
func TokenCachePath() string {
	return filepath.Join(paths.StateDir(), "cli-365", TokenCacheFile)
}

// LoadTokens loads cached tokens from disk.
func LoadTokens() (*Tokens, error) {
	path := TokenCachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tokens Tokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, err
	}
	return &tokens, nil
}

// SaveTokens saves tokens to disk.
func SaveTokens(t *Tokens) error {
	path := TokenCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadOrDiscoverTokens loads cached tokens if available, otherwise re-discovers
// tokens from a logged-in page and attempts to save them.
func LoadOrDiscoverTokens(page *rod.Page) (*Tokens, error) {
	tokens, err := LoadTokens()
	if err == nil && tokens != nil && tokens.Canary != "" {
		if page != nil {
			updated := false
			if bearerTokens, err := getBearerTokensFromStorage(page); err == nil {
				if bearerTokens.OWA != "" && bearerTokens.OWA != tokens.Bearer {
					tokens.Bearer = bearerTokens.OWA
					updated = true
				}
				if bearerTokens.Graph != "" && bearerTokens.Graph != tokens.GraphBearer {
					tokens.GraphBearer = bearerTokens.Graph
					updated = true
				}
				if bearerTokens.Substrate != "" && bearerTokens.Substrate != tokens.Substrate {
					tokens.Substrate = bearerTokens.Substrate
					updated = true
				}
			}
			if email, err := getUserEmailFromPage(page); err == nil && email != "" && email != tokens.UserEmail {
				tokens.UserEmail = email
				updated = true
			}
			if session, err := getSessionHeadersFromPage(page); err == nil {
				if session.AnchorMailbox == "" && tokens.UserEmail != "" {
					session.AnchorMailbox = tokens.UserEmail
				}
				if !session.IsZero() {
					tokens.Session = MergeSessionHeaders(tokens.Session, session)
					updated = true
				}
			}
			if updated {
				_ = SaveTokens(tokens)
			}
		}
		return tokens, nil
	}

	if page == nil {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("tokens not found and page is nil")
	}

	tokens, err = DiscoverTokens(page)
	if err != nil {
		return nil, err
	}

	// Non-fatal if caching fails; caller can still proceed with in-page tokens.
	_ = SaveTokens(tokens)

	return tokens, nil
}

// ClearTokens removes the cached tokens.
func ClearTokens() error {
	path := TokenCachePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LoadTokensWithKeyring loads tokens using keyring storage, falling back to file.
func LoadTokensWithKeyring(storageType string) (*Tokens, error) {
	// Try keyring first if configured
	if storageType != "" && storageType != "plain" {
		// Import keyring package inline to avoid circular dependency
		// In practice, the caller should use keyring.TokenStorage directly
		tokens, err := LoadTokens()
		if err == nil {
			return tokens, nil
		}
	}
	return LoadTokens()
}

// SaveTokensWithKeyring saves tokens using keyring storage.
func SaveTokensWithKeyring(t *Tokens, storageType string) error {
	// For now, save to file. Callers can use keyring.TokenStorage for secure storage.
	return SaveTokens(t)
}

// ClearTokensWithKeyring removes tokens from keyring storage.
func ClearTokensWithKeyring(storageType string) error {
	return ClearTokens()
}

func isOutlookHostURL(url string) bool {
	return len(url) > 0 && (contains(url, "outlook.office.com") ||
		contains(url, "outlook.office365.com") ||
		contains(url, "outlook.live.com") ||
		contains(url, "outlook.cloud.microsoft"))
}

func isOWAURL(url string) bool {
	if !isOutlookHostURL(url) {
		return false
	}
	return contains(url, "/mail") || contains(url, "/owa/") || contains(url, "/calendar")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
