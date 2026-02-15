package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomeDir(t *testing.T) {
	home := HomeDir()
	if home == "" {
		t.Error("HomeDir returned empty string")
	}
	if !filepath.IsAbs(home) {
		t.Errorf("HomeDir returned relative path: %s", home)
	}
}

func TestConfigDir(t *testing.T) {
	// Test with XDG_CONFIG_HOME set
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	dir := ConfigDir()
	if dir != tmpDir {
		t.Errorf("ConfigDir = %q, want %q", dir, tmpDir)
	}
}

func TestConfigDirDefault(t *testing.T) {
	// Test without XDG_CONFIG_HOME
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	dir := ConfigDir()
	if dir == "" {
		t.Error("ConfigDir returned empty string")
	}
	if !strings.HasSuffix(dir, ".config") {
		t.Errorf("ConfigDir should end with .config: %s", dir)
	}
}

func TestStateDir(t *testing.T) {
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	dir := StateDir()
	if dir != tmpDir {
		t.Errorf("StateDir = %q, want %q", dir, tmpDir)
	}
}

func TestStateDirDefault(t *testing.T) {
	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Unsetenv("XDG_STATE_HOME")
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	dir := StateDir()
	if dir == "" {
		t.Error("StateDir returned empty string")
	}
	if !strings.HasSuffix(dir, filepath.Join(".local", "state")) {
		t.Errorf("StateDir should end with .local/state: %s", dir)
	}
}

func TestConfigPath(t *testing.T) {
	path := ConfigPath()
	if path == "" {
		t.Error("ConfigPath returned empty string")
	}
	if !strings.HasSuffix(path, "config.yaml") {
		t.Errorf("ConfigPath should end with config.yaml: %s", path)
	}
}

func TestProfileDir(t *testing.T) {
	dir := ProfileDir()
	if dir == "" {
		t.Error("ProfileDir returned empty string")
	}
	if !strings.HasSuffix(dir, "profile") {
		t.Errorf("ProfileDir should end with profile: %s", dir)
	}
}

func TestRuntimePath(t *testing.T) {
	path := RuntimePath()
	if path == "" {
		t.Error("RuntimePath returned empty string")
	}
	if !strings.HasSuffix(path, "runtime.json") {
		t.Errorf("RuntimePath should end with runtime.json: %s", path)
	}
}

func TestDaemonPaths(t *testing.T) {
	stateDir := DaemonStateDir()
	if stateDir == "" {
		t.Error("DaemonStateDir returned empty string")
	}
	if !strings.HasSuffix(stateDir, "cli-365") {
		t.Errorf("DaemonStateDir should end with cli-365: %s", stateDir)
	}

	socketPath := DaemonSocketPath()
	if !strings.HasSuffix(socketPath, "daemon.sock") {
		t.Errorf("DaemonSocketPath should end with daemon.sock: %s", socketPath)
	}

	lockPath := DaemonLockPath()
	if !strings.HasSuffix(lockPath, "daemon.lock") {
		t.Errorf("DaemonLockPath should end with daemon.lock: %s", lockPath)
	}

	statusPath := DaemonStatusPath()
	if !strings.HasSuffix(statusPath, "daemon.json") {
		t.Errorf("DaemonStatusPath should end with daemon.json: %s", statusPath)
	}
}

func TestExpandUser(t *testing.T) {
	home := HomeDir()

	tests := []struct {
		name   string
		input  string
		want   string
		prefix string
	}{
		{"Empty", "", "", ""},
		{"Home only", "~", home, home},
		{"Home prefix", "~/Documents", filepath.Join(home, "Documents"), home},
		{"No tilde", "/tmp/test", "/tmp/test", "/tmp"},
		{"Tilde not at start", "/path/to/~", "/path/to/~", "/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandUser(tt.input)
			if tt.want != "" && got != tt.want {
				t.Errorf("ExpandUser(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if tt.prefix != "" && len(got) >= len(tt.prefix) {
				if got[:len(tt.prefix)] != tt.prefix {
					t.Errorf("ExpandUser(%q) = %q, should start with %q", tt.input, got, tt.prefix)
				}
			}
		})
	}
}
