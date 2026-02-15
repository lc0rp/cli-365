package owa

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/go-rod/rod"
)

var (
	errPageNotOWA    = errors.New("page not on owa")
	errTokensMissing = errors.New("tokens missing")
)

// IsSessionValid reports whether the current page can successfully call the OWA service API.
// This is stronger than IsLoggedIn(), which only checks whether tokens are present.
func IsSessionValid(page *rod.Page) bool {
	status, err := probeSessionStatus(page)
	if err != nil {
		return false
	}
	return !isAuthFailureStatus(status)
}

func probeSessionStatus(page *rod.Page) (int, error) {
	if page == nil {
		return 0, errors.New("page is nil")
	}
	info, err := page.Info()
	if err != nil || info == nil || !isOWAURL(info.URL) {
		return 0, errPageNotOWA
	}

	tokens, err := DiscoverTokens(page)
	if err != nil || tokens == nil || (tokens.Canary == "" && tokens.Bearer == "") {
		return 0, errTokensMissing
	}

	// Minimal "am I authorized" probe. Distinguished folder avoids needing cached folder ids.
	body := buildGetFolderRequest("inbox")
	resp, err := CallOWAAction(page, tokens, "GetFolder", body)
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, errors.New("probe response is nil")
	}
	return resp.Status, nil
}

type clickResult struct {
	Clicked bool   `json:"clicked"`
	Text    string `json:"text,omitempty"`
}

func tryClickReauth(page *rod.Page) (bool, string, error) {
	if page == nil {
		return false, "", errors.New("page is nil")
	}

	result, err := page.Eval(`() => {
		const isVisible = (el) => {
			if (!el) return false;
			const style = window.getComputedStyle(el);
			if (!style || style.visibility === "hidden" || style.display === "none") return false;
			const r = el.getBoundingClientRect();
			return r && r.width > 0 && r.height > 0;
		};
		const textOf = (el) => {
			const t = (el.innerText || el.textContent || el.getAttribute("aria-label") || "").trim();
			return t.replace(/\\s+/g, " ");
		};
		const match = (t) => {
			const lower = t.toLowerCase();
			if (!lower) return false;
			if (lower.includes("sign out")) return false;
			return lower === "sign in" || lower.includes("sign in again") || lower.includes("need to sign in") || lower.includes("sign in to continue");
		};
		const candidates = Array.from(document.querySelectorAll("button,a,[role='button'],input[type='button'],input[type='submit']"));
		for (const el of candidates) {
			const t = textOf(el);
			if (!match(t)) continue;
			if (!isVisible(el)) continue;
			try {
				el.click();
				return { clicked: true, text: t };
			} catch {}
		}
		return { clicked: false };
	}`)
	if err != nil {
		return false, "", err
	}
	if result.Value.Nil() {
		return false, "", nil
	}

	var parsed clickResult
	if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &parsed); err != nil {
		return false, "", err
	}
	return parsed.Clicked, parsed.Text, nil
}

// WaitForSessionValid waits until the current page can successfully call the OWA service API.
// Best-effort recovery: if we have tokens but get 401/440, try clicking a "Sign in" banner/button.
func WaitForSessionValid(page *rod.Page, timeout time.Duration) error {
	if page == nil {
		return errors.New("page is nil")
	}
	if timeout <= 0 {
		return errors.New("timeout must be positive")
	}

	deadline := time.Now().Add(timeout)
	probeInterval := 2 * time.Second
	reauthInterval := 2 * time.Second

	var lastProbe time.Time
	var lastReauth time.Time

	for time.Now().Before(deadline) {
		if time.Since(lastProbe) >= probeInterval {
			status, err := probeSessionStatus(page)
			if err == nil && !isAuthFailureStatus(status) {
				return nil
			}

			// If we have tokens but are unauthorized, try to trigger the OWA "sign in again" flow.
			if !errors.Is(err, errTokensMissing) && isAuthFailureStatus(status) {
				if time.Since(lastReauth) >= reauthInterval {
					_, _, _ = tryClickReauth(page)
					lastReauth = time.Now()
				}
			}
			lastProbe = time.Now()
		}

		time.Sleep(500 * time.Millisecond)
	}

	return errors.New("login timeout - session not valid")
}

func isAuthFailureStatus(status int) bool {
	switch status {
	case 401, 440, 449:
		return true
	default:
		return false
	}
}
