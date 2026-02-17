package owa

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
	if canary == "" {
		// Fallback to in-page extraction
		canary, err = getCanaryFromPage(page)
		if err != nil && !isNonFatalCanaryEvalError(err) {
			return nil, fmt.Errorf("failed to extract canary: %w", err)
		}
	}
	if canary == "" {
		canary, _ = getCanaryFromStartupData(page)
	}

	// Get bearer tokens from storage
	bearers, _ := getBearerTokensFromStorage(page)

	if canary == "" && bearers.OWA == "" {
		return nil, errors.New("canary token not found - are you logged in?")
	}

	// Get user email from page state
	userEmail, _ := getUserEmailFromPage(page)
	// Get session headers from page
	session, _ := getSessionHeadersFromPage(page)
	if session.AnchorMailbox == "" && userEmail != "" {
		session.AnchorMailbox = userEmail
	}

	tokens := &Tokens{
		Canary:      canary,
		Bearer:      bearers.OWA,
		GraphBearer: bearers.Graph,
		Substrate:   bearers.Substrate,
		UserEmail:   userEmail,
		Session:     session,
		ExtractedAt: time.Now(),
	}

	return tokens, nil
}

func getSessionHeadersFromPage(page *rod.Page) (SessionHeaders, error) {
	result, err := page.Eval(`() => {
		const isGuid = (v) => typeof v === "string" && /^[0-9a-fA-F-]{8}-[0-9a-fA-F-]{4}-[0-9a-fA-F-]{4}-[0-9a-fA-F-]{4}-[0-9a-fA-F-]{12}$/.test(v);
		const looksLikeAnchorMailbox = (v) => typeof v === "string" && v.length > 0 && (
			v.startsWith("PUID:") || v.startsWith("SMTP:") || v.startsWith("OID:") || v.includes("@")
		);
		const looksLikePrefer = (v) => typeof v === "string" && v.length > 0 && /exchange\\.behavior/i.test(v);
		const pick = (obj, keys) => {
			if (!obj || typeof obj !== "object") return null;
			for (const k of keys) {
				if (obj[k]) return obj[k];
			}
			return null;
		};
		const merge = (dst, src) => {
			if (!src) return dst;
			if (src.sessionId && !dst.sessionId) dst.sessionId = src.sessionId;
			if (src.anchorMailbox && !dst.anchorMailbox) dst.anchorMailbox = src.anchorMailbox;
			if (src.tenantId && !dst.tenantId) dst.tenantId = src.tenantId;
			if (src.prefer && !dst.prefer) dst.prefer = src.prefer;
			return dst;
		};
		const fromObj = (obj) => {
			if (!obj || typeof obj !== "object") return null;
			const sessionId = pick(obj, ["sessionId", "SessionId", "owaSessionId", "OWASessionId"]);
			const tenantId = pick(obj, ["tenantId", "TenantId"]);
			const anchorMailbox = pick(obj, ["anchorMailbox", "AnchorMailbox", "primarySmtpAddress", "PrimarySmtpAddress"]);
			const prefer = pick(obj, ["prefer", "Prefer"]);
			const out = {};
			if (sessionId && isGuid(sessionId)) out.sessionId = sessionId;
			if (tenantId && isGuid(tenantId)) out.tenantId = tenantId;
			if (anchorMailbox && typeof anchorMailbox === "string") out.anchorMailbox = anchorMailbox;
			if (prefer && typeof prefer === "string" && looksLikePrefer(prefer)) out.prefer = prefer;
			return Object.keys(out).length ? out : null;
		};
		const out = {};

		const w = window;
		merge(out, fromObj(w.owa && w.owa.sessionData));
		merge(out, fromObj(w.owaSettings));
		merge(out, fromObj(w.__owa));
		merge(out, fromObj(w.__OWA));

		for (const key of Object.keys(w)) {
			if (!/STATE|CONFIG|SESSION/i.test(key)) continue;
			try {
				merge(out, fromObj(w[key]));
			} catch {}
		}

		const scanStorage = (storage) => {
			if (!storage) return;
			for (const key of Object.keys(storage)) {
				if (!/session|owa|tenant|anchor|prefer/i.test(key)) continue;
				const raw = storage.getItem(key);
				if (!raw) continue;

				// Some sessions store anchor mailbox + prefer as plain strings.
				if (!out.anchorMailbox && /anchor/i.test(key) && looksLikeAnchorMailbox(raw)) {
					out.anchorMailbox = raw;
				}
				if (!out.prefer && /prefer/i.test(key) && looksLikePrefer(raw)) {
					out.prefer = raw;
				}

				if (isGuid(raw)) {
					if (!out.tenantId && /tenant/i.test(key)) {
						out.tenantId = raw;
						continue;
					}
					if (!out.sessionId && /session/i.test(key)) {
						out.sessionId = raw;
						continue;
					}
					// Unknown GUID value; ignore.
					continue;
				}
				try {
					const parsed = JSON.parse(raw);
					merge(out, fromObj(parsed));
				} catch {}
			}
		};

		try { scanStorage(localStorage); } catch {}
		try { scanStorage(sessionStorage); } catch {}

		return out;
	}`)
	if err != nil {
		return SessionHeaders{}, err
	}
	if result.Value.Nil() {
		return SessionHeaders{}, nil
	}

	var parsed struct {
		SessionID     string `json:"sessionId"`
		AnchorMailbox string `json:"anchorMailbox"`
		TenantID      string `json:"tenantId"`
		Prefer        string `json:"prefer"`
	}
	if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &parsed); err != nil {
		return SessionHeaders{}, err
	}

	return SessionHeaders{
		SessionID:     parsed.SessionID,
		AnchorMailbox: parsed.AnchorMailbox,
		TenantID:      parsed.TenantID,
		Prefer:        parsed.Prefer,
	}, nil
}

// getCanaryFromCookies extracts the X-OWA-CANARY cookie value.
func getCanaryFromCookies(page *rod.Page) (string, error) {
	canaryNames := []string{"X-OWA-CANARY", "OWA-CANARY", "XOWACANARY"}

	cookies, err := page.Cookies([]string{})
	if err == nil {
		for _, cookie := range cookies {
			for _, name := range canaryNames {
				if strings.EqualFold(cookie.Name, name) && cookie.Value != "" {
					return cookie.Value, nil
				}
			}
		}
	}

	// Fallback: fetch all cookies via CDP (some domains may not match current URL).
	_ = proto.NetworkEnable{}.Call(page)
	all, err := proto.NetworkGetAllCookies{}.Call(page)
	if err != nil {
		if err != nil && len(cookies) == 0 {
			return "", err
		}
		return "", nil
	}

	for _, cookie := range all.Cookies {
		for _, name := range canaryNames {
			if strings.EqualFold(cookie.Name, name) && cookie.Value != "" {
				return cookie.Value, nil
			}
		}
	}

	return "", nil
}

func getCanaryFromStartupData(page *rod.Page) (string, error) {
	if page == nil {
		return "", errors.New("page is nil")
	}

	info, err := page.Info()
	if err != nil {
		return "", err
	}
	if info == nil || info.URL == "" {
		return "", errors.New("page url missing")
	}

	parsed, err := url.Parse(info.URL)
	if err != nil {
		return "", err
	}
	origin := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	req := FetchRequest{
		URL:    origin + "/owa/startupdata.ashx?app=Mail&n=0",
		Method: "POST",
	}

	resp, err := Fetch(page, req)
	if err != nil {
		return "", err
	}

	for key, val := range resp.Headers {
		if strings.EqualFold(key, "x-owa-canary") && val != "" {
			return val, nil
		}
	}

	var payload interface{}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		// If body is a string, check for a canary-like key.
		var raw string
		if err := json.Unmarshal(resp.Body, &raw); err == nil {
			return extractCanaryFromString(raw), nil
		}
		return "", nil
	}

	return extractCanaryFromValue(payload), nil
}

func extractCanaryFromValue(v interface{}) string {
	switch val := v.(type) {
	case map[string]interface{}:
		for key, child := range val {
			if strings.Contains(strings.ToLower(key), "canary") {
				if s, ok := child.(string); ok && s != "" {
					return s
				}
			}
			if s := extractCanaryFromValue(child); s != "" {
				return s
			}
		}
	case []interface{}:
		for _, child := range val {
			if s := extractCanaryFromValue(child); s != "" {
				return s
			}
		}
	}
	return ""
}

func extractCanaryFromString(raw string) string {
	lower := strings.ToLower(raw)
	idx := strings.Index(lower, "canary")
	if idx < 0 {
		return ""
	}
	return ""
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

func isNonFatalCanaryEvalError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "securityerror") {
		return true
	}
	return strings.Contains(msg, "failed to read the 'cookie' property from 'document'")
}

type bearerTokens struct {
	OWA       string `json:"owa"`
	Graph     string `json:"graph"`
	Substrate string `json:"substrate"`
}

// getBearerFromStorage extracts the OWA bearer token from storage.
func getBearerFromStorage(page *rod.Page) (string, error) {
	tokens, err := getBearerTokensFromStorage(page)
	if err != nil {
		return "", err
	}
	return tokens.OWA, nil
}

// getBearerTokensFromStorage extracts bearer tokens for OWA, Graph, and Substrate.
func getBearerTokensFromStorage(page *rod.Page) (bearerTokens, error) {
	var empty bearerTokens
	result, err := page.Eval(`() => {
		const tokens = [];
		const decodeJwt = (token) => {
			try {
				const parts = token.split(".");
				if (parts.length < 2) return null;
				const b64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
				const json = atob(b64);
				return JSON.parse(json);
			} catch {
				return null;
			}
		};

		const scoreAud = (aud) => {
			if (!aud) return 0;
			if (Array.isArray(aud)) {
				return aud.reduce((max, item) => Math.max(max, scoreAud(item)), 0);
			}
			if (typeof aud !== "string") return 0;
			if (aud === "https://outlook.office.com") return 3;
			if (aud.includes("https://outlook.office.com") && !aud.includes("/search")) return 2;
			if (aud.includes("https://outlook.office.com")) return 1;
			return 0;
		};

		const matchAud = (aud, target) => {
			if (!aud || !target) return false;
			if (Array.isArray(aud)) {
				return aud.some((item) => matchAud(item, target));
			}
			if (typeof aud !== "string") return false;
			if (aud === target) return true;
			return aud.includes(target);
		};

		const pickToken = (predicate, scoreFn) => {
			let best = null;
			let bestScore = -1;
			for (const entry of tokens) {
				if (!predicate(entry.aud)) continue;
				const score = scoreFn ? scoreFn(entry.aud) : 1;
				if (score > bestScore) {
					bestScore = score;
					best = entry.token;
				}
			}
			return best;
		};

		// Check localStorage
		for (const key of Object.keys(localStorage || {})) {
			if (!/accesstoken/i.test(key)) continue;
			const raw = localStorage.getItem(key);
			if (!raw) continue;
			try {
				const parsed = JSON.parse(raw);
				if (parsed.secret && parsed.tokenType) {
					const payload = decodeJwt(parsed.secret);
					tokens.push({
						token: parsed.tokenType + " " + parsed.secret,
						aud: payload && payload.aud ? payload.aud : "",
					});
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
					const payload = decodeJwt(parsed.token);
					tokens.push({
						token: parsed.tokenType + " " + parsed.token,
						aud: payload && payload.aud ? payload.aud : "",
					});
				}
			} catch {}
		}

		if (!tokens.length) return null;
		return {
			owa: pickToken((aud) => matchAud(aud, "https://outlook.office.com"), scoreAud),
			graph: pickToken((aud) => matchAud(aud, "https://graph.microsoft.com")),
			substrate: pickToken((aud) => matchAud(aud, "https://substrate.office.com")),
		};
	}`)
	if err != nil {
		return empty, err
	}

	if result.Value.Nil() {
		return empty, nil
	}

	var out bearerTokens
	if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &out); err != nil {
		return empty, err
	}

	return out, nil
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
	startupInterval := 3 * time.Second
	lastStartupCheck := time.Time{}

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
		if isOutlookHostURL(info.URL) && time.Since(lastStartupCheck) >= startupInterval {
			if canary, err := getCanaryFromStartupData(page); err == nil && canary != "" {
				return nil
			}
			lastStartupCheck = time.Now()
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
	if canary != "" {
		return true
	}
	bearer, _ := getBearerFromStorage(page)
	return bearer != ""
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

// WaitForLoggedIn waits up to timeout for a valid OWA session.
func WaitForLoggedIn(page *rod.Page, timeout time.Duration) error {
	if page == nil {
		return errors.New("page is nil")
	}
	if timeout <= 0 {
		return errors.New("timeout must be positive")
	}

	deadline := time.Now().Add(timeout)
	startupInterval := 3 * time.Second
	lastStartupCheck := time.Time{}
	for time.Now().Before(deadline) {
		if IsLoggedIn(page) {
			return nil
		}
		info, err := page.Info()
		if err == nil && isOutlookHostURL(info.URL) && time.Since(lastStartupCheck) >= startupInterval {
			if canary, err := getCanaryFromStartupData(page); err == nil && canary != "" {
				return nil
			}
			lastStartupCheck = time.Now()
		}
		time.Sleep(500 * time.Millisecond)
	}

	return errors.New("login timeout - not logged in")
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
