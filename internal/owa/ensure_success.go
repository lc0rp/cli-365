package owa

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ensureOWASuccess returns an error when the response payload contains an explicit EWS/OWA error.
// Some OWA actions return HTTP 200 even when the operation failed; the failure is encoded in
// Body.ResponseMessages.Items[].ResponseClass/ResponseCode.
func ensureOWASuccess(body json.RawMessage) error {
	if len(body) == 0 {
		return nil
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
		return nil
	}

	if envelope.Error.Code != "" {
		if envelope.Error.Message != "" {
			return fmt.Errorf("%s: %s", envelope.Error.Code, envelope.Error.Message)
		}
		return fmt.Errorf("%s", envelope.Error.Code)
	}

	if code := strings.TrimSpace(envelope.Body.ResponseCode); code != "" && !strings.EqualFold(code, "NoError") {
		msg := strings.TrimSpace(envelope.Body.MessageText)
		if msg == "" {
			msg = strings.TrimSpace(envelope.Body.FaultMessage)
		}
		if msg != "" {
			return fmt.Errorf("%s: %s", code, msg)
		}
		if envelope.Body.ExceptionName != "" {
			return fmt.Errorf("%s: %s", code, envelope.Body.ExceptionName)
		}
		return fmt.Errorf("%s", code)
	}

	for _, item := range envelope.Body.ResponseMessages.Items {
		class := strings.TrimSpace(item.ResponseClass)
		code := strings.TrimSpace(item.ResponseCode)
		msg := strings.TrimSpace(item.MessageText)

		if class != "" && !strings.EqualFold(class, "Success") {
			if code != "" && msg != "" {
				return fmt.Errorf("%s: %s", code, msg)
			}
			if code != "" {
				return fmt.Errorf("%s", code)
			}
			if msg != "" {
				return fmt.Errorf("%s", msg)
			}
			return fmt.Errorf("%s", class)
		}

		if code != "" && !strings.EqualFold(code, "NoError") {
			if msg != "" {
				return fmt.Errorf("%s: %s", code, msg)
			}
			return fmt.Errorf("%s", code)
		}
	}

	return nil
}
