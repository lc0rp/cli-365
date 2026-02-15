package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

var (
	logBearerPattern = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*`)
	logJWTPattern    = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	logCanaryPattern = regexp.MustCompile(`(?i)(x-owa-canary|canary)\s*[:=]\s*[A-Za-z0-9\-._~+/=]{6,}`)
	logTokenPattern  = regexp.MustCompile(`(?i)\b(token|session|cookie|authorization)\b\s*[:=]\s*[^\s,;]+`)
)

func (s *Server) logEvent(level, event string, fields map[string]interface{}) {
	if s == nil {
		return
	}

	entry := map[string]interface{}{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": strings.ToLower(strings.TrimSpace(level)),
		"event": strings.TrimSpace(event),
	}
	if entry["level"] == "" {
		entry["level"] = "info"
	}

	for k, v := range fields {
		entry[k] = sanitizeLogValue(k, v)
	}

	line, err := json.Marshal(entry)
	if err != nil {
		line = []byte(fmt.Sprintf(`{"ts":"%s","level":"error","event":"log_marshal_failed","error":"%s"}`,
			time.Now().UTC().Format(time.RFC3339Nano),
			redactLogText(err.Error()),
		))
	}

	s.logMu.Lock()
	defer s.logMu.Unlock()
	w := s.logWriter
	if w == nil {
		w = io.Discard
	}
	_, _ = w.Write(append(line, '\n'))
}

func (s *Server) SetLogWriter(w io.Writer) {
	if s == nil {
		return
	}
	s.logMu.Lock()
	defer s.logMu.Unlock()
	if w == nil {
		s.logWriter = io.Discard
		return
	}
	s.logWriter = w
}

func sanitizeLogValue(key string, value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		if isSensitiveLogKey(key) {
			return "<redacted>"
		}
		return redactLogText(typed)
	case []string:
		out := make([]string, 0, len(typed))
		for _, v := range typed {
			out = append(out, redactLogText(v))
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for k, v := range typed {
			out[k] = sanitizeLogValue(k, v)
		}
		return out
	default:
		return value
	}
}

func isSensitiveLogKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, needle := range []string{
		"token",
		"bearer",
		"authorization",
		"cookie",
		"canary",
		"session",
		"secret",
		"password",
		"auth",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func redactLogText(value string) string {
	redacted := logBearerPattern.ReplaceAllString(value, "<redacted:bearer>")
	redacted = logJWTPattern.ReplaceAllString(redacted, "<redacted:jwt>")
	redacted = logCanaryPattern.ReplaceAllString(redacted, "<redacted:canary>")
	redacted = logTokenPattern.ReplaceAllString(redacted, "<redacted:token>")
	return redacted
}
