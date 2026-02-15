package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"

	"github.com/lc0rp/cli-365/internal/browser"
	"github.com/lc0rp/cli-365/internal/owa"
)

const (
	defaultTokenRefreshLead  = 5 * time.Minute
	defaultSessionProbeLimit = 5 * time.Second
)

var errSessionProbeUnavailable = errors.New("session probe unavailable")

type tokenLoaderFunc func() (*owa.Tokens, error)
type tokenSaverFunc func(tokens *owa.Tokens) error
type tokenRefresherFunc func(ctx context.Context) (*owa.Tokens, error)
type sessionProbeFunc func(ctx context.Context) (bool, error)

func defaultTokenLoader() (*owa.Tokens, error) {
	return owa.LoadTokens()
}

func defaultTokenSaver(tokens *owa.Tokens) error {
	return owa.SaveTokens(tokens)
}

func shouldManageSession(commandPath string, argv []string) bool {
	path := normalizedCommandPath(commandPath, argv)
	switch {
	case path == "mail":
		return true
	case strings.HasPrefix(path, "mail "):
		return true
	case path == "calendar":
		return true
	case strings.HasPrefix(path, "calendar "):
		return true
	default:
		return false
	}
}

func (s *Server) ensureSessionReady(parent context.Context, deadline time.Time, commandPath string, argv []string) bool {
	if !shouldManageSession(commandPath, argv) {
		return true
	}
	if !s.shouldCleanupManagedBrowser() {
		return true
	}

	remaining := time.Until(deadline)
	if remaining <= 0 {
		return false
	}

	refreshCtx, refreshCancel := context.WithTimeout(parent, minDuration(remaining, defaultSessionProbeLimit))
	refreshErr := s.refreshTokensIfNeeded(refreshCtx, time.Now().UTC())
	refreshCancel()
	if refreshErr != nil && !errors.Is(refreshErr, context.Canceled) && !errors.Is(refreshErr, context.DeadlineExceeded) {
		s.logEvent("warn", "session_token_refresh_error", map[string]interface{}{
			"error": refreshErr.Error(),
		})
	}

	probeFn := s.sessionProbe
	if probeFn == nil {
		return true
	}

	probeCtx, probeCancel := context.WithTimeout(parent, minDuration(remaining, defaultSessionProbeLimit))
	valid, err := probeFn(probeCtx)
	probeCancel()
	if err != nil {
		if errors.Is(err, errSessionProbeUnavailable) {
			if !s.tryRecoverBrowserForSession(parent, deadline) {
				return true
			}

			remaining = time.Until(deadline)
			if remaining <= 0 {
				return false
			}

			retryCtx, retryCancel := context.WithTimeout(parent, minDuration(remaining, defaultSessionProbeLimit))
			retryValid, retryErr := probeFn(retryCtx)
			retryCancel()
			if retryErr != nil {
				if errors.Is(retryErr, errSessionProbeUnavailable) {
					return true
				}
				if !errors.Is(retryErr, context.Canceled) && !errors.Is(retryErr, context.DeadlineExceeded) {
					s.logEvent("warn", "session_probe_error", map[string]interface{}{
						"error": retryErr.Error(),
					})
				}
				return true
			}
			if retryValid {
				return true
			}
			return s.runAuthRecovery(parent)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return true
		}
		s.logEvent("warn", "session_probe_error", map[string]interface{}{
			"error": err.Error(),
		})
		return true
	}
	if valid {
		return true
	}

	return s.runAuthRecovery(parent)
}

func (s *Server) refreshTokensIfNeeded(ctx context.Context, now time.Time) error {
	loadTokens := s.tokenLoader
	if loadTokens == nil {
		return nil
	}
	cached, err := loadTokens()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if cached == nil {
		return nil
	}

	expiresAt, ok := tokenExpiry(cached)
	if !ok {
		return nil
	}

	lead := s.tokenRefreshLead
	if lead <= 0 {
		lead = defaultTokenRefreshLead
	}
	if expiresAt.After(now.Add(lead)) {
		return nil
	}

	refreshTokens := s.tokenRefresher
	if refreshTokens == nil {
		return nil
	}
	refreshed, err := refreshTokens(ctx)
	if err != nil {
		return err
	}
	if refreshed == nil {
		return errors.New("token refresh returned nil tokens")
	}

	merged := owa.MergeTokens(cached, refreshed)
	if merged == nil {
		merged = refreshed
	}
	if refreshedExp, ok := tokenExpiry(merged); ok {
		merged.ExpiresAt = refreshedExp
	}

	saveTokens := s.tokenSaver
	if saveTokens == nil {
		return nil
	}
	if err := saveTokens(merged); err != nil {
		return err
	}

	s.logEvent("info", "session_token_refreshed", map[string]interface{}{
		"expires_at": merged.ExpiresAt.Format(time.RFC3339),
	})
	return nil
}

func (s *Server) defaultTokenRefresher(ctx context.Context) (*owa.Tokens, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	page, err := s.currentPrimaryOWAPage()
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, errors.New("primary owa tab is unavailable")
	}

	tokens, err := owa.DiscoverTokens(page)
	if err != nil {
		return nil, err
	}
	if expiresAt, ok := tokenExpiry(tokens); ok {
		tokens.ExpiresAt = expiresAt
	}
	return tokens, nil
}

func (s *Server) defaultSessionProbe(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	page, err := s.currentPrimaryOWAPage()
	if err != nil {
		return false, err
	}
	if page == nil {
		return false, errSessionProbeUnavailable
	}
	return owa.IsLoggedIn(page), nil
}

func (s *Server) tryRecoverBrowserForSession(parent context.Context, deadline time.Time) bool {
	if s.execFn == nil {
		return false
	}

	remaining := time.Until(deadline)
	if remaining <= 0 {
		return false
	}
	timeout := minDuration(remaining, 30*time.Second)

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	result := s.execFn(ctx, []string{"browser", "start"}, timeout)
	if result.ExitCode == 0 && result.Err == nil {
		s.runPrimaryMaintenance()
		s.logEvent("info", "session_browser_recovered", map[string]interface{}{
			"timeout_ms": timeout.Milliseconds(),
		})
		return true
	}

	msg := strings.TrimSpace(result.Stderr)
	if msg == "" && result.Err != nil {
		msg = result.Err.Error()
	}
	if msg == "" {
		msg = "browser start recovery failed"
	}
	s.logEvent("warn", "session_browser_recovery_failed", map[string]interface{}{
		"error": msg,
	})
	return false
}

func (s *Server) currentPrimaryOWAPage() (*rod.Page, error) {
	rt, err := browser.LoadRuntime()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if rt == nil || strings.TrimSpace(rt.WSEndpoint) == "" {
		return nil, nil
	}

	b := s.getTabBrowser(rt.WSEndpoint)
	if b == nil {
		return nil, nil
	}

	pages, err := b.Pages()
	if err != nil {
		s.resetTabBrowser()
		return nil, nil
	}

	primaryID := s.getPrimaryTabID()
	var fallback *rod.Page
	for _, page := range pages {
		info, err := page.Info()
		if err != nil || info == nil {
			continue
		}
		id := strings.TrimSpace(string(info.TargetID))
		if primaryID != "" && id == primaryID {
			return page, nil
		}
		if fallback == nil && isOWAURLForDaemon(info.URL) {
			fallback = page
		}
	}
	return fallback, nil
}

func tokenExpiry(tokens *owa.Tokens) (time.Time, bool) {
	if tokens == nil {
		return time.Time{}, false
	}
	if !tokens.ExpiresAt.IsZero() {
		return tokens.ExpiresAt.UTC(), true
	}
	for _, raw := range []string{tokens.Bearer, tokens.GraphBearer, tokens.Substrate} {
		if expiresAt, ok := parseJWTExpiry(raw); ok {
			return expiresAt, true
		}
	}
	return time.Time{}, false
}

func parseJWTExpiry(raw string) (time.Time, bool) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return time.Time{}, false
	}
	if strings.Contains(token, " ") {
		parts := strings.Fields(token)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			token = parts[1]
		}
	}

	jwtParts := strings.Split(token, ".")
	if len(jwtParts) < 2 {
		return time.Time{}, false
	}

	payloadRaw, err := decodeJWTPart(jwtParts[1])
	if err != nil {
		return time.Time{}, false
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return time.Time{}, false
	}

	expRaw, ok := payload["exp"]
	if !ok {
		return time.Time{}, false
	}

	expUnix, ok := toUnixSeconds(expRaw)
	if !ok || expUnix <= 0 {
		return time.Time{}, false
	}

	return time.Unix(expUnix, 0).UTC(), true
}

func decodeJWTPart(seg string) ([]byte, error) {
	if seg == "" {
		return nil, errors.New("jwt payload segment is empty")
	}
	if rem := len(seg) % 4; rem != 0 {
		seg += strings.Repeat("=", 4-rem)
	}
	return base64.URLEncoding.DecodeString(seg)
}

func toUnixSeconds(raw interface{}) (int64, bool) {
	switch v := raw.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}
