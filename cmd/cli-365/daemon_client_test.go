package main

import (
	"os/exec"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/lc0rp/cli-365/internal/daemon"
)

func TestStripDaemonFlag(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "standalone flag",
			in:   []string{"--daemon", "auth", "status"},
			want: []string{"auth", "status"},
		},
		{
			name: "equals true",
			in:   []string{"--daemon=true", "mail", "search", "x"},
			want: []string{"mail", "search", "x"},
		},
		{
			name: "value token",
			in:   []string{"--daemon", "true", "calendar", "list"},
			want: []string{"calendar", "list"},
		},
		{
			name: "mixed args",
			in:   []string{"--config", "x.yaml", "--daemon", "auth", "status"},
			want: []string{"--config", "x.yaml", "auth", "status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripDaemonFlag(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("stripDaemonFlag(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestComputeDaemonTimeouts(t *testing.T) {
	t.Run("defaults when unset", func(t *testing.T) {
		req, call := computeDaemonTimeouts(daemon.Options{})
		if req != 2*time.Minute {
			t.Fatalf("request timeout = %s, want 2m", req)
		}
		if call != 2*time.Minute+15*time.Second {
			t.Fatalf("call timeout = %s, want 2m15s", call)
		}
	})

	t.Run("auth recovery extends request timeout", func(t *testing.T) {
		req, call := computeDaemonTimeouts(daemon.Options{
			DefaultCommandTimeout: 2 * time.Minute,
			AuthRecoveryTimeout:   5 * time.Minute,
		})
		if req != 5*time.Minute+30*time.Second {
			t.Fatalf("request timeout = %s, want 5m30s", req)
		}
		if call != 5*time.Minute+45*time.Second {
			t.Fatalf("call timeout = %s, want 5m45s", call)
		}
	})

	t.Run("larger command timeout preserved", func(t *testing.T) {
		req, call := computeDaemonTimeouts(daemon.Options{
			DefaultCommandTimeout: 15 * time.Minute,
			AuthRecoveryTimeout:   5 * time.Minute,
		})
		if req != 15*time.Minute {
			t.Fatalf("request timeout = %s, want 15m", req)
		}
		if call != 15*time.Minute+15*time.Second {
			t.Fatalf("call timeout = %s, want 15m15s", call)
		}
	})
}

func TestDetachDaemonProcess(t *testing.T) {
	cmd := exec.Command("echo")
	detachDaemonProcess(cmd)
	switch runtime.GOOS {
	case "linux", "darwin":
		if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
			t.Fatalf("SysProcAttr = %+v, want Setsid=true", cmd.SysProcAttr)
		}
	default:
		if cmd.SysProcAttr != nil {
			t.Fatalf("SysProcAttr = %+v, want nil on %s", cmd.SysProcAttr, runtime.GOOS)
		}
	}
}
