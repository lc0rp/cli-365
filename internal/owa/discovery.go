package owa

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// DiscoverTokens extracts canary and bearer tokens from the current OWA page.
func DiscoverTokens(page *rod.Page) (*Tokens, error) {
	if page == nil {
		return nil, errors.New("page is nil")
	}

	// First try to get canary from cookies
	canary, err := getCanaryFromCookies(page)
	if err != nil || canary == "" {
		// Fallback to in-page extraction
		canary, err = getCanaryFromPage(page)
		if err != nil {
			return nil, fmt.Errorf("failed to extract canary: %w", err)
		}
	}

	if canary == "" {
		return nil, errors.New("canary token not found - are you logged in?")
	}

	// Get bearer token from localStorage
	bearer, _ := getBearerFromStorage(page)

	// Get user email from page state
	userEmail, _ := getUserEmailFromPage(page)

	tokens := &Tokens{
		Canary:      canary,
		Bearer:      bearer,
		UserEmail:   userEmail,
		ExtractedAt: time.Now(),
	}

	return tokens, nil
}

// getCanaryFromCookies extracts the X-OWA-CANARY cookie value.
func getCanaryFromCookies(page *rod.Page) (string, error) {
	cookies, err := page.Cookies([]string{})
	if err != nil {
		return "", err
	}

	canaryNames := []string{"X-OWA-CANARY", "OWA-CANARY", "XOWACANARY"}
	for _, cookie := range cookies {
		for _, name := range canaryNames {
			if strings.EqualFold(cookie.Name, name) && cookie.Value != "" {
				return cookie.Value, nil
			}
		}
	}

	return "", nil
}

// getCanaryFromPage extracts canary from document.cookie or global variables.
func getCanaryFromPage(page *rod.Page) (string, error) {
	result, err := page.Eval(`() => {
		// Try document.cookie first
		const cookie = document.cookie || "";
		for (const key of ["X-OWA-CANARY", "OWA-CANARY", "XOWACANARY"]) {
			const match = cookie.match(new RegExp(key + "=([^;]+)"));
			if (match && match[1]) return decodeURIComponent(match[1]);
		}

		// Try known global variables
		const w = window;
		if (w.owa && w.owa.canary) return w.owa.canary;
		if (w.owaSettings && w.owaSettings.canary) return w.owaSettings.canary;
		if (w.__owa && w.__owa.canary) return w.__owa.canary;
		if (w.__OWA_CANARY__) return w.__OWA_CANARY__;

		// Try to find it in __INITIAL_STATE__ or similar
		for (const key of Object.keys(w)) {
			if (key.includes("INITIAL") || key.includes("STATE") || key.includes("CONFIG")) {
				try {
					const val = w[key];
					if (val && typeof val === "object") {
						if (val.canary) return val.canary;
						if (val.sessionSettings && val.sessionSettings.canary) return val.sessionSettings.canary;
					}
				} catch {}
			}
		}

		return null;
	}`)
	if err != nil {
		return "", err
	}

	if result.Value.Nil() {
		return "", nil
	}

	return result.Value.Str(), nil
}

// getBearerFromStorage extracts bearer token from localStorage.
func getBearerFromStorage(page *rod.Page) (string, error) {
	result, err := page.Eval(`() => {
		const tokens = [];
		const matchesTarget = (key) =>
			/https:\/\/outlook\.office\.com|https:\/\/outlook\.cloud\.microsoft/i.test(key);

		// Check localStorage
		for (const key of Object.keys(localStorage || {})) {
			if (!/accesstoken/i.test(key)) continue;
			if (!matchesTarget(key)) continue;
			const raw = localStorage.getItem(key);
			if (!raw) continue;
			try {
				const parsed = JSON.parse(raw);
				if (parsed.secret && parsed.tokenType) {
					tokens.push(parsed.tokenType + " " + parsed.secret);
				}
			} catch {}
		}

		// Check sessionStorage
		for (const key of Object.keys(sessionStorage || {})) {
			if (!/token|auth/i.test(key)) continue;
			const raw = sessionStorage.getItem(key);
			if (!raw) continue;
			try {
				const parsed = JSON.parse(raw);
				if (parsed.token && parsed.tokenType) {
					tokens.push(parsed.tokenType + " " + parsed.token);
				}
			} catch {}
		}

		return tokens[0] || null;
	}`)
	if err != nil {
		return "", err
	}

	if result.Value.Nil() {
		return "", nil
	}

	return result.Value.Str(), nil
}

// getUserEmailFromPage extracts the current user's email from page state.
func getUserEmailFromPage(page *rod.Page) (string, error) {
	result, err := page.Eval(`() => {
		const w = window;
		
		// Try various known locations
		if (w.owa && w.owa.sessionData && w.owa.sessionData.primarySmtpAddress) {
			return w.owa.sessionData.primarySmtpAddress;
		}
		if (w.owaSettings && w.owaSettings.primarySmtpAddress) {
			return w.owaSettings.primarySmtpAddress;
		}
		
		// Try __INITIAL_STATE__ patterns
		for (const key of Object.keys(w)) {
			if (key.includes("INITIAL") || key.includes("STATE")) {
				try {
					const val = w[key];
					if (val && typeof val === "object") {
						if (val.sessionSettings && val.sessionSettings.primarySmtpAddress) {
							return val.sessionSettings.primarySmtpAddress;
						}
						if (val.userConfiguration && val.userConfiguration.SessionSettings && 
						    val.userConfiguration.SessionSettings.UserEmailAddress) {
							return val.userConfiguration.SessionSettings.UserEmailAddress;
						}
					}
				} catch {}
			}
		}

		return null;
	}`)
	if err != nil {
		return "", err
	}

	if result.Value.Nil() {
		return "", nil
	}

	return result.Value.Str(), nil
}

// NavigateToOWA navigates to OWA and waits for it to load.
func NavigateToOWA(page *rod.Page) error {
	if err := page.Navigate(OWABaseURL); err != nil {
		return fmt.Errorf("failed to navigate: %w", err)
	}
	
	// Wait for network to be idle
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("failed to wait for load: %w", err)
	}

	return nil
}

// WaitForLogin waits for the user to complete login and OWA to load.
func WaitForLogin(page *rod.Page, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		info, err := page.Info()
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Check if we're on OWA mail page
		if isOWAURL(info.URL) {
			// Try to extract canary to confirm we're logged in
			canary, _ := getCanaryFromCookies(page)
			if canary == "" {
				canary, _ = getCanaryFromPage(page)
			}
			if canary != "" {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return errors.New("login timeout - user did not complete authentication")
}

// IsLoggedIn checks if the current page has valid OWA tokens.
func IsLoggedIn(page *rod.Page) bool {
	if page == nil {
		return false
	}

	info, err := page.Info()
	if err != nil || !isOWAURL(info.URL) {
		return false
	}

	canary, _ := getCanaryFromCookies(page)
	if canary == "" {
		canary, _ = getCanaryFromPage(page)
	}

	return canary != ""
}

// EnsureLoggedIn checks if logged in and navigates to OWA if needed.
func EnsureLoggedIn(page *rod.Page) error {
	if IsLoggedIn(page) {
		return nil
	}

	info, err := page.Info()
	if err != nil || !isOWAURL(info.URL) {
		if err := NavigateToOWA(page); err != nil {
			return err
		}
	}

	// Give it a moment to redirect/load
	time.Sleep(2 * time.Second)

	if !IsLoggedIn(page) {
		return errors.New("not logged in - run 'auth login' first")
	}

	return nil
}

// SetCanaryCookie ensures the canary is set as a request header cookie.
func SetCanaryCookie(page *rod.Page, canary string) error {
	return page.SetCookies([]*proto.NetworkCookieParam{
		{
			Name:   "X-OWA-CANARY",
			Value:  canary,
			Domain: "outlook.office.com",
			Path:   "/",
		},
	})
}
