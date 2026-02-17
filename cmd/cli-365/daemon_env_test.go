package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestApplyDaemonRuntimeEnv(t *testing.T) {
	t.Setenv("DISPLAY", ":0")

	applyDaemonRuntimeEnv(" :1 ")
	if got := os.Getenv("DISPLAY"); got != ":1" {
		t.Fatalf("DISPLAY = %q, want %q", got, ":1")
	}

	applyDaemonRuntimeEnv("")
	if got := os.Getenv("DISPLAY"); got != ":1" {
		t.Fatalf("DISPLAY changed unexpectedly = %q", got)
	}
}

func TestApplyDaemonRuntimeEnvLinuxFallback(t *testing.T) {
	t.Setenv("DISPLAY", "")
	t.Setenv("XAUTHORITY", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	auth := filepath.Join(home, ".Xauthority")
	if err := os.WriteFile(auth, []byte("xauth"), 0o600); err != nil {
		t.Fatalf("write .Xauthority: %v", err)
	}

	applyDaemonRuntimeEnv("")
	if runtime.GOOS == "linux" {
		if got := os.Getenv("DISPLAY"); got != ":1" {
			t.Fatalf("DISPLAY = %q, want :1", got)
		}
		if got := os.Getenv("XAUTHORITY"); got != auth {
			t.Fatalf("XAUTHORITY = %q, want %q", got, auth)
		}
		return
	}
	if got := os.Getenv("DISPLAY"); got != "" {
		t.Fatalf("DISPLAY = %q, want empty on %s", got, runtime.GOOS)
	}
}

func TestApplyDaemonRuntimeEnvPreservesExistingXAuthority(t *testing.T) {
	t.Setenv("DISPLAY", "")
	t.Setenv("XAUTHORITY", "/tmp/already-set")

	applyDaemonRuntimeEnv(":1")
	if got := os.Getenv("XAUTHORITY"); got != "/tmp/already-set" {
		t.Fatalf("XAUTHORITY changed unexpectedly = %q", got)
	}
}
