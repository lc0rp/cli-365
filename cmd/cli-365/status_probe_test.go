package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lc0rp/cli-365/internal/browser"
	"github.com/lc0rp/cli-365/internal/owa"
)

func runCLIForStatusTest(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	exitCode := 0
	stdout, stderr, err := captureProcessStdio(func() {
		exitCode = runCLI(context.Background(), append([]string{"cli-365"}, argv...), cliAppOptions{DisableDaemonForwarding: true})
	}, 0)
	if err != nil {
		t.Fatalf("captureProcessStdio() error: %v", err)
	}
	return exitCode, stdout, stderr
}

func TestAuthStatusJSONUsesLiveSessionProbe(t *testing.T) {
	stateHome := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("HOME", homeDir)

	if err := owa.SaveTokens(&owa.Tokens{
		Canary:      "cached-canary",
		UserEmail:   "cached@example.com",
		ExtractedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveTokens() error: %v", err)
	}

	exitCode, stdout, stderr := runCLIForStatusTest(t, "--json", "auth", "status")
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%q", exitCode, stderr)
	}

	var payload struct {
		Authenticated bool `json:"authenticated"`
		HasCanary     bool `json:"has_canary"`
		BrowserRun    bool `json:"browser_running"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal auth status: %v (stdout=%q)", err, stdout)
	}
	if payload.Authenticated {
		t.Fatalf("authenticated = true, want false when no live browser session")
	}
	if !payload.HasCanary {
		t.Fatalf("has_canary = false, want true from cached tokens")
	}
	if payload.BrowserRun {
		t.Fatalf("browser_running = true, want false")
	}
}

func TestBrowserStatusJSONDetectsStaleRuntime(t *testing.T) {
	stateHome := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("HOME", homeDir)

	if err := browser.SaveRuntime(&browser.RuntimeInfo{
		WSEndpoint: "ws://127.0.0.1:1/devtools/browser/stale",
		PID:        0,
		Managed:    false,
		StartedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveRuntime() error: %v", err)
	}

	exitCode, stdout, stderr := runCLIForStatusTest(t, "--json", "browser", "status")
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%q", exitCode, stderr)
	}

	var payload struct {
		Running           bool `json:"running"`
		EndpointReachable bool `json:"endpoint_reachable"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal browser status: %v (stdout=%q)", err, stdout)
	}
	if payload.Running {
		t.Fatalf("running = true, want false for stale runtime")
	}
	if payload.EndpointReachable {
		t.Fatalf("endpoint_reachable = true, want false for stale runtime")
	}
}

func TestBrowserStatusJSONDetectsReachableEndpoint(t *testing.T) {
	stateHome := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("HOME", homeDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"webSocketDebuggerUrl": "ws://127.0.0.1:9222/devtools/browser/ok",
		})
	}))
	defer server.Close()

	wsEndpoint := strings.Replace(server.URL, "http://", "ws://", 1) + "/devtools/browser/ok"
	if err := browser.SaveRuntime(&browser.RuntimeInfo{
		WSEndpoint: wsEndpoint,
		PID:        0,
		Managed:    false,
		StartedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveRuntime() error: %v", err)
	}

	exitCode, stdout, stderr := runCLIForStatusTest(t, "--json", "browser", "status")
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%q", exitCode, stderr)
	}

	var payload struct {
		Running           bool `json:"running"`
		EndpointReachable bool `json:"endpoint_reachable"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal browser status: %v (stdout=%q)", err, stdout)
	}
	if !payload.Running {
		t.Fatalf("running = false, want true for reachable endpoint")
	}
	if !payload.EndpointReachable {
		t.Fatalf("endpoint_reachable = false, want true")
	}
}
