package main

import (
	"context"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"

	"github.com/lc0rp/cli-365/internal/browser"
	"github.com/lc0rp/cli-365/internal/owa"
)

type browserStatusProbe struct {
	Running           bool      `json:"running"`
	Managed           bool      `json:"managed"`
	PID               int       `json:"pid"`
	PIDAlive          bool      `json:"pid_alive"`
	WSEndpoint        string    `json:"ws_endpoint"`
	EndpointReachable bool      `json:"endpoint_reachable"`
	StartedAt         time.Time `json:"started_at,omitempty"`
	Error             string    `json:"error,omitempty"`
}

type authStatusProbe struct {
	Authenticated bool
	UserEmail     string
}

func probeBrowserStatus(timeout time.Duration) browserStatusProbe {
	probe := browserStatusProbe{}
	rt, err := browser.Status()
	if err != nil {
		if !os.IsNotExist(err) {
			probe.Error = err.Error()
		}
		return probe
	}
	if rt == nil {
		return probe
	}

	probe.Managed = rt.Managed
	probe.PID = rt.PID
	probe.WSEndpoint = strings.TrimSpace(rt.WSEndpoint)
	probe.StartedAt = rt.StartedAt
	if probe.PID > 0 {
		probe.PIDAlive = browser.PIDAlive(probe.PID)
	}

	if probe.WSEndpoint != "" {
		reachable, err := browser.WSEndpointReachable(probe.WSEndpoint, timeout)
		probe.EndpointReachable = reachable
		if err != nil && probe.Error == "" {
			probe.Error = err.Error()
		}
	}

	if probe.Managed {
		switch {
		case probe.PID > 0:
			probe.Running = probe.PIDAlive && probe.EndpointReachable
		default:
			probe.Running = probe.EndpointReachable
		}
	} else {
		probe.Running = probe.EndpointReachable
	}
	return probe
}

func probeAuthStatusLive(browserProbe browserStatusProbe) authStatusProbe {
	if !browserProbe.Running || strings.TrimSpace(browserProbe.WSEndpoint) == "" {
		return authStatusProbe{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	b := rod.New().Context(ctx).ControlURL(browserProbe.WSEndpoint)
	if err := b.Connect(); err != nil {
		return authStatusProbe{}
	}

	pages, err := b.Pages()
	if err != nil {
		return authStatusProbe{}
	}
	for _, page := range pages {
		info, err := page.Info()
		if err != nil || info == nil {
			continue
		}
		if !isOWAStatusURL(info.URL) {
			continue
		}
		if !owa.IsLoggedIn(page) {
			continue
		}

		result := authStatusProbe{Authenticated: true}
		if tokens, err := owa.DiscoverTokens(page); err == nil && tokens != nil {
			result.UserEmail = strings.TrimSpace(tokens.UserEmail)
		}
		return result
	}
	return authStatusProbe{}
}

func isOWAStatusURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	if host == "" {
		return false
	}
	if !strings.Contains(host, "outlook.office.com") &&
		!strings.Contains(host, "outlook.office365.com") &&
		!strings.Contains(host, "outlook.live.com") &&
		!strings.Contains(host, "outlook.cloud.microsoft") {
		return false
	}
	path := strings.ToLower(u.Path)
	return strings.Contains(path, "/mail") ||
		strings.Contains(path, "/owa/") ||
		strings.Contains(path, "/calendar")
}
