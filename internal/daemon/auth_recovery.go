package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lc0rp/cli-365/internal/browser"
	"github.com/lc0rp/cli-365/internal/paths"
)

type AuthNotification struct {
	Severity             string
	Reason               string
	LoginURL             string
	SecureInputURL       string
	SecureInputExpiresAt time.Time
	Detail               string
	QueueDepth           int
	At                   time.Time
}

type authStage string

const (
	authStageUnknown        authStage = "unknown"
	authStagePasswordInput  authStage = "password_input"
	authStagePasswordError  authStage = "password_error"
	authStageOTPInput       authStage = "otp_input"
	authStageMFANumberMatch authStage = "mfa_number_match"
	authStageMFAPush        authStage = "mfa_push_approval"
	authStageMFAOther       authStage = "mfa_other"
	authStageKMSI           authStage = "stay_signed_in"
)

const (
	maxPasswordSecurePrompts = 3
	maxOTPSecurePrompts      = 3
	maxKMSIAttempts          = 3
	securePromptCooldown     = 500 * time.Millisecond
)

var authCodePattern = regexp.MustCompile(`\b\d{1,3}\b`)

type authStageInfo struct {
	URL           string
	ChallengeCode string
	Detail        string
}

type authProbeFunc func(ctx context.Context) (bool, error)
type secureInputRunnerFunc func(ctx context.Context, command string, onURL func(string)) error
type notifyAuthFunc func(ctx context.Context, note AuthNotification) error

func (s *Server) currentAuthState() AuthState {
	s.authMu.RLock()
	defer s.authMu.RUnlock()
	return s.authState
}

func (s *Server) setAuthState(state AuthState) {
	s.authMu.Lock()
	s.authState = state
	s.authMu.Unlock()
}

func (s *Server) runAuthRecovery(parent context.Context) bool {
	timeout := s.opts.AuthRecoveryTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	s.setAuthState(AuthStateRecovering)
	s.setPaused(true)
	defer s.setPaused(false)
	s.logEvent("warn", "auth_recovery_start", map[string]interface{}{
		"state":       string(AuthStateRecovering),
		"login_url":   s.opts.LoginURL,
		"queue_depth": len(s.execQ),
	})

	_ = s.notifyAuth(ctx, AuthNotification{
		Severity:   "warning",
		Reason:     "auth_required",
		LoginURL:   s.opts.LoginURL,
		QueueDepth: len(s.execQ),
		At:         time.Now().UTC(),
	})

	secureInputExpiresAt := time.Time{}
	if deadline, ok := ctx.Deadline(); ok {
		secureInputExpiresAt = deadline.UTC()
	}
	runtimeEndpoint := ""
	if rt, err := browser.LoadRuntime(); err == nil && rt != nil {
		runtimeEndpoint = strings.TrimSpace(rt.WSEndpoint)
	}
	secureInputCommand := prepareSecureInputCommand(
		s.opts.SecureInputCommand,
		s.opts.CDPPort,
		runtimeEndpoint,
		s.getPrimaryTabID(),
		paths.RuntimePath(),
	)
	var secureURLMu sync.Mutex
	lastSecureURL := ""
	notifySecureInputURL := func(url string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		secureURLMu.Lock()
		if url == lastSecureURL {
			secureURLMu.Unlock()
			return
		}
		lastSecureURL = url
		secureURLMu.Unlock()

		now := time.Now().UTC()
		expiresInSec := 0
		expiresAtText := ""
		if !secureInputExpiresAt.IsZero() {
			expiresAtText = secureInputExpiresAt.Format(time.RFC3339)
			expiresInSec = int(secureInputExpiresAt.Sub(now).Seconds())
			if expiresInSec < 0 {
				expiresInSec = 0
			}
		}
		s.logEvent("info", "auth_secure_input_url", map[string]interface{}{
			"secure_input_url":            url,
			"secure_input_expires_at":     expiresAtText,
			"secure_input_expires_in_sec": expiresInSec,
		})
		_ = s.notifyAuth(ctx, AuthNotification{
			Severity:             "warning",
			Reason:               "secure_input_url",
			LoginURL:             s.opts.LoginURL,
			SecureInputURL:       url,
			SecureInputExpiresAt: secureInputExpiresAt,
			QueueDepth:           len(s.execQ),
			At:                   now,
		})
	}
	if s.waitForAuthReady(ctx, secureInputCommand, secureInputExpiresAt, notifySecureInputURL) {
		s.setAuthState(AuthStateReady)
		s.logEvent("info", "auth_recovery_success", map[string]interface{}{
			"state":       string(AuthStateReady),
			"queue_depth": len(s.execQ),
		})
		return true
	}

	s.setAuthState(AuthStateFailed)
	s.logEvent("error", "auth_recovery_timeout", map[string]interface{}{
		"state":       string(AuthStateFailed),
		"login_url":   s.opts.LoginURL,
		"queue_depth": len(s.execQ),
	})
	_ = s.notifyAuth(context.Background(), AuthNotification{
		Severity:   "error",
		Reason:     "auth_timeout",
		LoginURL:   s.opts.LoginURL,
		QueueDepth: len(s.execQ),
		At:         time.Now().UTC(),
	})
	s.drainPending(ErrorCodeAuthTimeout, "daemon auth recovery timed out")
	return false
}

func (s *Server) waitForAuthReady(ctx context.Context, defaultSecureInputCommand string, secureInputExpiresAt time.Time, onURL func(string)) bool {
	interval := s.opts.AuthProbeInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	lastBrowserRecoveryAttempt := time.Time{}
	lastStage := authStage("")
	lastStageDetail := ""
	lastNotifiedCode := ""
	otpNotified := false
	pushNotified := false
	passwordPromptAttempts := 0
	otpPromptAttempts := 0
	kmsiAttempts := 0
	nextPasswordPromptAt := time.Time{}
	nextOTPPromptAt := time.Time{}
	nextKMSIPromptAt := time.Time{}

	otpSecureInputCommand := prepareSecureInputOTPCommand(
		s.opts.SecureInputCommand,
		s.opts.CDPPort,
		s.runtimeEndpointForRecovery(),
		s.getPrimaryTabID(),
		paths.RuntimePath(),
	)

	for {
		if s.stopRequested() {
			return false
		}
		if s.authProbe == nil {
			return false
		}

		stage, info := s.detectAuthStage(ctx)
		if stage != lastStage || info.Detail != lastStageDetail {
			lastStage = stage
			lastStageDetail = info.Detail
			s.logEvent("info", "auth_stage_update", map[string]interface{}{
				"stage":  string(stage),
				"url":    info.URL,
				"detail": info.Detail,
			})
		}

		now := time.Now()
		if shouldPromptSecureInputForStage(stage) && now.After(nextPasswordPromptAt) {
			if passwordPromptAttempts < maxPasswordSecurePrompts {
				passwordPromptAttempts++
				nextPasswordPromptAt = now.Add(securePromptCooldown)
				s.logEvent("info", "auth_secure_input_prompt", map[string]interface{}{
					"stage":   string(stage),
					"attempt": passwordPromptAttempts,
				})
				if err := s.runSecureInputAttempt(ctx, defaultSecureInputCommand, onURL); err != nil &&
					!errors.Is(err, context.Canceled) &&
					!errors.Is(err, context.DeadlineExceeded) {
					s.logEvent("warn", "auth_secure_input_error", map[string]interface{}{
						"stage":   string(stage),
						"attempt": passwordPromptAttempts,
						"error":   err.Error(),
					})
				}
				continue
			}
			if stage == authStagePasswordError {
				_ = s.notifyAuth(ctx, AuthNotification{
					Severity:             "warning",
					Reason:               "password_retry_exhausted",
					LoginURL:             s.opts.LoginURL,
					SecureInputExpiresAt: secureInputExpiresAt,
					QueueDepth:           len(s.execQ),
					Detail:               "password prompt remained after retries",
					At:                   now.UTC(),
				})
			}
		}

		if stage == authStageOTPInput && now.After(nextOTPPromptAt) {
			if !otpNotified {
				otpNotified = true
				_ = s.notifyAuth(ctx, AuthNotification{
					Severity:             "warning",
					Reason:               "mfa_otp_required",
					LoginURL:             s.opts.LoginURL,
					SecureInputExpiresAt: secureInputExpiresAt,
					QueueDepth:           len(s.execQ),
					Detail:               "otp input required",
					At:                   now.UTC(),
				})
			}
			if otpPromptAttempts < maxOTPSecurePrompts {
				otpPromptAttempts++
				nextOTPPromptAt = now.Add(securePromptCooldown)
				s.logEvent("info", "auth_secure_input_prompt", map[string]interface{}{
					"stage":   string(stage),
					"attempt": otpPromptAttempts,
				})
				if err := s.runSecureInputAttempt(ctx, otpSecureInputCommand, onURL); err != nil &&
					!errors.Is(err, context.Canceled) &&
					!errors.Is(err, context.DeadlineExceeded) {
					s.logEvent("warn", "auth_secure_input_error", map[string]interface{}{
						"stage":   string(stage),
						"attempt": otpPromptAttempts,
						"error":   err.Error(),
					})
				}
				continue
			}
		}

		if stage == authStageMFANumberMatch && info.ChallengeCode != "" && info.ChallengeCode != lastNotifiedCode {
			lastNotifiedCode = info.ChallengeCode
			_ = s.notifyAuth(ctx, AuthNotification{
				Severity:             "warning",
				Reason:               "mfa_number_challenge",
				LoginURL:             s.opts.LoginURL,
				SecureInputExpiresAt: secureInputExpiresAt,
				QueueDepth:           len(s.execQ),
				Detail:               "authenticator number: " + info.ChallengeCode,
				At:                   now.UTC(),
			})
		}
		if stage == authStageMFAPush && !pushNotified {
			pushNotified = true
			_ = s.notifyAuth(ctx, AuthNotification{
				Severity:             "warning",
				Reason:               "mfa_push_approval",
				LoginURL:             s.opts.LoginURL,
				SecureInputExpiresAt: secureInputExpiresAt,
				QueueDepth:           len(s.execQ),
				Detail:               "approve sign-in request in authenticator",
				At:                   now.UTC(),
			})
		}
		if stage == authStageKMSI && now.After(nextKMSIPromptAt) && kmsiAttempts < maxKMSIAttempts {
			kmsiAttempts++
			nextKMSIPromptAt = now.Add(securePromptCooldown)
			s.logEvent("info", "auth_kmsi_continue", map[string]interface{}{
				"attempt": kmsiAttempts,
			})
			if err := s.advanceStaySignedIn(ctx); err != nil {
				s.logEvent("warn", "auth_kmsi_continue_error", map[string]interface{}{
					"attempt": kmsiAttempts,
					"error":   err.Error(),
				})
			}
			continue
		}

		probeCtx, cancelProbe := context.WithTimeout(ctx, minDuration(interval, 2*time.Second))
		ok, err := s.authProbe(probeCtx)
		cancelProbe()
		if err == nil && ok {
			return true
		}
		if errors.Is(err, errSessionProbeUnavailable) {
			if lastBrowserRecoveryAttempt.IsZero() || now.Sub(lastBrowserRecoveryAttempt) >= interval {
				lastBrowserRecoveryAttempt = now
				s.tryRecoverBrowserDuringAuthRecovery(ctx)
			}
		} else if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return false
		}

		select {
		case <-s.stopCh:
			return false
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

func (s *Server) tryRecoverBrowserDuringAuthRecovery(ctx context.Context) {
	if s == nil {
		return
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}
	if s.tryRecoverBrowserForSession(ctx, deadline) {
		s.runPrimaryMaintenance()
	}
}

func (s *Server) runtimeEndpointForRecovery() string {
	rt, err := browser.LoadRuntime()
	if err != nil || rt == nil {
		return ""
	}
	return strings.TrimSpace(rt.WSEndpoint)
}

func (s *Server) runSecureInputAttempt(ctx context.Context, command string, onURL func(string)) error {
	if s == nil || s.secureInputRunner == nil {
		return nil
	}
	if s.stopRequested() {
		return context.Canceled
	}

	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-attemptCtx.Done():
		case <-s.stopCh:
			cancel()
		}
	}()

	err := s.secureInputRunner(attemptCtx, command, onURL)
	cancel()
	<-done
	return err
}

func shouldPromptSecureInputForStage(stage authStage) bool {
	switch stage {
	case authStagePasswordInput, authStagePasswordError, authStageUnknown:
		return true
	default:
		return false
	}
}

func (s *Server) detectAuthStage(ctx context.Context) (authStage, authStageInfo) {
	select {
	case <-ctx.Done():
		return authStageUnknown, authStageInfo{Detail: "context canceled"}
	default:
	}

	page, err := s.currentPrimaryOWAPage()
	if err != nil {
		return authStageUnknown, authStageInfo{Detail: "primary page unavailable"}
	}
	if page == nil {
		return authStageUnknown, authStageInfo{Detail: "no auth page"}
	}

	info := authStageInfo{}
	if pageInfo, err := page.Info(); err == nil && pageInfo != nil {
		info.URL = strings.TrimSpace(pageInfo.URL)
	}

	payload, err := page.Eval(`() => {
		const q = (s) => document.querySelector(s);
		const visible = (el) => {
			if (!el) return false;
			const style = window.getComputedStyle(el);
			const rect = el.getBoundingClientRect();
			return !!style && style.visibility !== 'hidden' && style.display !== 'none' && rect.width > 0 && rect.height > 0;
		};
		const text = (document.body && document.body.innerText ? document.body.innerText : '').toLowerCase();
		const pickText = (sel) => {
			const el = q(sel);
			if (!el || !visible(el)) return '';
			return (el.innerText || el.textContent || '').trim();
		};
		return {
			hasPassword: visible(q('input[name=passwd],#i0118,input[type=password]')),
			hasOTPInput: visible(q('input[name=otc],input[id*="otc"],input[name*="otp"],input[id*="Otp"],input[name*="verification"],input[autocomplete="one-time-code"]')),
			passwordError: text.includes('please enter your password') || text.includes('your account or password is incorrect') || text.includes('incorrect password') || visible(q('#passwordError')),
			pushPrompt: text.includes('approve sign in request') || text.includes('open your authenticator app') || text.includes('use your authenticator app'),
			kmsiPrompt: text.includes('stay signed in') && visible(q('#idSIButton9')),
			mfaPrompt: text.includes('verify your identity') || text.includes('verification code') || text.includes('one-time passcode') || text.includes('enter code'),
			numberChallenge: pickText('#idRichContext_DisplaySign') || pickText('[id*="DisplaySign"]') || '',
			snippet: text.slice(0, 400)
		};
	}`)
	if err != nil || payload.Value.Nil() {
		info.Detail = "unable to inspect auth page"
		return authStageUnknown, info
	}

	var state struct {
		HasPassword     bool   `json:"hasPassword"`
		HasOTPInput     bool   `json:"hasOTPInput"`
		PasswordError   bool   `json:"passwordError"`
		PushPrompt      bool   `json:"pushPrompt"`
		KMSIPrompt      bool   `json:"kmsiPrompt"`
		MFAPrompt       bool   `json:"mfaPrompt"`
		NumberChallenge string `json:"numberChallenge"`
		Snippet         string `json:"snippet"`
	}
	raw, _ := payload.Value.MarshalJSON()
	if err := json.Unmarshal(raw, &state); err != nil {
		info.Detail = "unable to parse auth page state"
		return authStageUnknown, info
	}
	info.Detail = strings.TrimSpace(state.Snippet)
	info.ChallengeCode = firstAuthChallengeCode(state.NumberChallenge)

	switch {
	case state.HasPassword && state.PasswordError:
		return authStagePasswordError, info
	case state.HasPassword:
		return authStagePasswordInput, info
	case state.HasOTPInput:
		return authStageOTPInput, info
	case info.ChallengeCode != "":
		return authStageMFANumberMatch, info
	case state.PushPrompt:
		return authStageMFAPush, info
	case state.KMSIPrompt:
		return authStageKMSI, info
	case state.MFAPrompt:
		return authStageMFAOther, info
	default:
		return authStageUnknown, info
	}
}

func firstAuthChallengeCode(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	match := authCodePattern.FindString(trimmed)
	return strings.TrimSpace(match)
}

func (s *Server) advanceStaySignedIn(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	page, err := s.currentPrimaryOWAPage()
	if err != nil {
		return err
	}
	if page == nil {
		return errors.New("stay-signed-in page unavailable")
	}

	result, err := page.Eval(`() => {
		const candidates = [
			'#idSIButton9',
			'button#idSIButton9',
			'input#idSIButton9',
			'button[type=submit]',
			'input[type=submit]'
		];
		for (const sel of candidates) {
			const el = document.querySelector(sel);
			if (!el) continue;
			const style = window.getComputedStyle(el);
			const rect = el.getBoundingClientRect();
			const visible = !!style && style.visibility !== 'hidden' && style.display !== 'none' && rect.width > 0 && rect.height > 0;
			if (!visible) continue;
			el.click();
			return true;
		}
		return false;
	}`)
	if err != nil {
		return err
	}
	if result.Value.Nil() || !result.Value.Bool() {
		return errors.New("stay-signed-in continue button not found")
	}
	return nil
}

func defaultAuthProbe(execFn ExecFunc) authProbeFunc {
	return func(ctx context.Context) (bool, error) {
		result := execFn(ctx, []string{"auth", "status", "--json"}, 5*time.Second)
		if result.Err != nil {
			if errors.Is(result.Err, context.Canceled) || errors.Is(result.Err, context.DeadlineExceeded) {
				return false, result.Err
			}
			return false, nil
		}
		if result.ExitCode != 0 {
			return false, nil
		}

		var payload struct {
			Authenticated bool `json:"authenticated"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &payload); err == nil {
			return payload.Authenticated, nil
		}

		lower := strings.ToLower(result.Stdout + "\n" + result.Stderr)
		return strings.Contains(lower, "authenticated: yes"), nil
	}
}

func (s *Server) defaultAuthRecoveryProbe(ctx context.Context) (bool, error) {
	if s != nil && s.sessionProbe != nil {
		valid, err := s.sessionProbe(ctx)
		switch {
		case err == nil:
			return valid, nil
		case errors.Is(err, errSessionProbeUnavailable):
			return false, errSessionProbeUnavailable
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return false, err
		default:
			s.logEvent("warn", "auth_recovery_probe_error", map[string]interface{}{
				"error": err.Error(),
			})
			return false, nil
		}
	}
	if s == nil || s.execFn == nil {
		return false, nil
	}
	return defaultAuthProbe(s.execFn)(ctx)
}

func defaultSecureInputRunner(ctx context.Context, command string, onURL func(string)) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("secure input command is empty")
	}

	parts := strings.Fields(command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return err
	}

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if url := firstURL(scanner.Text()); url != "" && onURL != nil {
				onURL(url)
			}
		}
	}()

	err = cmd.Wait()
	<-readDone
	if err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return fmt.Errorf("%w: %s", err, errText)
		}
		return err
	}
	return nil
}

func prepareSecureInputCommand(command string, cdpPort int, cdpEndpoint string, targetTabID string, runtimeFile string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	if !isSecureTargetedInputCommand(parts[0]) {
		return command
	}
	if !hasCLIFlag(parts, "--selector") {
		parts = append(
			parts,
			"--selector", "input[name=passwd],#i0118,input[type=password]",
			"--selector-2", "input[type=email],input[name=loginfmt],#i0116",
			"--submit-selector", "#idSIButton9,button[type=submit],input[type=submit]",
			"--page-timeout", "10s",
		)
	} else if !hasCLIFlag(parts, "--selector-2") {
		parts = append(parts, "--selector-2", "input[type=email],input[name=loginfmt],#i0116")
	}
	if !hasCLIFlag(parts, "--submit-selector") {
		parts = append(parts, "--submit-selector", "#idSIButton9,button[type=submit],input[type=submit]")
	}
	runtimeFile = strings.TrimSpace(runtimeFile)
	if runtimeFile != "" &&
		!hasCLIFlag(parts, "--cli-365-runtime-config-file") &&
		!hasCLIFlag(parts, "--runtime-file") {
		parts = append(parts, "--cli-365-runtime-config-file", runtimeFile)
	}
	if !hasCLIFlag(parts, "--cdp-port") && !hasCLIFlag(parts, "--cdp-endpoint") && !hasCLIFlag(parts, "--ws-endpoint") {
		cdpEndpoint = strings.TrimSpace(cdpEndpoint)
		switch {
		case cdpPort > 0:
			parts = append(parts, "--cdp-port", fmt.Sprintf("%d", cdpPort))
		case runtimeFile != "":
			// Let secure-targeted-input resolve the current endpoint from runtime on each submit.
		case cdpEndpoint != "":
			parts = append(parts, "--cdp-endpoint", cdpEndpoint)
		}
	}
	targetTabID = strings.TrimSpace(targetTabID)
	if targetTabID != "" &&
		!hasCLIFlag(parts, "--target-tab-id") &&
		!hasCLIFlag(parts, "--target-id") &&
		!hasCLIFlag(parts, "--target-tab-url") &&
		!hasCLIFlag(parts, "--page-url") {
		parts = append(parts, "--target-tab-id", targetTabID)
	}
	if !hasCLIFlag(parts, "--target-tab-url") && !hasCLIFlag(parts, "--page-url") {
		parts = append(parts, "--target-tab-url", "microsoftonline.com")
	}
	return strings.Join(parts, " ")
}

func prepareSecureInputOTPCommand(command string, cdpPort int, cdpEndpoint string, targetTabID string, runtimeFile string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}

	parts := strings.Fields(command)
	if len(parts) == 0 || !isSecureTargetedInputCommand(parts[0]) {
		return command
	}
	if !hasCLIFlag(parts, "--selector") {
		parts = append(
			parts,
			"--selector", "input[name=otc],input[id*=otc],input[name*=otp],input[id*=Otp],input[name*=verification],input[autocomplete=one-time-code]",
			"--page-timeout", "10s",
		)
	}
	if !hasCLIFlag(parts, "--submit-selector") {
		parts = append(parts, "--submit-selector", "#idSubmit_SAOTCC_Continue,#idSIButton9")
	}
	runtimeFile = strings.TrimSpace(runtimeFile)
	if runtimeFile != "" &&
		!hasCLIFlag(parts, "--cli-365-runtime-config-file") &&
		!hasCLIFlag(parts, "--runtime-file") {
		parts = append(parts, "--cli-365-runtime-config-file", runtimeFile)
	}
	if !hasCLIFlag(parts, "--cdp-port") && !hasCLIFlag(parts, "--cdp-endpoint") && !hasCLIFlag(parts, "--ws-endpoint") {
		cdpEndpoint = strings.TrimSpace(cdpEndpoint)
		switch {
		case cdpPort > 0:
			parts = append(parts, "--cdp-port", fmt.Sprintf("%d", cdpPort))
		case runtimeFile != "":
		case cdpEndpoint != "":
			parts = append(parts, "--cdp-endpoint", cdpEndpoint)
		}
	}
	targetTabID = strings.TrimSpace(targetTabID)
	if targetTabID != "" &&
		!hasCLIFlag(parts, "--target-tab-id") &&
		!hasCLIFlag(parts, "--target-id") &&
		!hasCLIFlag(parts, "--target-tab-url") &&
		!hasCLIFlag(parts, "--page-url") {
		parts = append(parts, "--target-tab-id", targetTabID)
	}
	if !hasCLIFlag(parts, "--target-tab-url") && !hasCLIFlag(parts, "--page-url") {
		parts = append(parts, "--target-tab-url", "microsoftonline.com")
	}
	return strings.Join(parts, " ")
}

func isSecureTargetedInputCommand(binary string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(binary)))
	if base == "secure-targeted-input" {
		return true
	}
	return strings.HasPrefix(base, "secure-targeted-input.")
}

func hasCLIFlag(parts []string, name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}

	for _, part := range parts {
		value := strings.ToLower(strings.TrimSpace(part))
		if value == name || strings.HasPrefix(value, name+"=") {
			return true
		}
	}
	return false
}

func firstURL(line string) string {
	for _, field := range strings.Fields(strings.TrimSpace(line)) {
		candidate := strings.TrimSpace(field)
		candidate = strings.Trim(candidate, `"'()[]{}<>,`)
		lower := strings.ToLower(candidate)
		if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
			return candidate
		}
	}
	return ""
}

func defaultNotifyAuth(provider, openClawCmd, channel, target string) notifyAuthFunc {
	provider = strings.TrimSpace(strings.ToLower(provider))
	openClawCmd = strings.TrimSpace(openClawCmd)
	channel = strings.TrimSpace(channel)
	target = strings.TrimSpace(target)

	return func(ctx context.Context, note AuthNotification) error {
		if provider != "openclaw-cli" {
			return nil
		}
		if openClawCmd == "" {
			return errors.New("openclaw command is empty")
		}
		cmdParts := strings.Fields(openClawCmd)
		if len(cmdParts) == 0 {
			return errors.New("openclaw command is empty")
		}

		message := fmt.Sprintf(
			"service=cli-365 severity=%s reason=%s login_url=%s secure_input_url=%s secure_input_expires_at=%s secure_input_expires_in=%s queue_depth=%d at=%s",
			note.Severity,
			note.Reason,
			note.LoginURL,
			note.SecureInputURL,
			formatSecureInputExpiresAt(note.SecureInputExpiresAt),
			formatSecureInputExpiresIn(note.At, note.SecureInputExpiresAt),
			note.QueueDepth,
			note.At.Format(time.RFC3339),
		)
		if detail := strings.TrimSpace(note.Detail); detail != "" {
			message += " detail=" + strings.ReplaceAll(detail, "\n", " ")
		}

		args := []string{"message", "send"}
		if channel != "" {
			args = append(args, "--channel", channel)
		}
		if target != "" {
			args = append(args, "--target", target)
		}
		args = append(args, "--message", message)
		finalArgs := make([]string, 0, len(cmdParts)-1+len(args))
		if len(cmdParts) > 1 {
			finalArgs = append(finalArgs, cmdParts[1:]...)
		}
		finalArgs = append(finalArgs, args...)

		cmd := exec.CommandContext(ctx, cmdParts[0], finalArgs...)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run()
	}
}

func formatSecureInputExpiresAt(expiresAt time.Time) string {
	if expiresAt.IsZero() {
		return ""
	}
	return expiresAt.UTC().Format(time.RFC3339)
}

func formatSecureInputExpiresIn(at time.Time, expiresAt time.Time) string {
	if expiresAt.IsZero() {
		return ""
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	remaining := expiresAt.Sub(at)
	if remaining < 0 {
		remaining = 0
	}
	return remaining.Round(time.Second).String()
}

func shouldAttemptAuthRecovery(commandPath string, argv []string, result ExecResult) bool {
	if result.ExitCode == 0 && result.Err == nil {
		return false
	}
	if errors.Is(result.Err, context.DeadlineExceeded) || errors.Is(result.Err, context.Canceled) {
		return false
	}
	if errors.Is(result.Err, ErrAuthRequired) {
		return true
	}

	path := strings.ToLower(strings.TrimSpace(commandPath))
	if path == "" {
		path = strings.ToLower(strings.TrimSpace(inferCommandPath(argv)))
	}
	if !strings.HasPrefix(path, "mail") &&
		!strings.HasPrefix(path, "calendar") &&
		!strings.HasPrefix(path, "auth login") {
		return false
	}

	text := strings.ToLower(result.Stdout + "\n" + result.Stderr)
	if result.Err != nil {
		text += "\n" + strings.ToLower(result.Err.Error())
	}
	for _, needle := range []string{
		"not logged in",
		"auth login",
		"login timeout",
		"status 401",
		"unauthorized",
		"authentication required",
		"auth required",
		"failed to extract canary",
		"canary token not found",
		"tokens not found and page is nil",
		"missing bearer token",
		"login.microsoftonline.com",
		"startupdata.ashx",
		"user did not complete authentication",
	} {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func authRecoveryFailureResponse(requestID string, msg string) Response {
	if strings.TrimSpace(msg) == "" {
		msg = "daemon auth recovery timed out"
	}
	resp := daemonFailureResponse(requestID, ErrorCodeAuthTimeout, msg)
	resp.ErrorCode = ErrorCodeAuthTimeout
	return resp
}
