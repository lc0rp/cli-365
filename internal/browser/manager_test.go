package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeInfo(t *testing.T) {
	rt := RuntimeInfo{
		WSEndpoint: "ws://localhost:9222/devtools/browser/abc123",
		PID:        12345,
		Managed:    true,
		StartedAt:  time.Now(),
	}

	data, err := json.Marshal(rt)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed RuntimeInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.WSEndpoint != rt.WSEndpoint {
		t.Errorf("WSEndpoint mismatch: got %q, want %q", parsed.WSEndpoint, rt.WSEndpoint)
	}
	if parsed.PID != rt.PID {
		t.Errorf("PID mismatch: got %d, want %d", parsed.PID, rt.PID)
	}
	if parsed.Managed != rt.Managed {
		t.Errorf("Managed mismatch: got %v, want %v", parsed.Managed, rt.Managed)
	}
}

func TestSaveAndLoadRuntime(t *testing.T) {
	tmpDir := t.TempDir()

	// Override state dir
	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	rt := &RuntimeInfo{
		WSEndpoint: "ws://localhost:9222/devtools/browser/test",
		PID:        54321,
		Managed:    true,
		StartedAt:  time.Now().Truncate(time.Second),
	}

	// Save
	if err := SaveRuntime(rt); err != nil {
		t.Fatalf("SaveRuntime error: %v", err)
	}

	// Load
	loaded, err := LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime error: %v", err)
	}

	if loaded.WSEndpoint != rt.WSEndpoint {
		t.Errorf("WSEndpoint mismatch: got %q, want %q", loaded.WSEndpoint, rt.WSEndpoint)
	}
	if loaded.PID != rt.PID {
		t.Errorf("PID mismatch: got %d, want %d", loaded.PID, rt.PID)
	}
}

func TestLoadRuntimeNotExist(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	_, err := LoadRuntime()
	if err == nil {
		t.Error("LoadRuntime should return error when file doesn't exist")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected IsNotExist error, got: %v", err)
	}
}

func TestStatusNotRunning(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	_, err := Status()
	if err == nil {
		t.Error("Status should return error when no runtime file exists")
	}
}

func TestIsRunningNotRunning(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	if IsRunning() {
		t.Error("IsRunning should return false when no browser is running")
	}
}

func TestIsRunningStalePID(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	// Save a runtime with a non-existent PID
	rt := &RuntimeInfo{
		WSEndpoint: "ws://localhost:9222/devtools/browser/test",
		PID:        999999, // Very unlikely to exist
		Managed:    true,
		StartedAt:  time.Now(),
	}
	if err := SaveRuntime(rt); err != nil {
		t.Fatalf("SaveRuntime error: %v", err)
	}

	// IsRunning should return false for stale PID
	if IsRunning() {
		t.Error("IsRunning should return false for non-existent PID")
	}
}

func TestRuntimePath(t *testing.T) {
	path := runtimePath()
	if path == "" {
		t.Error("runtimePath returned empty string")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("runtimePath returned relative path: %s", path)
	}
	if !strings.HasSuffix(path, "runtime.json") {
		t.Errorf("runtimePath should end with runtime.json: %s", path)
	}
}
