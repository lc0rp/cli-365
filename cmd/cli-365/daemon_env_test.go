package main

import (
	"os"
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
