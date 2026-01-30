package owa

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

var sessionFeatures = NewFeatureState(DefaultFeatureCatalog())

// FeatureState tracks feature availability for the current process.
type FeatureState struct {
	mu       sync.RWMutex
	catalog  FeatureCatalog
	disabled map[string]FeatureDisable
}

// FeatureDisable records a disabled action and why.
type FeatureDisable struct {
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
}

// FeatureDisabledError is returned when an action has been disabled.
type FeatureDisabledError struct {
	Action string
	Reason string
}

func (e FeatureDisabledError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("feature %s disabled", e.Action)
	}
	return fmt.Sprintf("feature %s disabled: %s", e.Action, e.Reason)
}

// SessionFeatures returns the process-local feature state.
func SessionFeatures() *FeatureState {
	return sessionFeatures
}

// NewFeatureState creates a new feature state from a catalog.
func NewFeatureState(catalog FeatureCatalog) *FeatureState {
	return &FeatureState{
		catalog:  catalog,
		disabled: make(map[string]FeatureDisable),
	}
}

// IsDisabled reports whether an action has been disabled.
func (s *FeatureState) IsDisabled(action string) (bool, FeatureDisable) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if action == "" {
		return false, FeatureDisable{}
	}
	disabled, ok := s.disabled[action]
	return ok, disabled
}

// Disable marks an action as disabled with a reason.
func (s *FeatureState) Disable(action string, reason string) {
	if action == "" {
		return
	}
	s.mu.Lock()
	s.disabled[action] = FeatureDisable{Reason: reason, At: time.Now()}
	s.mu.Unlock()
}

// Check returns an error if the action is disabled.
func (s *FeatureState) Check(action string) error {
	if ok, disabled := s.IsDisabled(action); ok {
		return FeatureDisabledError{Action: action, Reason: disabled.Reason}
	}
	return nil
}

// MaybeDisableFromResponse inspects a response and disables the action if unsupported.
func (s *FeatureState) MaybeDisableFromResponse(action string, resp *FetchResponse) {
	reason := classifyUnsupportedResponse(resp)
	if reason == "" {
		return
	}
	s.Disable(action, reason)
}

func classifyUnsupportedResponse(resp *FetchResponse) string {
	if resp == nil {
		return ""
	}
	info := parseOWAError(resp.Body)
	if info.Code != "" && isUnsupportedCode(info.Code) {
		return info.Code
	}
	if info.Exception != "" && isUnsupportedException(info.Exception) {
		return info.Exception
	}
	if info.Message != "" && isUnsupportedMessage(info.Message) {
		return info.Message
	}
	return ""
}

type owaErrorInfo struct {
	Code      string
	Message   string
	Exception string
}

func parseOWAError(body json.RawMessage) owaErrorInfo {
	var info owaErrorInfo
	if len(body) == 0 {
		return info
	}

	var envelope struct {
		Body struct {
			ResponseCode     string `json:"ResponseCode"`
			MessageText      string `json:"MessageText"`
			FaultMessage     string `json:"FaultMessage"`
			ExceptionName    string `json:"ExceptionName"`
			ResponseMessages struct {
				Items []struct {
					ResponseCode  string `json:"ResponseCode"`
					MessageText   string `json:"MessageText"`
					ResponseClass string `json:"ResponseClass"`
				} `json:"Items"`
			} `json:"ResponseMessages"`
		} `json:"Body"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		return info
	}

	if envelope.Error.Code != "" {
		info.Code = envelope.Error.Code
	}
	if envelope.Error.Message != "" {
		info.Message = envelope.Error.Message
	}

	if info.Code == "" && envelope.Body.ResponseCode != "" {
		info.Code = envelope.Body.ResponseCode
	}
	if info.Message == "" {
		if envelope.Body.MessageText != "" {
			info.Message = envelope.Body.MessageText
		} else if envelope.Body.FaultMessage != "" {
			info.Message = envelope.Body.FaultMessage
		}
	}
	if envelope.Body.ExceptionName != "" {
		info.Exception = envelope.Body.ExceptionName
	}

	if info.Code == "" && len(envelope.Body.ResponseMessages.Items) > 0 {
		info.Code = envelope.Body.ResponseMessages.Items[0].ResponseCode
	}
	if info.Message == "" && len(envelope.Body.ResponseMessages.Items) > 0 {
		info.Message = envelope.Body.ResponseMessages.Items[0].MessageText
	}

	return info
}

func isUnsupportedCode(code string) bool {
	lower := strings.ToLower(code)
	for _, needle := range []string{
		"not supported",
		"notsupported",
		"feature",
		"unsupported",
		"accessdenied",
		"invalidrequest",
		"notallowed",
		"notavailable",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func isUnsupportedException(name string) bool {
	lower := strings.ToLower(name)
	for _, needle := range []string{
		"notsupported",
		"notallowed",
		"accessdenied",
		"unauthorized",
		"invalid",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func isUnsupportedMessage(message string) bool {
	lower := strings.ToLower(message)
	for _, needle := range []string{
		"not supported",
		"not enabled",
		"access denied",
		"not allowed",
		"feature",
		"unsupported",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}
