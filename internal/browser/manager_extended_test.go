package browser

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeInfoJSONSerialization(t *testing.T) {
	tests := []struct {
		name string
		info RuntimeInfo
	}{
		{
			name: "managed browser",
			info: RuntimeInfo{
				WSEndpoint: "ws://127.0.0.1:9222/devtools/browser/abc123",
				PID:        12345,
				Managed:    true,
				StartedAt:  time.Date(2025, 1, 24, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "external browser",
			info: RuntimeInfo{
				WSEndpoint: "ws://remote-host:9222/devtools/browser/xyz",
				PID:        0,
				Managed:    false,
				StartedAt:  time.Now().Truncate(time.Second),
			},
		},
		{
			name: "minimal info",
			info: RuntimeInfo{
				WSEndpoint: "ws://localhost:9222",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.info)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var parsed RuntimeInfo
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			if parsed.WSEndpoint != tt.info.WSEndpoint {
				t.Errorf("WSEndpoint = %q, want %q", parsed.WSEndpoint, tt.info.WSEndpoint)
			}
			if parsed.PID != tt.info.PID {
				t.Errorf("PID = %d, want %d", parsed.PID, tt.info.PID)
			}
			if parsed.Managed != tt.info.Managed {
				t.Errorf("Managed = %v, want %v", parsed.Managed, tt.info.Managed)
			}
		})
	}
}

func TestRuntimePathConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	path := runtimePath()

	// Verify path is inside our temp dir
	if !filepath.IsAbs(path) {
		t.Errorf("runtimePath should be absolute: %s", path)
	}

	// Path should include cli-365 directory
	if !containsString(path, "cli-365") {
		t.Errorf("runtimePath should contain cli-365: %s", path)
	}

	// Path should end with runtime.json
	if filepath.Base(path) != "runtime.json" {
		t.Errorf("runtimePath should end with runtime.json: %s", path)
	}
}

func TestSaveRuntimeCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	rt := &RuntimeInfo{
		WSEndpoint: "ws://test:9222",
		PID:        999,
		Managed:    true,
		StartedAt:  time.Now(),
	}

	if err := SaveRuntime(rt); err != nil {
		t.Fatalf("SaveRuntime error: %v", err)
	}

	// Verify file was created
	path := runtimePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("runtime file was not created")
	}

	// Verify directory was created
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory stat error: %v", err)
	}
	if !info.IsDir() {
		t.Error("parent path is not a directory")
	}
}

func TestSaveAndLoadRuntimeRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	original := &RuntimeInfo{
		WSEndpoint: "ws://127.0.0.1:9222/devtools/browser/roundtrip-test",
		PID:        88888,
		Managed:    true,
		StartedAt:  time.Now().Truncate(time.Second),
	}

	if err := SaveRuntime(original); err != nil {
		t.Fatalf("SaveRuntime error: %v", err)
	}

	loaded, err := LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime error: %v", err)
	}

	if loaded.WSEndpoint != original.WSEndpoint {
		t.Errorf("WSEndpoint = %q, want %q", loaded.WSEndpoint, original.WSEndpoint)
	}
	if loaded.PID != original.PID {
		t.Errorf("PID = %d, want %d", loaded.PID, original.PID)
	}
	if loaded.Managed != original.Managed {
		t.Errorf("Managed = %v, want %v", loaded.Managed, original.Managed)
	}
	if !loaded.StartedAt.Equal(original.StartedAt) {
		t.Errorf("StartedAt = %v, want %v", loaded.StartedAt, original.StartedAt)
	}
}

func TestLoadRuntimeInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	// Create directory structure
	path := runtimePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}

	// Write invalid JSON
	if err := os.WriteFile(path, []byte("not valid json {{{"), 0o600); err != nil {
		t.Fatalf("write error: %v", err)
	}

	_, err := LoadRuntime()
	if err == nil {
		t.Error("LoadRuntime should return error for invalid JSON")
	}
}

func TestSaveRuntimeOverwrites(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	// Save first version
	rt1 := &RuntimeInfo{
		WSEndpoint: "ws://first:9222",
		PID:        111,
		Managed:    true,
		StartedAt:  time.Now(),
	}
	if err := SaveRuntime(rt1); err != nil {
		t.Fatalf("SaveRuntime 1 error: %v", err)
	}

	// Save second version
	rt2 := &RuntimeInfo{
		WSEndpoint: "ws://second:9222",
		PID:        222,
		Managed:    false,
		StartedAt:  time.Now(),
	}
	if err := SaveRuntime(rt2); err != nil {
		t.Fatalf("SaveRuntime 2 error: %v", err)
	}

	// Load and verify it's the second version
	loaded, err := LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime error: %v", err)
	}

	if loaded.WSEndpoint != "ws://second:9222" {
		t.Errorf("WSEndpoint = %q, want ws://second:9222", loaded.WSEndpoint)
	}
	if loaded.PID != 222 {
		t.Errorf("PID = %d, want 222", loaded.PID)
	}
}

func TestStatusReturnsLoadedRuntime(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	rt := &RuntimeInfo{
		WSEndpoint: "ws://status-test:9222",
		PID:        333,
		Managed:    true,
		StartedAt:  time.Now().Truncate(time.Second),
	}
	if err := SaveRuntime(rt); err != nil {
		t.Fatalf("SaveRuntime error: %v", err)
	}

	status, err := Status()
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}

	if status.WSEndpoint != rt.WSEndpoint {
		t.Errorf("WSEndpoint = %q, want %q", status.WSEndpoint, rt.WSEndpoint)
	}
}

func TestIsRunningWithNoRuntimeFile(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	// No runtime file exists
	if IsRunning() {
		t.Error("IsRunning should return false when no runtime file exists")
	}
}

func TestIsRunningWithExternalBrowser(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	// Save runtime for external browser (PID=0)
	rt := &RuntimeInfo{
		WSEndpoint: "ws://external:9222",
		PID:        0,
		Managed:    false,
		StartedAt:  time.Now(),
	}
	if err := SaveRuntime(rt); err != nil {
		t.Fatalf("SaveRuntime error: %v", err)
	}

	// Should return true because WSEndpoint is set (even without PID)
	if !IsRunning() {
		t.Error("IsRunning should return true for external browser with WSEndpoint")
	}
}

func TestIsRunningWithDeadPID(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	// Save runtime with a PID that doesn't exist
	rt := &RuntimeInfo{
		WSEndpoint: "ws://dead:9222",
		PID:        999999999, // Very unlikely to exist
		Managed:    true,
		StartedAt:  time.Now(),
	}
	if err := SaveRuntime(rt); err != nil {
		t.Fatalf("SaveRuntime error: %v", err)
	}

	// Should return false because PID doesn't exist
	if IsRunning() {
		t.Error("IsRunning should return false for non-existent PID")
	}
}

func TestStopWithNoRuntime(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	// Stop with no runtime file should error
	err := Stop()
	if err == nil {
		t.Error("Stop should return error when no runtime file exists")
	}
}

func TestStopWithUnmanagedBrowser(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	// Save runtime for unmanaged browser
	rt := &RuntimeInfo{
		WSEndpoint: "ws://external:9222",
		PID:        0,
		Managed:    false,
		StartedAt:  time.Now(),
	}
	if err := SaveRuntime(rt); err != nil {
		t.Fatalf("SaveRuntime error: %v", err)
	}

	// Stop should error for unmanaged browser
	err := Stop()
	if err == nil {
		t.Error("Stop should return error for unmanaged browser")
	}
	if err.Error() != "no managed browser to stop" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWSEndpointReachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"webSocketDebuggerUrl": "ws://127.0.0.1:9222/devtools/browser/test",
		})
	}))
	defer server.Close()

	wsEndpoint := strings.Replace(server.URL, "http://", "ws://", 1) + "/devtools/browser/test"
	reachable, err := WSEndpointReachable(wsEndpoint, time.Second)
	if err != nil {
		t.Fatalf("WSEndpointReachable() error: %v", err)
	}
	if !reachable {
		t.Fatal("WSEndpointReachable() = false, want true")
	}
}

func TestWSEndpointReachableInvalidPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer server.Close()

	wsEndpoint := strings.Replace(server.URL, "http://", "ws://", 1) + "/devtools/browser/test"
	reachable, err := WSEndpointReachable(wsEndpoint, time.Second)
	if err == nil {
		t.Fatal("WSEndpointReachable() error = nil, want error")
	}
	if reachable {
		t.Fatal("WSEndpointReachable() = true, want false")
	}
}

func TestPIDAliveCurrentProcess(t *testing.T) {
	if !PIDAlive(os.Getpid()) {
		t.Fatal("PIDAlive(current) = false, want true")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
