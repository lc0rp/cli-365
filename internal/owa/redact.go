package owa

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

const binaryPlaceholder = "[binary data redacted]"

var (
	reEmail     = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	reBearer    = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*`)
	reJWT       = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	reGUID      = regexp.MustCompile(`[A-Fa-f0-9]{8}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{12}`)
	reLongToken = regexp.MustCompile(`[A-Za-z0-9_-]{24,}`)
)

func normalizeBody(payload []byte, maxBytes int) (string, bool) {
	if len(payload) == 0 {
		return "", false
	}
	truncated := false
	if maxBytes > 0 && len(payload) > maxBytes {
		payload = payload[:maxBytes]
		truncated = true
	}
	if !utf8.Valid(payload) {
		return binaryPlaceholder, truncated
	}
	return string(payload), truncated
}

func redactHeaders(headers map[string]string, hash bool) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		if isSensitiveHeader(k) {
			out[k] = redactToken("header", v, hash)
			continue
		}
		out[k] = redactString(v, hash)
	}
	return out
}

func redactBody(body string, contentType string, hash bool) string {
	if body == "" || body == binaryPlaceholder {
		return body
	}
	trimmed := strings.TrimSpace(body)
	if isJSONContent(contentType) || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if redacted, ok := redactJSONBody(trimmed, hash); ok {
			return redacted
		}
	}
	if isFormContent(contentType) || looksLikeFormEncoded(trimmed) {
		if redacted, ok := redactFormEncoded(trimmed, hash); ok {
			return redacted
		}
	}
	return redactString(body, hash)
}

func redactJSONBody(body string, hash bool) (string, bool) {
	var payload interface{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return "", false
	}
	payload = redactValue(payload, "", hash)
	redacted, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(redacted), true
}

func redactFormEncoded(body string, hash bool) (string, bool) {
	values, err := url.ParseQuery(body)
	if err != nil {
		return "", false
	}
	for key, list := range values {
		if isSensitiveKey(key) {
			values[key] = []string{redactToken(key, strings.Join(list, ","), hash)}
			continue
		}
		for i := range list {
			list[i] = redactString(list[i], hash)
		}
		values[key] = list
	}
	return values.Encode(), true
}

func redactValue(value interface{}, key string, hash bool) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		for k, v := range typed {
			typed[k] = redactValue(v, k, hash)
		}
		return typed
	case []interface{}:
		for i, v := range typed {
			typed[i] = redactValue(v, key, hash)
		}
		return typed
	case string:
		if isSensitiveKey(key) {
			return redactToken(key, typed, hash)
		}
		return redactString(typed, hash)
	default:
		return value
	}
}

func redactString(value string, hash bool) string {
	redacted := reBearer.ReplaceAllStringFunc(value, func(token string) string {
		return redactToken("bearer", token, hash)
	})
	redacted = reJWT.ReplaceAllStringFunc(redacted, func(token string) string {
		return redactToken("jwt", token, hash)
	})
	redacted = reEmail.ReplaceAllStringFunc(redacted, func(token string) string {
		return redactToken("email", token, hash)
	})
	redacted = reGUID.ReplaceAllStringFunc(redacted, func(token string) string {
		return redactToken("guid", token, hash)
	})
	redacted = reLongToken.ReplaceAllStringFunc(redacted, func(token string) string {
		return redactToken("token", token, hash)
	})
	return redacted
}

func redactToken(kind string, value string, hash bool) string {
	if !hash {
		return "<redacted:" + kind + ">"
	}
	sum := sha256.Sum256([]byte(value))
	return "<redacted:" + kind + ":" + hex.EncodeToString(sum[:6]) + ">"
}

func isSensitiveHeader(key string) bool {
	lower := strings.ToLower(key)
	switch {
	case strings.Contains(lower, "authorization"):
		return true
	case strings.Contains(lower, "cookie"):
		return true
	case strings.Contains(lower, "x-owa-"):
		return true
	case strings.Contains(lower, "x-ms-"):
		return true
	case strings.Contains(lower, "ms-cv"):
		return true
	default:
		return false
	}
}

func isSensitiveKey(key string) bool {
	if key == "" {
		return false
	}
	lower := strings.ToLower(key)
	for _, needle := range []string{
		"token",
		"bearer",
		"authorization",
		"cookie",
		"secret",
		"password",
		"session",
		"canary",
		"correlation",
		"auth",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func isJSONContent(contentType string) bool {
	lower := strings.ToLower(contentType)
	return strings.Contains(lower, "application/json") || strings.Contains(lower, "+json")
}

func isFormContent(contentType string) bool {
	lower := strings.ToLower(contentType)
	return strings.Contains(lower, "application/x-www-form-urlencoded")
}

func looksLikeFormEncoded(body string) bool {
	return strings.Contains(body, "=") && strings.Contains(body, "&")
}

func headerValue(headers map[string]string, key string) string {
	if len(headers) == 0 {
		return ""
	}
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}
