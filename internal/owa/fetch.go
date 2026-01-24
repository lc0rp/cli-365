package owa

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-rod/rod"
)

// FetchRequest represents an OWA API request.
type FetchRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body,omitempty"`
}

// FetchResponse represents an OWA API response.
type FetchResponse struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers"`
	Body       json.RawMessage   `json:"body"`
}

// Fetch executes an HTTP request in the page context using OWA session cookies.
func Fetch(page *rod.Page, req FetchRequest) (*FetchResponse, error) {
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}

	// Default headers for OWA requests
	if _, ok := req.Headers["Accept"]; !ok {
		req.Headers["Accept"] = "application/json"
	}
	if _, ok := req.Headers["Content-Type"]; !ok && req.Body != nil {
		req.Headers["Content-Type"] = "application/json"
	}

	// Serialize body if needed
	var bodyStr string
	if req.Body != nil {
		switch v := req.Body.(type) {
		case string:
			bodyStr = v
		case []byte:
			bodyStr = string(v)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal body: %w", err)
			}
			bodyStr = string(b)
		}
	}

	result, err := page.Eval(`(url, method, headers, body) => {
		return new Promise(async (resolve, reject) => {
			try {
				const opts = {
					method: method,
					headers: headers,
					credentials: "include",
				};
				if (body) {
					opts.body = body;
				}

				const res = await fetch(url, opts);
				
				const respHeaders = {};
				res.headers.forEach((v, k) => { respHeaders[k] = v; });

				const text = await res.text();
				let body_parsed = null;
				if (text) {
					try {
						body_parsed = JSON.parse(text);
					} catch {
						body_parsed = text;
					}
				}

				resolve({
					status: res.status,
					statusText: res.statusText,
					headers: respHeaders,
					body: body_parsed,
				});
			} catch (err) {
				reject(err.message || String(err));
			}
		});
	}`, req.URL, req.Method, req.Headers, bodyStr)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	var resp FetchResponse
	if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// FetchWithCanary executes an OWA request with the canary header.
func FetchWithCanary(page *rod.Page, canary string, req FetchRequest) (*FetchResponse, error) {
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	req.Headers["X-OWA-CANARY"] = canary
	req.Headers["X-Req-Source"] = "Mail"

	return Fetch(page, req)
}

// OWAActionRequest is the standard OWA action request format.
type OWAActionRequest struct {
	Header OWARequestHeader `json:"Header"`
	Body   interface{}      `json:"Body,omitempty"`
}

// OWARequestHeader is the standard OWA request header.
type OWARequestHeader struct {
	RequestServerVersion string       `json:"RequestServerVersion"`
	TimeZoneContext      *OWATimeZone `json:"TimeZoneContext,omitempty"`
}

// OWATimeZone represents timezone context for OWA requests.
type OWATimeZone struct {
	TimeZoneDefinition struct {
		Id string `json:"Id"`
	} `json:"TimeZoneDefinition"`
}

// NewOWARequest creates a standard OWA action request.
func NewOWARequest(body interface{}) OWAActionRequest {
	return OWAActionRequest{
		Header: OWARequestHeader{
			RequestServerVersion: "Exchange2016",
		},
		Body: body,
	}
}

// OWAEndpoint returns the full URL for an OWA service action.
func OWAEndpoint(action string) string {
	return fmt.Sprintf("%s?action=%s&app=Mail&n=0", OWAAPIBase, action)
}

// OWAEndpointForURL returns the OWA endpoint for the given page URL.
func OWAEndpointForURL(pageURL string, action string) string {
	base := OWAAPIBase
	if pageURL != "" {
		if u, err := url.Parse(pageURL); err == nil && u.Scheme != "" && u.Host != "" {
			origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			switch {
			case strings.Contains(u.Host, "outlook.cloud.microsoft"):
				base = origin + "/owa/service.svc"
			case strings.Contains(u.Host, "outlook.office.com"):
				base = origin + "/owa/0/service.svc"
			case strings.Contains(u.Host, "outlook.office365.com"):
				base = origin + "/owa/0/service.svc"
			case strings.Contains(u.Host, "outlook.live.com"):
				base = origin + "/owa/0/service.svc"
			default:
				base = origin + "/owa/service.svc"
			}
		}
	}
	return fmt.Sprintf("%s?action=%s&app=Mail&n=0", base, action)
}

// OWAEndpointForPage returns the OWA endpoint for the current page.
func OWAEndpointForPage(page *rod.Page, action string) string {
	if page == nil {
		return OWAEndpoint(action)
	}
	info, err := page.Info()
	if err != nil || info == nil {
		return OWAEndpoint(action)
	}
	return OWAEndpointForURL(info.URL, action)
}

// CallOWAAction calls an OWA service action with proper formatting.
func CallOWAAction(page *rod.Page, canary string, action string, body interface{}) (*FetchResponse, error) {
	req := FetchRequest{
		URL:    OWAEndpointForPage(page, action),
		Method: "POST",
		Headers: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
			"Action":       action,
		},
		Body: NewOWARequest(body),
	}

	return FetchWithCanary(page, canary, req)
}

// CallOWAActionWithBearer calls an OWA service action with bearer auth.
func CallOWAActionWithBearer(page *rod.Page, bearer string, action string, body interface{}) (*FetchResponse, error) {
	req := FetchRequest{
		URL:    OWAEndpointForPage(page, action),
		Method: "POST",
		Headers: map[string]string{
			"Accept":        "application/json",
			"Content-Type":  "application/json",
			"Action":        action,
			"Authorization": bearer,
		},
		Body: NewOWARequest(body),
	}

	return Fetch(page, req)
}
