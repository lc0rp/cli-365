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
	Canary      string    `json:"canary"`
	Bearer      string    `json:"bearer,omitempty"`
	UserEmail   string    `json:"user_email,omitempty"`
	ExtractedAt time.Time `json:"extracted_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
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

func isOWAURL(url string) bool {
	return len(url) > 0 && (contains(url, "outlook.office.com/mail") ||
		contains(url, "outlook.office365.com/mail") ||
		contains(url, "outlook.live.com/mail") ||
		contains(url, "outlook.cloud.microsoft/mail"))
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
