package owa

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// NetworkLogEntry captures a single network request lifecycle.
type NetworkLogEntry struct {
	RequestID         string `json:"request_id"`
	URL               string `json:"url"`
	Method            string `json:"method,omitempty"`
	ResourceType      string `json:"resource_type,omitempty"`
	Status            int    `json:"status,omitempty"`
	StatusText        string `json:"status_text,omitempty"`
	MimeType          string `json:"mime_type,omitempty"`
	FromDiskCache     bool   `json:"from_disk_cache,omitempty"`
	FromServiceWorker bool   `json:"from_service_worker,omitempty"`
	Failed            bool   `json:"failed,omitempty"`
	ErrorText         string `json:"error_text,omitempty"`
}

// NetworkLog is a snapshot of logged network activity.
type NetworkLog struct {
	StartedAt time.Time         `json:"started_at"`
	EndedAt   time.Time         `json:"ended_at"`
	Dropped   int               `json:"dropped"`
	Entries   []NetworkLogEntry `json:"entries"`
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
}

// StartNetworkLogger begins capturing network events for a page.
func StartNetworkLogger(page *rod.Page, maxEntries int) (*NetworkLogger, func(), error) {
	if page == nil {
		return nil, nil, errors.New("page is nil")
	}
	if maxEntries <= 0 {
		maxEntries = 500
	}

	logger := &NetworkLogger{
		startedAt: time.Now(),
		max:       maxEntries,
		index:     make(map[proto.NetworkRequestID]int),
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
	l.index[e.RequestID] = len(l.entries)
	l.entries = append(l.entries, entry)
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
		StartedAt: l.startedAt,
		EndedAt:   l.endedAt,
		Dropped:   l.dropped,
		Entries:   entries,
	}
}
