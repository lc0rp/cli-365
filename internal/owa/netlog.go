package owa

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// NetworkLogOptions controls what gets captured.
type NetworkLogOptions struct {
	MaxEntries    int
	CaptureBodies bool
	MaxBodyBytes  int
	BodyTimeout   time.Duration
	Redact        bool
	HashRedaction bool
}

// NetworkLogEntry captures a single network request lifecycle.
type NetworkLogEntry struct {
	RequestID             string            `json:"request_id"`
	URL                   string            `json:"url"`
	Method                string            `json:"method,omitempty"`
	ResourceType          string            `json:"resource_type,omitempty"`
	Status                int               `json:"status,omitempty"`
	StatusText            string            `json:"status_text,omitempty"`
	MimeType              string            `json:"mime_type,omitempty"`
	FromDiskCache         bool              `json:"from_disk_cache,omitempty"`
	FromServiceWorker     bool              `json:"from_service_worker,omitempty"`
	Failed                bool              `json:"failed,omitempty"`
	ErrorText             string            `json:"error_text,omitempty"`
	RequestHeaders        map[string]string `json:"request_headers,omitempty"`
	ResponseHeaders       map[string]string `json:"response_headers,omitempty"`
	RequestBody           string            `json:"request_body,omitempty"`
	RequestBodyTruncated  bool              `json:"request_body_truncated,omitempty"`
	RequestBodyRedacted   bool              `json:"request_body_redacted,omitempty"`
	ResponseBody          string            `json:"response_body,omitempty"`
	ResponseBodyTruncated bool              `json:"response_body_truncated,omitempty"`
	ResponseBodyRedacted  bool              `json:"response_body_redacted,omitempty"`
}

// NetworkLog is a snapshot of logged network activity.
type NetworkLog struct {
	StartedAt    time.Time         `json:"started_at"`
	EndedAt      time.Time         `json:"ended_at"`
	Dropped      int               `json:"dropped"`
	Entries      []NetworkLogEntry `json:"entries"`
	Redacted     bool              `json:"redacted"`
	BodyCapture  bool              `json:"body_capture"`
	MaxBodyBytes int               `json:"max_body_bytes,omitempty"`
}

// NetworkLogger collects network events from a page.
type NetworkLogger struct {
	mu        sync.Mutex
	startedAt time.Time
	endedAt   time.Time
	max       int
	dropped   int
	entries   []NetworkLogEntry
	index     map[proto.NetworkRequestID]int
	page      *rod.Page
	opts      NetworkLogOptions
	wg        sync.WaitGroup
	canary    string
}

// StartNetworkLogger begins capturing network events for a page.
func StartNetworkLogger(page *rod.Page, opts NetworkLogOptions) (*NetworkLogger, func(), error) {
	if page == nil {
		return nil, nil, errors.New("page is nil")
	}
	opts = normalizeNetworkLogOptions(opts)

	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       opts.MaxEntries,
		index:     make(map[proto.NetworkRequestID]int),
		page:      page,
		opts:      opts,
	}

	ctx, cancel := context.WithCancel(context.Background())
	wait := page.Context(ctx).EachEvent(
		func(e *proto.NetworkRequestWillBeSent) bool {
			logger.onRequest(e)
			return false
		},
		func(e *proto.NetworkResponseReceived) bool {
			logger.onResponse(e)
			return false
		},
		func(e *proto.NetworkLoadingFinished) bool {
			logger.onFinished(e)
			return false
		},
		func(e *proto.NetworkLoadingFailed) bool {
			logger.onFailed(e)
			return false
		},
	)

	done := make(chan struct{})
	go func() {
		defer close(done)
		wait()
	}()

	stop := func() {
		cancel()
		<-done
		logger.wg.Wait()
		logger.finish()
	}

	return logger, stop, nil
}

func (l *NetworkLogger) onRequest(e *proto.NetworkRequestWillBeSent) {
	if e == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.entries) >= l.max {
		l.dropped++
		return
	}

	entry := NetworkLogEntry{
		RequestID:    string(e.RequestID),
		URL:          e.Request.URL,
		Method:       e.Request.Method,
		ResourceType: string(e.Type),
	}
	if shouldCaptureHeaders(e.Request.URL) {
		raw := headersToStringMap(e.Request.Headers)
		l.maybeSetCanary(raw)
		entry.RequestHeaders = l.maybeRedactHeaders(raw)
	}
	l.index[e.RequestID] = len(l.entries)
	l.entries = append(l.entries, entry)

	if l.opts.CaptureBodies && e.Request.HasPostData && shouldCaptureBody(entry.URL) {
		l.wg.Add(1)
		reqID := e.RequestID
		go func() {
			defer l.wg.Done()
			l.captureRequestBody(reqID)
		}()
	}
}

func (l *NetworkLogger) onResponse(e *proto.NetworkResponseReceived) {
	if e == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	idx, ok := l.index[e.RequestID]
	if !ok {
		if len(l.entries) >= l.max {
			l.dropped++
			return
		}
		entry := NetworkLogEntry{
			RequestID:    string(e.RequestID),
			URL:          e.Response.URL,
			ResourceType: string(e.Type),
		}
		l.index[e.RequestID] = len(l.entries)
		l.entries = append(l.entries, entry)
		idx = len(l.entries) - 1
	}

	entry := &l.entries[idx]
	entry.Status = int(e.Response.Status)
	entry.StatusText = e.Response.StatusText
	entry.MimeType = e.Response.MIMEType
	entry.FromDiskCache = e.Response.FromDiskCache
	entry.FromServiceWorker = e.Response.FromServiceWorker
	if shouldCaptureHeaders(e.Response.URL) {
		raw := headersToStringMap(e.Response.Headers)
		l.maybeSetCanary(raw)
		entry.ResponseHeaders = l.maybeRedactHeaders(raw)
	}
	if entry.URL == "" {
		entry.URL = e.Response.URL
	}
	if entry.ResourceType == "" {
		entry.ResourceType = string(e.Type)
	}
}

func (l *NetworkLogger) onFailed(e *proto.NetworkLoadingFailed) {
	if e == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	idx, ok := l.index[e.RequestID]
	if !ok {
		if len(l.entries) >= l.max {
			l.dropped++
			return
		}
		entry := NetworkLogEntry{
			RequestID:    string(e.RequestID),
			ResourceType: string(e.Type),
		}
		l.index[e.RequestID] = len(l.entries)
		l.entries = append(l.entries, entry)
		idx = len(l.entries) - 1
	}

	entry := &l.entries[idx]
	entry.Failed = true
	entry.ErrorText = e.ErrorText
	if entry.ResourceType == "" {
		entry.ResourceType = string(e.Type)
	}
}

func (l *NetworkLogger) onFinished(e *proto.NetworkLoadingFinished) {
	if e == nil || !l.opts.CaptureBodies {
		return
	}

	var url string
	l.mu.Lock()
	idx, ok := l.index[e.RequestID]
	if ok {
		url = l.entries[idx].URL
	}
	l.mu.Unlock()

	if !ok || !shouldCaptureBody(url) {
		return
	}

	l.wg.Add(1)
	reqID := e.RequestID
	go func() {
		defer l.wg.Done()
		l.captureResponseBody(reqID)
	}()
}

func (l *NetworkLogger) finish() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.endedAt.IsZero() {
		l.endedAt = time.Now()
	}
}

// Snapshot returns a copy of the current log state.
func (l *NetworkLogger) Snapshot() NetworkLog {
	l.mu.Lock()
	defer l.mu.Unlock()

	entries := make([]NetworkLogEntry, len(l.entries))
	copy(entries, l.entries)

	return NetworkLog{
		StartedAt:    l.startedAt,
		EndedAt:      l.endedAt,
		Dropped:      l.dropped,
		Entries:      entries,
		Redacted:     l.opts.Redact,
		BodyCapture:  l.opts.CaptureBodies,
		MaxBodyBytes: l.opts.MaxBodyBytes,
	}
}

func (l *NetworkLogger) Canary() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.canary
}

func (l *NetworkLogger) maybeSetCanary(headers map[string]string) {
	if l.canary != "" || len(headers) == 0 {
		return
	}
	for k, v := range headers {
		if strings.EqualFold(k, "x-owa-canary") && v != "" {
			l.canary = v
			return
		}
	}
}

func (l *NetworkLogger) maybeRedactHeaders(headers map[string]string) map[string]string {
	if !l.opts.Redact || len(headers) == 0 {
		return headers
	}
	return redactHeaders(headers, l.opts.HashRedaction)
}

func (l *NetworkLogger) captureRequestBody(reqID proto.NetworkRequestID) {
	ctx, cancel := context.WithTimeout(context.Background(), l.opts.BodyTimeout)
	defer cancel()
	resp, err := proto.NetworkGetRequestPostData{RequestID: reqID}.Call(l.page.Context(ctx))
	if err != nil || resp == nil {
		return
	}
	l.setRequestBody(reqID, []byte(resp.PostData))
}

func (l *NetworkLogger) captureResponseBody(reqID proto.NetworkRequestID) {
	ctx, cancel := context.WithTimeout(context.Background(), l.opts.BodyTimeout)
	defer cancel()
	resp, err := proto.NetworkGetResponseBody{RequestID: reqID}.Call(l.page.Context(ctx))
	if err != nil || resp == nil {
		return
	}

	var payload []byte
	if resp.Base64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(resp.Body)
		if err != nil {
			return
		}
		payload = decoded
	} else {
		payload = []byte(resp.Body)
	}
	l.setResponseBody(reqID, payload)
}

func (l *NetworkLogger) setRequestBody(reqID proto.NetworkRequestID, payload []byte) {
	l.mu.Lock()
	idx, ok := l.index[reqID]
	if !ok {
		l.mu.Unlock()
		return
	}
	entry := &l.entries[idx]
	contentType := headerValue(entry.RequestHeaders, "content-type")
	l.mu.Unlock()

	body, truncated := normalizeBody(payload, l.opts.MaxBodyBytes)
	body = redactBody(body, contentType, l.opts.HashRedaction)

	l.mu.Lock()
	entry = &l.entries[idx]
	entry.RequestBody = body
	entry.RequestBodyTruncated = truncated
	entry.RequestBodyRedacted = l.opts.Redact
	l.mu.Unlock()
}

func (l *NetworkLogger) setResponseBody(reqID proto.NetworkRequestID, payload []byte) {
	l.mu.Lock()
	idx, ok := l.index[reqID]
	if !ok {
		l.mu.Unlock()
		return
	}
	entry := &l.entries[idx]
	contentType := entry.MimeType
	if contentType == "" {
		contentType = headerValue(entry.ResponseHeaders, "content-type")
	}
	l.mu.Unlock()

	body, truncated := normalizeBody(payload, l.opts.MaxBodyBytes)
	body = redactBody(body, contentType, l.opts.HashRedaction)

	l.mu.Lock()
	entry = &l.entries[idx]
	entry.ResponseBody = body
	entry.ResponseBodyTruncated = truncated
	entry.ResponseBodyRedacted = l.opts.Redact
	l.mu.Unlock()
}

func shouldCaptureHeaders(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "outlook.") || strings.Contains(lower, "/owa/")
}

func shouldCaptureBody(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "/owa/service.svc") || strings.Contains(lower, "/owa/startupdata.ashx")
}

func normalizeNetworkLogOptions(opts NetworkLogOptions) NetworkLogOptions {
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = 500
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 64 * 1024
	}
	if opts.BodyTimeout <= 0 {
		opts.BodyTimeout = 5 * time.Second
	}
	if opts.CaptureBodies {
		opts.Redact = true
	}
	return opts
}

func headersToStringMap(headers proto.NetworkHeaders) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}
