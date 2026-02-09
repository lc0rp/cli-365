package owa

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

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

// OWAEndpointWithApp returns the full URL for an OWA service action for the given app.
func OWAEndpointWithApp(action string, app string) string {
	if strings.TrimSpace(app) == "" {
		app = "Mail"
	}
	return fmt.Sprintf("%s?action=%s&app=%s&n=0", OWAAPIBase, action, app)
}

// OWAEndpointForURL returns the OWA endpoint for the given page URL.
func OWAEndpointForURL(pageURL string, action string) string {
	return OWAEndpointForURLWithApp(pageURL, action, "Mail")
}

// OWAEndpointForURLWithApp returns the OWA endpoint for the given page URL and app.
func OWAEndpointForURLWithApp(pageURL string, action string, app string) string {
	if strings.TrimSpace(app) == "" {
		app = "Mail"
	}
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
	return fmt.Sprintf("%s?action=%s&app=%s&n=0", base, action, app)
}

// OWAEndpointForPage returns the OWA endpoint for the current page.
func OWAEndpointForPage(page *rod.Page, action string) string {
	return OWAEndpointForPageWithApp(page, action, "Mail")
}

// OWAEndpointForPageWithApp returns the OWA endpoint for the current page and app.
func OWAEndpointForPageWithApp(page *rod.Page, action string, app string) string {
	if page == nil {
		return OWAEndpointWithApp(action, app)
	}
	info, err := page.Info()
	if err != nil || info == nil {
		return OWAEndpointWithApp(action, app)
	}
	return OWAEndpointForURLWithApp(info.URL, action, app)
}

// CallOWAAction calls an OWA service action with proper formatting.
func CallOWAAction(page *rod.Page, tokens *Tokens, action string, body interface{}) (*FetchResponse, error) {
	return callOWAAction(page, tokens, action, body, "", "Mail")
}

type OWAActionOptions struct {
	App       string
	ReqSource string
}

func CallOWAActionWithSource(page *rod.Page, tokens *Tokens, action string, body interface{}, reqSource string) (*FetchResponse, error) {
	return callOWAAction(page, tokens, action, body, "", reqSource)
}

func CallOWAActionWithOptions(page *rod.Page, tokens *Tokens, action string, body interface{}, opts OWAActionOptions) (*FetchResponse, error) {
	endpoint := ""
	if strings.TrimSpace(opts.App) != "" {
		endpoint = OWAEndpointForPageWithApp(page, action, opts.App)
	}
	return callOWAAction(page, tokens, action, body, endpoint, opts.ReqSource)
}

func callOWAAction(page *rod.Page, tokens *Tokens, action string, body interface{}, endpoint string, reqSource string) (*FetchResponse, error) {
	if tokens == nil {
		return nil, fmt.Errorf("tokens are nil")
	}
	if err := SessionFeatures().Check(action); err != nil {
		return nil, err
	}
	if endpoint == "" {
		endpoint = OWAEndpointForPage(page, action)
	}
	resp, err := callOWAActionAtWithSource(page, tokens, action, body, endpoint, reqSource)
	if err == nil && resp != nil && resp.Status == 401 && page != nil {
		if refreshed, rerr := refreshTokensFromPage(tokens, page); rerr == nil && refreshed != nil {
			resp, err = callOWAActionAtWithSource(page, refreshed, action, body, endpoint, reqSource)
		}
	}
	if err == nil && resp != nil && resp.Status == 404 {
		if alt := swapServiceSVCPath(endpoint); alt != "" && alt != endpoint {
			if retry, rerr := callOWAActionAtWithSource(page, tokens, action, body, alt, reqSource); rerr == nil && retry != nil {
				resp = retry
				err = nil
			}
		}
	}
	return resp, err
}

func callOWAActionAt(page *rod.Page, tokens *Tokens, action string, body interface{}, endpoint string) (*FetchResponse, error) {
	return callOWAActionAtWithSource(page, tokens, action, body, endpoint, "Mail")
}

func callOWAActionAtWithSource(page *rod.Page, tokens *Tokens, action string, body interface{}, endpoint string, reqSource string) (*FetchResponse, error) {
	fullBody := NewOWARequest(body)
	payload := interface{}(fullBody)
	if shouldUseRawBody(page) || isRawOWARequest(body) {
		payload = body
	}
	if strings.TrimSpace(reqSource) == "" {
		reqSource = "Mail"
	}
	req := FetchRequest{
		URL:    endpoint,
		Method: "POST",
		Headers: map[string]string{
			"Accept":              "application/json",
			"Content-Type":        "application/json; charset=utf-8",
			"Action":              action,
			"Prefer":              "IdType=\"ImmutableId\"",
			"X-OWA-CorrelationId": newCorrelationID(),
			"X-OWA-Hosted-UX":     "false",
			"X-Req-Source":        reqSource,
		},
		Body: payload,
	}

	addURLPostDataHeader(req.Headers, body)
	if tokens.Bearer != "" {
		req.Headers["Authorization"] = tokens.Bearer
	} else if tokens.Canary != "" {
		req.Headers["X-OWA-CANARY"] = tokens.Canary
	}
	if !tokens.Session.IsZero() {
		SetSessionHeaders(tokens.Session)
	}
	applySessionHeaders(req.Headers)
	applyPreferHeader(req.Headers)
	resp, err := Fetch(page, req)
	if err == nil {
		SessionFeatures().MaybeDisableFromResponse(action, resp)
	}
	return resp, err
}

func isRawOWARequest(body interface{}) bool {
	payload, ok := body.(map[string]interface{})
	if !ok || len(payload) == 0 {
		return false
	}
	if _, ok := payload["Header"]; !ok {
		return false
	}
	if _, ok := payload["Body"]; !ok {
		return false
	}
	if _, ok := payload["__type"]; !ok {
		return false
	}
	return true
}

func swapServiceSVCPath(endpoint string) string {
	if strings.Contains(endpoint, "/owa/0/service.svc") {
		return strings.Replace(endpoint, "/owa/0/service.svc", "/owa/service.svc", 1)
	}
	if strings.Contains(endpoint, "/owa/service.svc") {
		return strings.Replace(endpoint, "/owa/service.svc", "/owa/0/service.svc", 1)
	}
	return ""
}

func refreshTokensFromPage(tokens *Tokens, page *rod.Page) (*Tokens, error) {
	if page == nil {
		return tokens, errors.New("page is nil")
	}
	fresh, err := DiscoverTokens(page)
	if err != nil {
		return tokens, err
	}
	merged := MergeTokens(tokens, fresh)
	if merged != nil && !merged.Session.IsZero() {
		SetSessionHeaders(merged.Session)
	}
	if merged != nil {
		_ = SaveTokens(merged)
	}
	return merged, nil
}

// CallOWAActionWithBearer calls an OWA service action with bearer auth.
func CallOWAActionWithBearer(page *rod.Page, bearer string, action string, body interface{}) (*FetchResponse, error) {
	if err := SessionFeatures().Check(action); err != nil {
		return nil, err
	}
	endpoint := OWAEndpointForPage(page, action)
	req := FetchRequest{
		URL:    endpoint,
		Method: "POST",
		Headers: map[string]string{
			"Accept":        "application/json",
			"Content-Type":  "application/json",
			"Action":        action,
			"Authorization": bearer,
		},
		Body: NewOWARequest(body),
	}

	applySessionHeaders(req.Headers)
	resp, err := Fetch(page, req)
	if err == nil {
		SessionFeatures().MaybeDisableFromResponse(action, resp)
	}
	if err == nil && resp != nil && resp.Status == 404 {
		if alt := swapServiceSVCPath(endpoint); alt != "" && alt != endpoint {
			req.URL = alt
			if retry, rerr := Fetch(page, req); rerr == nil && retry != nil {
				resp = retry
			}
		}
	}
	return resp, err
}

func applySessionHeaders(headers map[string]string) {
	if headers == nil {
		return
	}
	session := CurrentSessionHeaders()
	if session.SessionID != "" {
		headers["X-OWA-SessionId"] = session.SessionID
	}
	if session.AnchorMailbox != "" {
		headers["X-AnchorMailbox"] = session.AnchorMailbox
	}
	if session.TenantID != "" {
		headers["X-TenantId"] = session.TenantID
	}
}

func applyPreferHeader(headers map[string]string) {
	if headers == nil {
		return
	}
	session := CurrentSessionHeaders()
	if session.Prefer == "" {
		return
	}
	existing, ok := headers["Prefer"]
	if !ok || existing == "" {
		headers["Prefer"] = session.Prefer
		return
	}
	lowerExisting := strings.ToLower(existing)
	if strings.Contains(lowerExisting, "exchange.behavior") {
		return
	}
	lowerSession := strings.ToLower(session.Prefer)
	if strings.Contains(lowerSession, "exchange.behavior") {
		if strings.Contains(lowerExisting, "idtype") && !strings.Contains(lowerSession, "idtype") {
			headers["Prefer"] = existing + ", " + session.Prefer
			return
		}
		headers["Prefer"] = session.Prefer
	}
}

func addURLPostDataHeader(headers map[string]string, body interface{}) {
	if headers == nil || body == nil {
		return
	}
	if _, ok := headers["X-OWA-UrlPostData"]; ok {
		return
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return
	}
	headers["X-OWA-UrlPostData"] = encodeURLPostData(string(encoded))
}

func encodeURLPostData(value string) string {
	escaped := url.QueryEscape(value)
	return strings.ReplaceAll(escaped, "+", "%20")
}

func shouldUseRawBody(page *rod.Page) bool {
	if page == nil {
		return false
	}
	info, err := page.Info()
	if err != nil || info == nil {
		return false
	}
	return strings.Contains(info.URL, "outlook.cloud.microsoft")
}

func newCorrelationID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
		uint16(b[6])<<8|uint16(b[7]),
		uint16(b[8])<<8|uint16(b[9]),
		uint64(b[10])<<40|uint64(b[11])<<32|uint64(b[12])<<24|uint64(b[13])<<16|uint64(b[14])<<8|uint64(b[15]),
	)
	return fmt.Sprintf("%s_%d", uuid, time.Now().UnixMilli()*100)
}
