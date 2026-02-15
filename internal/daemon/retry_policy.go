package daemon

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	maxReadRetries = 3
	readRetryBase  = 100 * time.Millisecond
	readRetryCap   = 2 * time.Second
)

var statusCodePattern = regexp.MustCompile(`(?i)\b(?:status|status code|http|code)\s*[:=]?\s*(\d{3})\b`)

func shouldRetryReadCommand(commandPath string, argv []string, result ExecResult) bool {
	if result.ExitCode == 0 && result.Err == nil {
		return false
	}
	if result.Err != nil {
		if strings.Contains(strings.ToLower(result.Err.Error()), "context deadline exceeded") {
			return false
		}
		if strings.Contains(strings.ToLower(result.Err.Error()), "context canceled") {
			return false
		}
	}

	path := strings.ToLower(strings.TrimSpace(commandPath))
	if path == "" {
		path = strings.ToLower(strings.TrimSpace(inferCommandPath(argv)))
	}
	if canonicalWritePath(path) != "" {
		return false
	}

	status := transientStatusCodeFromResult(result)
	return status == 429 || (status >= 500 && status <= 599)
}

func transientStatusCodeFromResult(result ExecResult) int {
	text := result.Stdout + "\n" + result.Stderr
	if result.Err != nil {
		text += "\n" + result.Err.Error()
	}
	matches := statusCodePattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return 0
	}
	code, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return code
}

func (s *Server) nextReadRetryDelay(retry int) time.Duration {
	if retry < 0 {
		retry = 0
	}
	max := readRetryBase << retry
	if max > readRetryCap {
		max = readRetryCap
	}
	if max <= 0 {
		return 0
	}

	s.randMu.Lock()
	n := s.rng.Int63n(int64(max) + 1)
	s.randMu.Unlock()
	return time.Duration(n)
}

func sleepWithContext(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
