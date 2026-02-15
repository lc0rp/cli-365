package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type AuthNotification struct {
	Severity   string
	Reason     string
	LoginURL   string
	QueueDepth int
	At         time.Time
}

type authProbeFunc func(ctx context.Context) (bool, error)
type secureInputRunnerFunc func(ctx context.Context, command string) error
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

	secureDone := make(chan error, 1)
	go func() {
		if s.secureInputRunner == nil {
			secureDone <- nil
			return
		}
		secureDone <- s.secureInputRunner(ctx, s.opts.SecureInputCommand)
	}()

	if s.waitForAuthReady(ctx, secureDone) {
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

func (s *Server) waitForAuthReady(ctx context.Context, secureDone <-chan error) bool {
	interval := s.opts.AuthProbeInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if s.authProbe == nil {
			return false
		}
		ok, err := s.authProbe(ctx)
		if err == nil && ok {
			return true
		}
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return false
		}

		select {
		case <-ctx.Done():
			return false
		case err, ok := <-secureDone:
			secureDone = nil
			if ok && err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				return false
			}
		case <-ticker.C:
		}
	}
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

func defaultSecureInputRunner(ctx context.Context, command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("secure input command is empty")
	}

	parts := strings.Fields(command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	return cmd.Run()
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

		message := fmt.Sprintf(
			"service=cli-365 severity=%s reason=%s login_url=%s queue_depth=%d at=%s",
			note.Severity,
			note.Reason,
			note.LoginURL,
			note.QueueDepth,
			note.At.Format(time.RFC3339),
		)

		args := []string{"message", "send"}
		if channel != "" {
			args = append(args, "--channel", channel)
		}
		if target != "" {
			args = append(args, "--target", target)
		}
		args = append(args, message)

		cmd := exec.CommandContext(ctx, openClawCmd, args...)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Run()
	}
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
	if !strings.HasPrefix(path, "mail") && !strings.HasPrefix(path, "calendar") {
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
